package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"dni/pkg/config"
)

func main() {
	cfg, err := config.LoadConsumerConfig()
	if err != nil {
		log.Fatalf("Failed to load consumer config: %v", err)
	}

	consumerGroup := flag.String("group", cfg.ConsumerGroup, "Consumer Group")
	consumerName := flag.String("name", cfg.ConsumerName, "Consumer Name")
	username := flag.String("username", cfg.Username, "Username for the consumer")

	flag.Parse()

	if username != nil {
		cfg.Username = *username
		cfg.StreamKey = "radius:updates:" + cfg.Username
	}
	if consumerGroup != nil {
		cfg.ConsumerGroup = *consumerGroup
	}
	if consumerName != nil {
		cfg.ConsumerName = *consumerName
	}

	log.Printf("Starting Redis consumer with config:")
	log.Printf("  Redis Host: %s", cfg.RedisHost)
	log.Printf("  Redis Port: %d", cfg.RedisPort)
	log.Printf("  Username: %s", cfg.Username)
	log.Printf("  Log File: %s", cfg.LogFile)
	log.Printf("  Stream Key: %s", cfg.StreamKey)
	log.Printf("  Consumer Group: %s", cfg.ConsumerGroup)
	log.Printf("  Consumer Name: %s", cfg.ConsumerName)

	// Initialize all dependencies
	deps, err := InitializeDependencies(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize dependencies: %v", err)
	}
	defer deps.Close()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		errChan <- deps.Consumer.Start()
	}()

	select {
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down...", sig)
		deps.Consumer.Stop()
	case err := <-errChan:
		if err != nil {
			log.Printf("Consumer error: %v", err)
			deps.Consumer.Stop()
			os.Exit(1)
		}
	}

	log.Printf("Consumer stopped")
}
