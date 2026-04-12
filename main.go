package main

import (
	"context"
	"embed"
	"excalidraw-complete/config"
	"excalidraw-complete/core"
	"excalidraw-complete/handlers/api/documents"
	"excalidraw-complete/handlers/api/firebase"
	rds "excalidraw-complete/redis"
	"excalidraw-complete/stores"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/zishang520/socket.io/adapters/redis/v3/adapter"
	"github.com/zishang520/socket.io/servers/engine/v3"
	socketio "github.com/zishang520/socket.io/servers/socket/v3"
	"github.com/zishang520/socket.io/v3/pkg/log"
	"github.com/zishang520/socket.io/v3/pkg/types"
	"github.com/zishang520/socket.io/v3/pkg/utils"
)

type (
	// UserToFollow represents a user that a client wants to follow.
	UserToFollow struct {
		SocketID string `json:"socketId"`
		Username string `json:"username"`
	}

	// OnUserFollowedPayload is the payload sent when a user follows or unfollows another user.
	OnUserFollowedPayload struct {
		UserToFollow UserToFollow `json:"userToFollow"`
		Action       string       `json:"action"` // "FOLLOW" | "UNFOLLOW"
	}
)

//go:embed all:frontend
var assets embed.FS

// init is invoked before main()
func init() {
	// loads values from .env into the system
	if err := godotenv.Load(); err != nil {
		logrus.WithError(err).Info("No .env file found, using environment variables")
	}
}

func handleUI(config *config.Config) http.Handler {
	sub, err := fs.Sub(assets, "frontend")
	if err != nil {
		panic(err)
	}
	// Check if the frontend URL is set and determine if SSL is used
	useSSL := strings.Split(config.FrontendURL, "://")[0] == "https"
	frontendBaseURL := strings.Split(config.FrontendURL, "://")[1]

	if frontendBaseURL != "localhost:3002" {

		// Create a log field for the frontend URL and SSL status
		urlField := logrus.Fields{
			"baseUrl": frontendBaseURL,
			"isSSL":   useSSL,
		}
		logrus.WithFields(urlField).Info("Frontend URL configuration")
	} else {
		// Create a log field for the frontend URL and SSL status
		logrus.Info("Frontend URL is not set; defaulting to localhost:3002 with SSL disabled")
		urlField := logrus.Fields{
			"baseUrl": frontendBaseURL,
			"isSSL":   useSSL,
		}
		logrus.WithFields(urlField).Info("Frontend URL configuration")
	}

	// Let's hot-patch all calls to firebase DB
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originalPath := r.URL.Path
		originalPath = strings.TrimPrefix(originalPath, "/")

		// Redirect "/" to "index.html"
		if originalPath == "" {
			originalPath = "index.html"
		}

		file, err := sub.Open(originalPath)
		if err != nil {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		defer func() {
			if cErr := file.Close(); cErr != nil {
				logrus.WithError(cErr).Warn("Failed to close file")
			}
		}()

		fileContent, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "Error reading file", http.StatusInternalServerError)
			return
		}

		// Replace firebase URLs with the base URL
		// and adjust SSL settings if necessary
		modifiedContent := strings.ReplaceAll(string(fileContent), "firestore.googleapis.com", frontendBaseURL)

		// Adjust SSL settings in the content based on the frontend URL configuration
		switch {
		case useSSL:
			modifiedContent = strings.ReplaceAll(modifiedContent, "cN=!1", "cN=!0")
			modifiedContent = strings.ReplaceAll(modifiedContent, "ssl:!1", "ssl:!0")

		case !useSSL:
			modifiedContent = strings.ReplaceAll(modifiedContent, "cN=!0", "cN=!1")
			modifiedContent = strings.ReplaceAll(modifiedContent, "ssl:!0", "ssl:!1")

		}

		// Set the correct Content-Type based on the file extension
		contentType := http.DetectContentType([]byte(modifiedContent))
		switch {
		case strings.HasSuffix(originalPath, ".js"):
			contentType = "application/javascript"
		case strings.HasSuffix(originalPath, ".html"):
			contentType = "text/html"
		case strings.HasSuffix(originalPath, ".css"):
			contentType = "text/css"
		case strings.HasSuffix(originalPath, ".wasm"):
			contentType = "application/wasm"
		case strings.HasSuffix(originalPath, ".tsx"):
			contentType = "text/typescript"
		case strings.HasSuffix(originalPath, ".png"):
			contentType = "image/png"
		case strings.HasSuffix(originalPath, ".woff2"):
			contentType = "font/woff2"
		}

		// Serve the modified content
		w.Header().Set("Content-Type", contentType)
		_, err = w.Write([]byte(modifiedContent))
		if err != nil {
			http.Error(w, "Error serving file", http.StatusInternalServerError)
			return
		}
	})
}

