package redis

import (
	"context"
	"crypto/md5"
	"strconv"
	"testing"

	"excalidraw-complete/config"

	"github.com/alicebob/miniredis/v2"
	socketio "github.com/zishang520/socket.io/v2/socket"
)

func TestUnMarshalledPublishedData(t *testing.T) {
	// Test valid JSON unmarshalling
	jsonStr := `{"origin":"test-origin","room":"test-room","event":"test-event","args":["arg1","arg2"]}`
	msg, err := UnMarshalledPublishedData([]byte(jsonStr))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if msg.Origin != "test-origin" {
		t.Errorf("expected origin 'test-origin', got %s", msg.Origin)
	}
	if msg.Room != "test-room" {
		t.Errorf("expected room 'test-room', got %s", msg.Room)
	}
	if msg.Event != "test-event" {
		t.Errorf("expected event 'test-event', got %s", msg.Event)
	}
	if len(msg.Args) != 2 || msg.Args[0] != "arg1" || msg.Args[1] != "arg2" {
		t.Errorf("expected args ['arg1','arg2'], got %v", msg.Args)
	}

	// Test invalid JSON
	_, err = UnMarshalledPublishedData([]byte(`invalid json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMarshalToUnMarshalBinary(t *testing.T) {
	data1 := []byte("testing arg 1")
	data2 := []byte("testing arg 2")
	bOriginal1 := md5.Sum(data1)
	bOriginal2 := md5.Sum(data2)
	test := []any{data1, data2}

	msg := Message{
		Origin: "origin-id",
		Room:   "room-id",
		Event:  "event-name",
		Args:   test,
	}

	marshalled, err := msg.MarshalBinary()
	if err != nil {
		t.Fatalf("expected no error during marshalling, got %v", err)
	}

	unmarshalledMsg, err := UnMarshalledPublishedData(marshalled)
	if err != nil {
		t.Fatalf("expected no error during unmarshalling, got %v", err)
	}

	if unmarshalledMsg.Origin != msg.Origin {
		t.Errorf("expected origin %s, got %s", msg.Origin, unmarshalledMsg.Origin)
	}
	if unmarshalledMsg.Room != msg.Room {
		t.Errorf("expected room %s, got %s", msg.Room, unmarshalledMsg.Room)
	}
	if unmarshalledMsg.Event != msg.Event {
		t.Errorf("expected event %s, got %s", msg.Event, unmarshalledMsg.Event)
	}
	nohexMsg, err := unmarshalledMsg.hexDecode()
	if err != nil {
		t.Fatalf("expected no error during hex decoding, got %v", err)
	}
	for _, value := range nohexMsg.Args {
		bite, ok := value.([]byte)
		if ok {
			b := md5.Sum(bite)
			if b != bOriginal1 && b != bOriginal2 {
				t.Errorf("byte argument does not match original data")
			}
		} else {
			t.Errorf("expected []byte argument, got %T", value)
		}

	}
}

func TestNewPubSub(t *testing.T) {
	ctx := context.Background()
	cfg := config.Redis{
		Host:     "localhost",
		Port:     6379,
		Password: "",
		DB:       0,
	}
	srv := socketio.NewServer(nil, nil) // Mock server for testing
	ps := NewPubSub(ctx, srv, cfg)

	if ps.ctx != ctx {
		t.Errorf("expected context to be set")
	}
	if ps.server != srv {
		t.Errorf("expected server to be set")
	}
	if ps.channelPrefix != "sio:broadcast:" {
		t.Errorf("expected channelPrefix 'sio:broadcast:', got %s", ps.channelPrefix)
	}
	if ps.instanceID == "" {
		t.Errorf("expected instanceID to be set")
	}
}

// TestPublishRoom uses miniredis for mocking Redis
func TestPublishRoom(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	portInt, err := strconv.Atoi(s.Port())
	if err != nil {
		t.Fatalf("failed to convert port to int: %v", err)
	}
	cfg := config.Redis{
		Host:     s.Host(),
		Port:     portInt,
		Password: "",
		DB:       0,
	}
	srv := socketio.NewServer(nil, nil)
	ps := NewPubSub(ctx, srv, cfg)

	// Test publishing a message
	err = ps.PublishRoom("test-room", "test-event", "arg1", "arg2")
	if err != nil {
		t.Fatalf("expected no error publishing, got %v", err)
	}

	ps.Close()
}

// Integration test for pub/sub system using miniredis
func TestPubSubIntegration(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	// Convert port to int
	portInt, err := strconv.Atoi(s.Port())
	if err != nil {
		t.Fatalf("failed to convert port to int: %v", err)
	}
	cfg := config.Redis{
		Host:     s.Host(),
		Port:     portInt,
		Password: "",
		DB:       0,
	}
	srv := socketio.NewServer(nil, nil)
	ps := NewPubSub(ctx, srv, cfg)

	// Start the pub/sub
	err = ps.Start()
	if err != nil {
		t.Fatalf("expected no error starting pub/sub, got %v", err)
	}

	// Publish a message
	err = ps.PublishRoom("test-room", "test-event", "arg1")
	if err != nil {
		t.Fatalf("expected no error publishing, got %v", err)
	}

	ps.Close()
}

func TestClose(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer s.Close()

	//convert port to int
	portInt, err := strconv.Atoi(s.Port())
	if err != nil {
		t.Fatalf("failed to convert port to int: %v", err)
	}
	ctx := context.Background()
	cfg := config.Redis{
		Host:     s.Host(),
		Port:     portInt,
		Password: "",
		DB:       0,
	}
	srv := socketio.NewServer(nil, nil)
	ps := NewPubSub(ctx, srv, cfg)

	// Close should not panic
	ps.Close()
}

func TestNewCacheStore(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	portInt, err := strconv.Atoi(s.Port())
	if err != nil {
		t.Fatalf("failed to convert port to int: %v", err)
	}
	cfg := config.Redis{
		Host:     s.Host(),
		Port:     portInt,
		Password: "",
		DB:       0,
	}
	cs := NewCacheStore(ctx, cfg)

	if cs.Ctx != ctx {
		t.Errorf("expected context to be set")
	}
	if cs.RedisClient == nil {
		t.Errorf("expected redis client to be set")
	}

	cs.CloseCacheStore()
}

func TestSetSavedData(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	portInt, err := strconv.Atoi(s.Port())
	if err != nil {
		t.Fatalf("failed to convert port to int: %v", err)
	}
	cfg := config.Redis{
		Host:     s.Host(),
		Port:     portInt,
		Password: "",
		DB:       0,
	}
	cs := NewCacheStore(ctx, cfg)

	data := map[string]interface{}{"key": "value"}
	err = cs.SetSavedData("test", data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	cs.CloseCacheStore()
}

func TestGetSavedData(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	portInt, err := strconv.Atoi(s.Port())
	if err != nil {
		t.Fatalf("failed to convert port to int: %v", err)
	}
	cfg := config.Redis{
		Host:     s.Host(),
		Port:     portInt,
		Password: "",
		DB:       0,
	}
	cs := NewCacheStore(ctx, cfg)

	data := map[string]interface{}{"key": "value"}
	err = cs.SetSavedData("test", data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	retrieved, err := cs.GetSavedData("test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if retrieved["key"] != "value" {
		t.Errorf("expected 'value', got %v", retrieved["key"])
	}

	// Test non-existing key
	retrieved, err = cs.GetSavedData("nonexist")
	if err != nil {
		t.Fatalf("expected no error for non-existing key, got %v", err)
	}
	if retrieved != nil {
		t.Errorf("expected nil for non-existing key, got %v", retrieved)
	}

	cs.CloseCacheStore()
}
