package redis

import (
	"context"
	"encoding/json"
	"excalidraw-complete/config"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheStore provides methods to interact with Redis for caching.
type CacheStore struct {
	Ctx         context.Context
	RedisClient *redis.Client
}

// NewCacheStore creates a new CacheStore instance
func NewCacheStore(ctx context.Context, cfg config.Redis) *CacheStore {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Host + ":" + strconv.Itoa(cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	return &CacheStore{
		Ctx:         ctx,
		RedisClient: rdb,
	}
}

func (c *CacheStore) set(key string, value interface{}, expiration time.Duration) error {
	statusCmd := c.RedisClient.Set(c.Ctx, key, value, expiration)
	if statusCmd.Err() != nil {
		return statusCmd.Err()
	}
	return nil
}

func (c *CacheStore) get(key string) (string, error) {
	redisCmd := c.RedisClient.Get(c.Ctx, key)
	value, err := redisCmd.Result()
	if err != nil {
		return "", err
	}
	return value, nil
}

// GetSavedData retrieves saved items from Redis for the specified key.
func (c *CacheStore) GetSavedData(key string) (map[string]interface{}, error) {
	existingFields, err := c.get(key)
	if err != nil && err.Error() == string(redis.Nil) {
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
	err = c.set(key, string(fieldsByte), 0)
	if err != nil {
		return err
	}
	return nil
}

// CloseCacheStore closes the Redis client connection.
func (c *CacheStore) CloseCacheStore() {
	if c != nil {
		_ = c.RedisClient.Close()
	}
	_ = c.RedisClient.Close()
}
