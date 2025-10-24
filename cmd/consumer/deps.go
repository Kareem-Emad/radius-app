package main

import (
	"context"
	"fmt"
	"log"

	"dni/internal/consumer"
	"dni/pkg/config"
	"dni/pkg/stream"

	"github.com/go-redis/redis/v8"
)

// Dependencies holds all initialized consumer dependencies
type Dependencies struct {
	Consumer     *consumer.Consumer
	RedisClient  *redis.Client
	StreamClient stream.Stream
}

// InitializeDependencies sets up all required consumer dependencies
func InitializeDependencies(cfg *config.ConsumerConfig) (*Dependencies, error) {
	ctx := context.Background()

	// Initialize Redis connection
	redisAddr := fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort)
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       0, // default DB
	})

	// Test Redis connection
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	log.Printf("Connected to Redis at %s", redisAddr)

	// Initialize stream client
	streamClient := stream.NewRedisStream(redisClient)

	// Create consumer with dependencies
	c := consumer.New(cfg, streamClient)

	return &Dependencies{
		Consumer:     c,
		RedisClient:  redisClient,
		StreamClient: streamClient,
	}, nil
}

// Close cleans up all resources
func (d *Dependencies) Close() error {
	if d.Consumer != nil {
		d.Consumer.Stop()
	}
	if d.RedisClient != nil {
		return d.RedisClient.Close()
	}
	return nil
}
