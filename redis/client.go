package redis

import (
	"context"
	"errors"
	"excalidraw-complete/config"
	"strconv"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// Client wraps the Redis client
type Client struct {
	RedisClient *redis.Client
}

// InitRedisClient creates a new RedisClient instance
func (c *Client) InitRedisClient(cfg config.Redis) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Host + ":" + strconv.Itoa(cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	c.RedisClient = rdb
}

// getClient retrieves the Redis client, ensuring it's initialized
func (c *Client) getClient(ctx context.Context) (client *redis.Client, err error) {
	if c.RedisClient == nil {
		return nil, errors.New("Redis client is not initialized")
	}
	_, err = c.RedisClient.Ping(ctx).Result()
	if err != nil {
		logrus.Errorf("Failed to connect to Redis: %v", err)
		return nil, err
	}
	return c.RedisClient, nil
}

// CloseClient closes the Redis client connection.
func (c *Client) CloseClient() error {
	if c != nil && c.RedisClient != nil {
		err := c.RedisClient.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