func setupRouter(config *config.Config, documentStore core.DocumentStore, redisClient *rds.Client) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	corsAllowedOrigins := config.CorsAllowedOrigins()
	logrus.WithField("corsAllowedOrigins", corsAllowedOrigins).Info("Configured allowed origins for CORS")
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   corsAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "Content-Length", "X-CSRF-Token", "Token", "session", "Origin", "Host", "Connection", "Accept-Encoding", "Accept-Language", "X-Requested-With", "X-Goog-Api-Client", "X-Firebase-GMPID", "X-HTTP-Session-Id", "X-Firebase-Client"},
		ExposedHeaders:   []string{"X-HTTP-Session-Id", "X-Goog-Channel-Id", "X-Goog-Channel-Token"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))
	r.Route("/google.firestore.v1.Firestore", func(r chi.Router) {
		r.Post("/Listen/channel", firebase.HandleFetchDocument(config, redisClient))
		r.Get("/Listen/channel", firebase.HandleFetchDocument(config, redisClient))
	})
	r.Route("/v1/projects/{project_id}/databases/{database_id}", func(r chi.Router) {
		r.Options("/documents:commit", firebase.HandleCors(corsAllowedOrigins))
		r.Post("/documents:commit", firebase.HandleBatchCommit(config, redisClient))
		r.Options("/documents:batchGet", firebase.HandleCors(corsAllowedOrigins))
		r.Post("/documents:batchGet", firebase.HandleBatchGet(config, redisClient))

	})

	r.Route("/api/v2", func(r chi.Router) {
		r.Post("/post/", documents.HandleCreate(documentStore))
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", documents.HandleGet(documentStore))
		})
	})
	return r
}

