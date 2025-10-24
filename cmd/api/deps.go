package main

import (
	"context"
	"fmt"
	"log"

	"dni/internal/accounting"
	"dni/internal/auth"
	"dni/pkg/config"
	"dni/pkg/datastore"
	"dni/pkg/stream"

	"github.com/go-redis/redis/v8"
)

// Dependencies holds all initialized handlers and clients
type Dependencies struct {
	AuthHandler *auth.Handler
	AcctHandler *accounting.Handler
	RedisClient *redis.Client
}

// InitializeDependencies sets up all required dependencies based on configuration
func InitializeDependencies(cfg *config.Config) (*Dependencies, error) {
	ctx := context.Background()

	// Initialize Redis connection
	redisAddr := fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort)
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	// Test Redis connection
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	log.Printf("Connected to Redis at %s", redisAddr)

	// Initialize interface implementations
	datastoreClient := datastore.NewRedisStore(redisClient)
	streamClient := stream.NewRedisStream(redisClient)

	// Get secret from configuration
	secret := []byte(cfg.Secret)

	// Create handlers
	authHandler := auth.NewHandler(secret, cfg.UserCredentials)
	acctHandler := accounting.NewHandler(datastoreClient, streamClient, cfg.AccountingTTL)

	return &Dependencies{
		AuthHandler: authHandler,
		AcctHandler: acctHandler,
		RedisClient: redisClient,
	}, nil
}

// Close cleans up all resources
func (d *Dependencies) Close() error {
	if d.RedisClient != nil {
		return d.RedisClient.Close()
	}
	return nil
}
