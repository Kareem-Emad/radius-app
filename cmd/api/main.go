package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"dni/pkg/config"
	"dni/internal/accounting"
	"dni/internal/auth"
	"dni/pkg/datastore"
	"dni/pkg/stream"

	"github.com/go-redis/redis/v8"
	"layeh.com/radius"
)

var redisClient *redis.Client
var ctx = context.Background()
var datastoreClient datastore.Datastore
var streamClient stream.Stream
var cfg *config.Config

func initRedis(config *config.Config) error {
	redisAddr := fmt.Sprintf("%s:%d", config.RedisHost, config.RedisPort)
	redisClient = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})

	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %v", err)
	}

	// Initialize the interface implementations
	datastoreClient = datastore.NewRedisStore(redisClient)
	streamClient = stream.NewRedisStream(redisClient)

	log.Printf("Connected to Redis at %s", redisAddr)
	return nil
}

func main() {
	// Load configuration from environment variables
	var err error
	cfg, err = config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize Redis connection
	if err := initRedis(cfg); err != nil {
		log.Fatalf("Failed to initialize Redis: %v", err)
	}
	defer redisClient.Close()

	// Get secret from configuration
	secret := []byte(cfg.Secret)

	// Create handlers
	authHandler := auth.NewHandler(secret)
	acctHandler := accounting.NewHandler(datastoreClient, streamClient, cfg.AccountingTTL)

	// Create servers
	authServer := &radius.PacketServer{
		Addr:         cfg.AuthPort,
		Handler:      radius.HandlerFunc(authHandler.Handle),
		SecretSource: radius.StaticSecretSource(secret),
	}

	acctServer := &radius.PacketServer{
		Addr:         cfg.AcctPort,
		Handler:      radius.HandlerFunc(acctHandler.Handle),
		SecretSource: radius.StaticSecretSource(secret),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Start authentication server
	go func() {
		defer wg.Done()
		log.Printf("Starting Authentication server on %s", cfg.AuthPort)
		if err := authServer.ListenAndServe(); err != nil {
			log.Fatalf("Authentication server error: %v", err)
		}
	}()

	// Start accounting server
	go func() {
		defer wg.Done()
		log.Printf("Starting Accounting server on %s", cfg.AcctPort)
		if err := acctServer.ListenAndServe(); err != nil {
			log.Fatalf("Accounting server error: %v", err)
		}
	}()

	wg.Wait()
}