func setupSocketIO(config *config.Config, opts *socketio.ServerOptions, redisClient *rds.Client, timeout time.Duration) (*socketio.Server, error) {
	//Set

	ioo := socketio.NewServer(nil, opts)

	ioo.On("connection", func(clients ...any) {

		socket := clients[0].(*socketio.Socket)
		me := socket.Id()
		myRoom := socketio.Room(me)

		ioo.To(myRoom).Emit("init-room")
		utils.Log().Printf("init room %v", myRoom)
		socket.On("join-room", func(datas ...any) {
			room := socketio.Room(datas[0].(string))
			utils.Log().Printf("Socket %v has joined %v\n", me, room)
			socket.Join(room)

			ioo.In(room).Timeout(timeout).FetchSockets()(func(usersInRoom []*socketio.RemoteSocket, err error) {
				if err != nil {
					utils.Log().Printf("Error fetching sockets in room %v: %v\n", room, err)
					return
				}

				newRoomUsers := []socketio.SocketId{}
				for _, user := range usersInRoom {
					newRoomUsers = append(newRoomUsers, user.Id())
				}

				//sync joined room to redis
				if config.HAActive {

					// setup cache store
					allUserInRoom, err := rds.SyncStoredUsersInRoomWithLocal(redisClient, room, newRoomUsers, timeout)
					if err != nil {
						utils.Log().Printf("Error syncing users in room %v to redis: %v\n", room, err)
						return
					}
					newRoomUsers = allUserInRoom
				}

				if len(newRoomUsers) <= 1 {
					utils.Log().Printf("emit first user %v in room %v\n", me, room)
					ioo.To(myRoom).Emit("first-in-room")

				} else {
					utils.Log().Printf("emit new user %v in room %v\n", me, room)
					socket.Broadcast().To(room).Emit("new-user", me)

				}

				utils.Log().Printf(" room %v has users %v", room, newRoomUsers)
				ioo.In(room).Emit(
					"room-user-change",
					newRoomUsers,
				)

			})
		})

		socket.On("server-broadcast", func(datas ...any) {
			roomID := datas[0].(string)
			room := socketio.Room(roomID)
			utils.Log().Printf(" user %v sends update to room %v\n", me, room)
			socket.Broadcast().To(room).Emit("client-broadcast", datas[1], datas[2])

		})

		socket.On("server-volatile-broadcast", func(datas ...any) {
			roomID := datas[0].(string)
			room := socketio.Room(roomID)
			utils.Log().Printf(" user %v sends volatile update to room %v\n", me, room)
			socket.Volatile().Broadcast().To(room).Emit("client-broadcast", datas[1], datas[2])

		})

		socket.On("user-follow", func(payload ...any) {
			//love this socket libary...
			paylow := OnUserFollowedPayload{
				UserToFollow: UserToFollow{
					SocketID: payload[0].(map[string]any)["userToFollow"].(map[string]any)["socketId"].(string),
					Username: payload[0].(map[string]any)["userToFollow"].(map[string]any)["username"].(string),
				},
				Action: payload[0].(map[string]any)["action"].(string),
			}

			fRoomID := socketio.Room(fmt.Sprintf("follow@%s", paylow.UserToFollow.SocketID))

			switch paylow.Action {
			case "FOLLOW":
				socket.Join(socketio.Room(fRoomID))

				sockets := ioo.In(fRoomID).Timeout(timeout).FetchSockets()
				followedby := []socketio.SocketId{}
				sockets(func(usersInRoom []*socketio.RemoteSocket, err error) {
					if err != nil {
						utils.Log().Printf("Error fetching sockets in room %v: %v\n", fRoomID, err)
						return
					}
					for _, user := range usersInRoom {
						followedby = append(followedby, user.Id())
					}

					if config.HAActive {
						// sync follow room to redis
						allUserInRoom, err := rds.SyncStoredUsersInRoomWithLocal(redisClient, fRoomID, followedby, timeout)
						if err != nil {
							utils.Log().Printf("Error syncing users in room %v to redis: %v\n", fRoomID, err)
							return
						}
						followedby = allUserInRoom
					}

				})

				utils.Log().Printf("user %v followed %v (followed by %v)\n", me, paylow.UserToFollow.SocketID, followedby)
				ioo.To(socketio.Room(paylow.UserToFollow.SocketID)).Emit(
					"user-follow-room-change",
					followedby,
				)

			case "UNFOLLOW":
				socket.Leave(socketio.Room(fRoomID))

				sockets := ioo.In(fRoomID).Timeout(timeout).FetchSockets()
				followedby := []socketio.SocketId{}
				sockets(func(usersInRoom []*socketio.RemoteSocket, err error) {
					if err != nil {
						utils.Log().Printf("Error fetching sockets in room %v: %v\n", fRoomID, err)
						return
					}
					for _, user := range usersInRoom {
						followedby = append(followedby, user.Id())
					}

					if config.HAActive {
						err = rds.RemoveUserFromStoredUsersInRoom(redisClient, fRoomID, me, timeout)
						if err != nil {
							utils.Log().Printf("Error removing user %v from stored users in room %v: %v\n", me, fRoomID, err)
							return
						}

						// sync follow room to redis
						allUserInRoom, err := rds.SyncStoredUsersInRoomWithLocal(redisClient, fRoomID, followedby, timeout)
						if err != nil {
							utils.Log().Printf("Error syncing users in room %v to redis: %v\n", fRoomID, err)
							return
						}
						followedby = allUserInRoom
					}
				})

				utils.Log().Printf("user %v unfollowed %v (followed by %v)\n", me, paylow.UserToFollow.SocketID, followedby)
				ioo.To(socketio.Room(paylow.UserToFollow.SocketID)).Emit(
					"user-follow-room-change",
					followedby,
				)

			default:
				utils.Log().Printf("user %v sent unknown follow action %v for %v\n", me, paylow.Action, paylow.UserToFollow.SocketID)

			}
		})

		socket.On("disconnecting", func(datas ...any) {
			for _, currentRoom := range socket.Rooms().Keys() {
				ioo.In(currentRoom).Timeout(timeout).FetchSockets()(func(usersInRoom []*socketio.RemoteSocket, err error) {
					if err != nil {
						utils.Log().Printf("Error fetching sockets in room when disconnecting %v: %v\n", currentRoom, err)
						return
					}
					utils.Log().Printf("disconnecting %v from room %v\n", me, currentRoom)

					localUsers := []socketio.SocketId{}
					for _, user := range usersInRoom {
						localUsers = append(localUsers, user.Id())
					}

					if config.HAActive {

						// sync room to redis
						allUserInRoom, err := rds.SyncStoredUsersInRoomWithLocal(redisClient, currentRoom, localUsers, timeout)
						if err != nil {
							utils.Log().Printf("Error syncing users in room %v to redis: %v\n", currentRoom, err)
							return
						}
						localUsers = allUserInRoom
					}

					otherClients := []socketio.SocketId{}
					for _, user := range localUsers {
						if user != me {
							otherClients = append(otherClients, user)
						}
					}

					if config.HAActive {
						// remove disconnected user from redis stored users
						err = rds.RemoveUserFromStoredUsersInRoom(redisClient, currentRoom, me, timeout)
						if err != nil {
							utils.Log().Printf("Error removing user %v from stored users in room %v: %v\n", me, currentRoom, err)
							return
						}
					}

					isFollowRoom := strings.HasPrefix(string(currentRoom), "follow@")

					if !isFollowRoom && len(otherClients) > 0 {
						utils.Log().Printf("leaving user, room %v has users  %v\n", currentRoom, otherClients)
						ioo.In(currentRoom).Emit(
							"room-user-change",
							otherClients,
						)
					}
					if isFollowRoom && len(otherClients) <= 0 {
						fsockerID := strings.Replace(string(currentRoom), "follow@", "", -1)
						rFsocketID := socketio.Room(fsockerID)
						ioo.To(rFsocketID).Emit("broadcast-unfollow")
					}
				})
			}
		})

		socket.On("disconnect", func(datas ...any) {
			socket.RemoveAllListeners("")
			socket.Disconnect(true)
			utils.Log().Printf("Socket %v disconnected\n", me)
		})
	})
	return ioo, nil
}

