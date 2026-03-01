package redis

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	"excalidraw-complete/config"

	"github.com/oklog/ulid/v2"
	"github.com/redis/go-redis/v9"
	socketio "github.com/zishang520/socket.io/servers/socket/v3"
	"github.com/zishang520/socket.io/v3/pkg/utils"
)

// PubSub implements a Redis-based Pub/Sub system for Socket.IO
type PubSub struct {
	ctx           context.Context
	client        *redis.Client
	server        *socketio.Server
	instanceID    string
	channelPrefix string
	sub           *redis.PubSub
}

// Message represents a message published to a Redis channel.
type Message struct {
	Origin string            `json:"origin"`
	Room   socketio.Room     `json:"room"`
	Event  string            `json:"event"`
	User   socketio.SocketId `json:"user"`
	Args   []any             `json:"args"`
}

// NewPubSub creates a new PubSub instance
func NewPubSub(ctx context.Context, srv *socketio.Server, cfg config.Redis) *PubSub {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Host + ":" + strconv.Itoa(cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	return &PubSub{
		ctx:           ctx,
		client:        rdb,
		server:        srv,
		instanceID:    ulid.Make().String(),
		channelPrefix: "sio:broadcast:",
	}
}

// MarshalBinary marshals the Message into binary format for Redis publishing
func (m Message) MarshalBinary() (data []byte, err error) {
	if m.Args != nil {
		for i, arg := range m.Args {
			bite, ok := arg.([]byte)
			if ok {
				m.Args[i] = hex.EncodeToString(bite)

			}

		}
	}
	bytes, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func (m Message) hexDecode() (Message, error) {
	if m.Args != nil {
		for i, arg := range m.Args {
			str, ok := arg.(string)
			if !ok {
				continue
			}
			// only decode if it looks like hex; otherwise keep the string
			if len(str)%2 != 0 || !isHex(str) {
				continue
			}
			decoded, err := hex.DecodeString(str)
			if err != nil {
				continue // keep original string instead of failing the whole message
			}
			m.Args[i] = decoded
		}
	}
	return m, nil
}

// helper to check hex characters
func isHex(s string) bool {
	for _, c := range s {
		if !('0' <= c && c <= '9') && !('a' <= c && c <= 'f') && !('A' <= c && c <= 'F') {
			return false
		}
	}
	return true
}

func getPublishPayload(r *PubSub, room socketio.Room, user socketio.SocketId, event string, args ...any) ([]byte, error) {
	newMessage := Message{
		Origin: r.instanceID,
		Room:   room,
		Event:  event,
		User:   user,
		Args:   args,
	}
	messageByte, err := newMessage.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return messageByte, nil
}

// UnMarshalledPublishedData unmarshals the published Message from Redis
func UnMarshalledPublishedData(publishedMessage []byte) (*Message, error) {
	var newMessage Message
	err := json.Unmarshal(publishedMessage, &newMessage)
	if err != nil {
		return nil, err
	}
	return &newMessage, nil
}

// Start begins listening for messages from Redis and broadcasting them to Socket.IO clients
func (r *PubSub) Start() error {
	r.sub = r.client.PSubscribe(r.ctx, r.channelPrefix+"*")
	ch := r.sub.Channel()
	go func() {
		for msg := range ch {
			m, err := UnMarshalledPublishedData([]byte(msg.Payload))
			if err != nil {
				fmt.Printf("unmarshal published data: %v\n", err)
				continue
			}
			mNoHex, err := m.hexDecode()
			if err != nil {
				fmt.Printf("hex decode message args: %v\n", err)
				continue
			}

			// ignore messages from self
			if mNoHex.Origin == r.instanceID {
				continue
			}

			switch mNoHex.Event {
			case "init-room":
				utils.Log().Printf("published: init room %v\n", mNoHex.Room)
				r.server.To(mNoHex.Room).Emit("init-room")
			case "join-room":
				r.server.In(mNoHex.Room).FetchSockets()(func(usersInRoom []*socketio.RemoteSocket, _ error) {
					for _, s := range usersInRoom {
						if s.Id() == mNoHex.User {
							utils.Log().Printf("published: Socket %v has joined %v\n", mNoHex.User, mNoHex.Room)
							s.Join(mNoHex.Room)
							break
						}
					}
				})
			case "first-in-room":
				utils.Log().Printf("published: emit first user %v in room %v\n", mNoHex.User, mNoHex.Room)
				r.server.To(mNoHex.Room).Emit(mNoHex.Event)

			case "new-user":
				utils.Log().Printf("published: emit new user %v in room %v\n", mNoHex.User, mNoHex.Room)
				r.server.To(mNoHex.Room).Emit(mNoHex.Event, mNoHex.User)

			case "room-user-change":
				utils.Log().Printf("published: room %v has users %v", mNoHex.Room, mNoHex.Args[0])
				r.server.In(mNoHex.Room).Emit(mNoHex.Event, mNoHex.Args[0])

			case "client-broadcast":
				utils.Log().Printf("published: user %v sends update to room %v\n", mNoHex.User, mNoHex.Room)
				r.server.To(mNoHex.Room).Emit(mNoHex.Event, mNoHex.Args[0], mNoHex.Args[1])

			case "client-volatile-broadcast":
				utils.Log().Printf("published: user %v sends volatile update to room %v\n", mNoHex.User, mNoHex.Room)
				// dont use mNoHex.Event here, as we want to always use "client-broadcast" for volatile messages
				r.server.Volatile().To(mNoHex.Room).Emit("client-broadcast", mNoHex.Args[0], mNoHex.Args[1])

			case "room-user-change-leaving":
				utils.Log().Printf("published: leaving user, room %v has users %v (leaving)\n", mNoHex.Room, mNoHex.Args[0])
				// dont use mNoHex.Event here, as we want use "room-user-change" for this event
				r.server.In(mNoHex.Room).Emit("room-user-change", mNoHex.Args[0])

			case "user-left":
				utils.Log().Printf("published: user %v left room %v\n", mNoHex.User, mNoHex.Room)
				r.server.To(mNoHex.Room).Emit(mNoHex.Event)

			default:
				utils.Log().Printf("published: !DEFAULT EVENT! %v %v", mNoHex.Event, mNoHex.Room)
				r.server.To(mNoHex.Room).Emit(mNoHex.Event)
			}
		}
	}()
	return nil
}

// PublishRoom publishes a message to a specific room channel in Redis
func (r *PubSub) PublishRoom(room socketio.Room, user socketio.SocketId, event string, args ...any) error {
	payload, err := getPublishPayload(r, room, user, event, args...)
	if err != nil {
		return fmt.Errorf("marshal publish: %w", err)
	}
	return r.client.Publish(r.ctx, r.channelPrefix+string(room), payload).Err()
}

// Close closes the Redis Pub/Sub connection
func (r *PubSub) Close() {
	if r.sub != nil {
		_ = r.sub.Close()
	}
	_ = r.client.Close()
}
