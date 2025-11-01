package stream

import (
	"context"
	"fmt"
	"time"

	clock "go.llib.dev/testcase/clock"

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
	values := []interface{}{
		"key", message.Key,
		"timestamp", clock.Now().Unix(),
		"username", message.Username,
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

// Pull consumes messages from a Redis stream using consumer groups and returns keys
func (rs *RedisStream) Pull(config ConsumerConfig) ([]string, error) {
	// Initialize the consumer group (create stream and consumer group if they don't exist)
	err := rs.initializeConsumerGroup(config.StreamKey, config.ConsumerGroup)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize consumer group: %v", err)
	}

	// Read messages from the consumer group
	streams, err := rs.client.XReadGroup(rs.ctx, &redis.XReadGroupArgs{
		Group:    config.ConsumerGroup,
		Consumer: config.ConsumerName,
		Streams:  []string{config.StreamKey, ">"},
		Count:    10,              // Read up to 10 messages at once
		Block:    time.Second * 5, // Block for 5 seconds if no messages
	}).Result()

	if err != nil {
		if err == redis.Nil {
			// No messages available
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read from stream: %v", err)
	}

	var keys []string

	// Process each stream (should only be one in our case)
	for _, stream := range streams {
		for _, msg := range stream.Messages {
			// Extract the key from the message
			if key, ok := msg.Values["key"].(string); ok {
				keys = append(keys, key)
			}

			// Acknowledge the message
			err = rs.client.XAck(rs.ctx, config.StreamKey, config.ConsumerGroup, msg.ID).Err()
			if err != nil {
				fmt.Printf("Failed to acknowledge message %s: %v\n", msg.ID, err)
			}
		}
	}

	return keys, nil
}

// initializeConsumerGroup creates the consumer group if it doesn't exist
func (rs *RedisStream) initializeConsumerGroup(streamKey, consumerGroup string) error {
	// Try to create the consumer group with MKSTREAM option
	// This will create the stream if it doesn't exist
	err := rs.client.XGroupCreateMkStream(rs.ctx, streamKey, consumerGroup, "$").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("failed to create consumer group %s for stream %s: %v", consumerGroup, streamKey, err)
	}
	return nil
}