func waitForShutdown(ioo *socketio.Server, redisclient *rds.Client) {
	exit := make(chan struct{})
	SignalC := make(chan os.Signal, 1)

	signal.Notify(SignalC, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for s := range SignalC {
			switch s {
			case os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				close(exit)
				return
			}
		}
	}()

	<-exit
	ioo.Close(nil)
	redisclient.CloseClient()
	os.Exit(0)
	fmt.Println("Shutting down...")
	// TODO(patwie): Close other resources
	os.Exit(0)
}

func main() {

	// Load configuration
	config := config.New()

	// Define a log level flag
	logLevel := config.LogLevel
	port := config.Port
	host := config.Host
	haActive := config.HAActive

	log.DEBUG = false

	listenAddr := fmt.Sprintf("%s:%s", host, port)
	// Set the log level
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid log level: %v\n", err)
		os.Exit(1)
	}
	logrus.SetLevel(level)
	// Initialize Redis client
	var rClient = &rds.Client{}
	if haActive {
		logrus.Info("HA is active: Using Redis for socketio and caching")

		// Initialize Redis client
		rClient.InitRedisClient(config.Redis)

	} else {
		logrus.Info("HA is not active: Running in single-instance mode")

	}

	// Setting up Socket.IO server
	opts := socketio.DefaultServerOptions()
	opts.SetTransports(types.NewSet(engine.WebSocket, engine.Polling))
	opts.SetMaxHttpBufferSize(5000000)
	opts.SetAllowEIO3(false)
	opts.SetAllowUpgrades(true)
	opts.SetCors(&types.Cors{
		Origin:      strings.Join(config.CorsAllowedOrigins(), ", "),
		Credentials: true,
	})

	timeoutInMili := int64(10000) // 10s
	heartbeatMili := int64(20000) // 20s
	timeout := time.Duration(timeoutInMili) * time.Millisecond
	heartbeat := time.Duration(heartbeatMili) * time.Millisecond
	opts.SetConnectTimeout(timeout + heartbeat)
	opts.SetPingTimeout(timeout)
	opts.SetPingInterval(heartbeat)

	if config.HAActive {
		//setup redis adapter for socket.io
		var err error
		connCtx := context.TODO()

		streamClient, err := rds.NewStreamClient(connCtx, rClient)

		if err != nil {
			logrus.Fatalf("Failed to create Redis adapter client: %v", err)
		}
		aOpts := adapter.DefaultRedisStreamsAdapterOptions()
		aOpts.SetMaxLen(10_000)
		aOpts.SetHeartbeatInterval(heartbeat)
		aOpts.SetHeartbeatTimeout(timeoutInMili)
		opts.SetAdapter(&adapter.RedisStreamsAdapterBuilder{
			Redis: streamClient.RedisAdapterClient,
			Opts:  aOpts,
		})
	}

	documentStore := stores.GetStore(config) // Make sure this is well-defined in your "stores" package

	ioo, err := setupSocketIO(config, opts, rClient, timeout)
	if err != nil {
		logrus.Fatalf("Failed to set up SocketIO: %v", err)
	}

	// setup router
	r := setupRouter(config, documentStore, rClient)

	r.Handle("/socket.io/", ioo.ServeHandler(opts))
	r.Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte("pong"))
		if err != nil {
			panic(err)
		}
	})
	r.Mount("/", handleUI(config))

	logrus.WithField("addr", listenAddr).Info("starting server")
	go func() {
		if err := http.ListenAndServe(listenAddr, r); err != nil {
			logrus.WithField("event", "start server").Fatal(err)
		}
	}()

	logrus.Debug("Server is running in the background")
	waitForShutdown(ioo, rClient)

}
