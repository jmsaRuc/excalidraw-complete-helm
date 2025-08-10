package main

import (
	"embed"
	"excalidraw-complete/config"
	"excalidraw-complete/core"
	"excalidraw-complete/handlers/api/documents"
	"excalidraw-complete/handlers/api/firebase"
	"excalidraw-complete/stores"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/zishang520/engine.io/v2/types"
	"github.com/zishang520/engine.io/v2/utils"
	socketio "github.com/zishang520/socket.io/v2/socket"
)

type (
	UserToFollow struct {
		SocketId string `json:"socketId"`
		Username string `json:"username"`
	}

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
	baseUrl := strings.Split(config.FrontendURL, "://")[1]

	// Create a log field for the base URL and SSL status
	urlField := logrus.Fields{
		"baseUrl": baseUrl,
		"isSSL":   useSSL,
	}
	logrus.WithFields(urlField).Info("Use frontend URL")

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
		defer file.Close()

		fileContent, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "Error reading file", http.StatusInternalServerError)
			return
		}

		// Replace firebase URLs with the base URL
		// and adjust SSL settings if necessary
		modifiedContent := strings.ReplaceAll(string(fileContent), "firestore.googleapis.com", baseUrl)
		if useSSL {
			modifiedContent = strings.ReplaceAll(modifiedContent, "ssl=!0", "ssl=1")
			modifiedContent = strings.ReplaceAll(modifiedContent, "ssl:!0", "ssl:1")
		} else {
			modifiedContent = strings.ReplaceAll(modifiedContent, "ssl=1", "ssl=!0")
			modifiedContent = strings.ReplaceAll(modifiedContent, "ssl:1", "ssl:!0")
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
		return
	})
}

func setupRouter(documentStore core.DocumentStore) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "Content-Length", "X-CSRF-Token", "Token", "session", "Origin", "Host", "Connection", "Accept-Encoding", "Accept-Language", "X-Requested-With"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	r.Route("/v1/projects/{project_id}/databases/{database_id}", func(r chi.Router) {
		r.Post("/documents:commit", firebase.HandleBatchCommit())
		r.Post("/documents:batchGet", firebase.HandleBatchGet())
	})

	r.Route("/api/v2", func(r chi.Router) {
		r.Post("/post/", documents.HandleCreate(documentStore))
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", documents.HandleGet(documentStore))
		})
	})
	return r
}
func setupSocketIO(config *config.Config) *socketio.Server {
	opts := socketio.DefaultServerOptions()
	opts.SetMaxHttpBufferSize(5000000)
	opts.SetPath("/socket.io")
	opts.SetAllowEIO3(true)
	opts.SetCors(&types.Cors{
		Origin:      config.FrontendURL,
		Credentials: true,
	})
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
			ioo.In(room).FetchSockets()(func(usersInRoom []*socketio.RemoteSocket, _ error) {
				if len(usersInRoom) <= 1 {
					ioo.To(myRoom).Emit("first-in-room")
				} else {
					utils.Log().Printf("emit new user %v in room %v\n", me, room)
					socket.Broadcast().To(room).Emit("new-user", me)
				}

				// Inform all clients by new users.
				newRoomUsers := []socketio.SocketId{}
				for _, user := range usersInRoom {
					newRoomUsers = append(newRoomUsers, user.Id())
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
			utils.Log().Printf(" user %v sends update to room %v\n", me, roomID)
			socket.Broadcast().To(socketio.Room(roomID)).Emit("client-broadcast", datas[1], datas[2])
		})
		socket.On("server-volatile-broadcast", func(datas ...any) {
			roomID := datas[0].(string)
			utils.Log().Printf(" user %v sends volatile update to room %v\n", me, roomID)
			socket.Volatile().Broadcast().To(socketio.Room(roomID)).Emit("client-broadcast", datas[1], datas[2])
		})

		socket.On("user-follow", func(datas ...any) {
			// TODO()

		})
		socket.On("disconnecting", func(datas ...any) {
			for _, currentRoom := range socket.Rooms().Keys() {
				ioo.In(currentRoom).FetchSockets()(func(usersInRoom []*socketio.RemoteSocket, _ error) {
					otherClients := []socketio.SocketId{}
					utils.Log().Printf("disconnecting %v from room %v\n", me, currentRoom)
					for _, userInRoom := range usersInRoom {
						if userInRoom.Id() != me {
							otherClients = append(otherClients, userInRoom.Id())
						}
					}
					if len(otherClients) > 0 {
						utils.Log().Printf("leaving user, room %v has users  %v\n", currentRoom, otherClients)
						ioo.In(currentRoom).Emit(
							"room-user-change",
							otherClients,
						)

					}

				})

			}

		})
		socket.On("disconnect", func(datas ...any) {
			socket.RemoveAllListeners("")
			socket.Disconnect(true)
		})
	})
	return ioo

}

func waitForShutdown(ioo *socketio.Server) {
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

	listenAddr := fmt.Sprintf("%s:%s", host, port)
	// Set the log level
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid log level: %v\n", err)
		os.Exit(1)
	}
	logrus.SetLevel(level)

	documentStore := stores.GetStore(config) // Make sure this is well-defined in your "stores" package
	r := setupRouter(documentStore)
	ioo := setupSocketIO(config)
	r.Handle("/socket.io/", ioo.ServeHandler(nil))
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
	waitForShutdown(ioo)

}
