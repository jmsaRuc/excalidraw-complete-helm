package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	socketio "github.com/zishang520/socket.io/servers/socket/v3"
)

// CacheStore provides methods to interact with Redis for caching.
type CacheStore struct {
	Ctx    context.Context
	client *redis.Client
}

// NewCacheStore creates a new CacheStore instance
func NewCacheStore(ctx context.Context, redisClient *Client) (*CacheStore, error) {
	client, err := redisClient.getClient(ctx)
	if err != nil {
		logrus.Errorf("CachStore failed to get RedisClient: %v", err)
		return nil, err
	}
	return &CacheStore{
		Ctx:    ctx,
		client: client,
	}, nil
}

// Set is a helper method to set a key-value pair in Redis with an optional expiration.
func (c *CacheStore) Set(key string, value interface{}, expiration time.Duration) error {
	statusCmd := c.client.Set(c.Ctx, key, value, expiration)
	if statusCmd.Err() != nil {
		return statusCmd.Err()
	}
	return nil
}

// Get is a helper method to get a value from Redis by key.
func (c *CacheStore) Get(key string) (string, error) {
	redisCmd := c.client.Get(c.Ctx, key)
	value, err := redisCmd.Result()
	if err != nil {
		return "", err
	}
	return value, nil
}

// GetSavedData retrieves saved items from Redis for the specified key.
func (c *CacheStore) GetSavedData(key string) (map[string]interface{}, error) {
	existingFields, err := c.Get(key)
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var savedItems map[string]any
	err = json.Unmarshal([]byte(existingFields), &savedItems)
	if err != nil {
		return nil, err
	}
	return savedItems, nil
}

// SetSavedData saves the provided items in Redis under the specified key.
func (c *CacheStore) SetSavedData(key string, savedItems map[string]interface{}) error {
	fieldsByte, err := json.Marshal(savedItems)
	if err != nil {
		return err
	}
	err = c.Set(key, string(fieldsByte), 0)
	if err != nil {
		return err
	}
	return nil
}

// used for syncing socket.io rooms in HA mode

// GetAllUsersInRoom retrieves all users in a room from Redis for the specified key.
func (c *CacheStore) GetAllUsersInRoom(room socketio.Room) ([]socketio.SocketId, error) {
	existingFields, err := c.Get(string(room))
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	savedItems := []socketio.SocketId{}
	err = json.Unmarshal([]byte(existingFields), &savedItems)
	if err != nil {
		return nil, err
	}
	return savedItems, nil
}

// SetAllUsersInRoom saves the provided items in Redis under the specified key.
func (c *CacheStore) SetAllUsersInRoom(room socketio.Room, savedItems []socketio.SocketId) error {
	fieldsByte, err := json.Marshal(savedItems)
	if err != nil {
		return err
	}
	err = c.Set(string(room), string(fieldsByte), 0)
	if err != nil {
		return err
	}
	return nil
}
