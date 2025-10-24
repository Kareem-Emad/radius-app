package stream

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisStream implements the Stream interface using Redis streams
type RedisStream struct {
	client *redis.Client
	ctx    context.Context
}

// NewRedisStream creates a new RedisStream instance
func NewRedisStream(client *redis.Client) *RedisStream {
	return &RedisStream{
		client: client,
		ctx:    context.Background(),
	}
}

// Push publishes a message to a Redis stream
func (rs *RedisStream) Push(streamKey string, message StreamMessage) error {
	// Create stream message with Redis-compatible values
	values := map[string]interface{}{
		"key":       message.Key,
		"timestamp": time.Now().Unix(),
		"username":  message.Username,
	}

	// Add any additional data from the message
	for k, v := range message.Data {
		values[k] = v
	}

	// Add message to stream
	_, err := rs.client.XAdd(rs.ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: values,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to publish to stream %s: %v", streamKey, err)
	}

	return nil
}
