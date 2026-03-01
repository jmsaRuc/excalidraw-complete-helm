package redis

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/zishang520/socket.io/adapters/redis/v3"
)

// StreamClient provides methods to interact with Redis for Socket.IO streams.
type StreamClient struct {
	Ctx                context.Context
	RedisAdapterClient *redis.RedisClient
}

// NewStreamClient creates a new StreamClient instance
func NewStreamClient(ctx context.Context, redisClient *Client) (*StreamClient, error) {

	client, err := redisClient.getClient(ctx)
	if err != nil {
		logrus.Errorf("StreamClient failed to get RedisClient: %v", err)
		return nil, err
	}

	redisAdapterClient := redis.NewRedisClient(ctx, client)
	redisAdapterClient.On("error", func(a ...any) {
		logrus.Errorf("Redis adapter client error: %v", a)
	})

	return &StreamClient{
		Ctx:                ctx,
		RedisAdapterClient: redisAdapterClient,
	}, nil
}

// CloseAdapterClient closes the Redis adapter client connection.
func (s *StreamClient) CloseAdapterClient() error {
	if s != nil && s.RedisAdapterClient != nil {
		err := s.RedisAdapterClient.Client.Close()
		if err != nil {
			logrus.Errorf("Failed to close Redis adapter client: %v", err)
			return err
		}
	}
	return nil
}
