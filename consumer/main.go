package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
)

type Consumer struct {
	client       *redis.Client
	username     string
	logFile      string
	streamKey    string
	groupName    string
	consumerName string
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewConsumer(redisHost, redisPort, username, logFile string) *Consumer {
	ctx, cancel := context.WithCancel(context.Background())

	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", redisHost, redisPort),
		Password: "",
		DB:       0, // default DB
	})

	return &Consumer{
		client:       rdb,
		username:     username,
		logFile:      logFile,
		streamKey:    fmt.Sprintf("radius:updates:%s", username),
		groupName:    fmt.Sprintf("consumer-group-%s", username),
		consumerName: fmt.Sprintf("consumer-%s-%d", username, os.Getpid()),
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (c *Consumer) writeLog(message string) error {
	logDir := "/var/log"
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %v", err)
		}
	}

	file, err := os.OpenFile(c.logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}
	defer file.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05.000000")
	logEntry := fmt.Sprintf("%s - %s\n", timestamp, message)

	if _, err := file.WriteString(logEntry); err != nil {
		return fmt.Errorf("failed to write to log file: %v", err)
	}

	return nil
}

func (c *Consumer) createConsumerGroup() error {
	err := c.client.XGroupCreateMkStream(c.ctx, c.streamKey, c.groupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("failed to create consumer group: %v", err)
	}
	log.Printf("Consumer group '%s' created/verified for stream '%s'", c.groupName, c.streamKey)
	return nil
}

func (c *Consumer) connectWithRetry() error {

	_, err := c.client.Ping(c.ctx).Result()
	if err == nil {
		log.Printf("Successfully connected to Redis")
		return nil
	}

	log.Printf("Failed to connect to Redis: %v", err)

	return fmt.Errorf("failed to connect to Redis")
}

func (c *Consumer) processMessage(msg redis.XMessage) error {
	keyName, exists := msg.Values["key"]
	if !exists {
		return fmt.Errorf("no 'key' field in message")
	}

	logMessage := fmt.Sprintf("Received update for key: %s", keyName)
	if err := c.writeLog(logMessage); err != nil {
		return fmt.Errorf("failed to write log: %v", err)
	}

	log.Printf("[%s] %s", c.username, logMessage)

	err := c.client.XAck(c.ctx, c.streamKey, c.groupName, msg.ID).Err()
	if err != nil {
		return fmt.Errorf("failed to acknowledge message: %v", err)
	}

	return nil
}

func (c *Consumer) Start() error {
	log.Printf("Starting Redis consumer for username: %s", c.username)
	log.Printf("Stream key: %s", c.streamKey)
	log.Printf("Log file: %s", c.logFile)

	if err := c.connectWithRetry(); err != nil {
		return err
	}

	if err := c.createConsumerGroup(); err != nil {
		return err
	}

	log.Printf("Consumer group '%s' ready", c.groupName)

	for {
		select {
		case <-c.ctx.Done():
			log.Printf("Consumer shutting down...")
			return nil
		default:
			msgs, err := c.client.XReadGroup(c.ctx, &redis.XReadGroupArgs{
				Group:    c.groupName,
				Consumer: c.consumerName,
				Streams:  []string{c.streamKey, ">"},
				Count:    1,
				Block:    time.Second * 5,
			}).Result()

			if err != nil {
				if err == redis.Nil {
					continue
				}

				log.Printf("Error reading from stream: %v", err)

				// Try to reconnect
				if err := c.connectWithRetry(); err != nil {
					log.Printf("Failed to reconnect: %v", err)
					time.Sleep(time.Second * 5)
					continue
				}
				continue
			}

			for _, stream := range msgs {
				for _, msg := range stream.Messages {
					if err := c.processMessage(msg); err != nil {
						log.Printf("Error processing message %s: %v", msg.ID, err)
					}
				}
			}
		}
	}
}

func (c *Consumer) Stop() {
	log.Printf("Stopping consumer...")
	c.cancel()
	c.client.Close()
}

func main() {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}

	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	username := os.Getenv("USERNAME")
	if username == "" {
		log.Fatal("USERNAME environment variable is required")
	}

	logFile := os.Getenv("LOG_FILE")
	if logFile == "" {
		logFile = "/var/log/radius_updates.log"
	}

	log.Printf("Starting Redis consumer with config:")
	log.Printf("  Redis Host: %s", redisHost)
	log.Printf("  Redis Port: %s", redisPort)
	log.Printf("  Username: %s", username)
	log.Printf("  Log File: %s", logFile)

	consumer := NewConsumer(redisHost, redisPort, username, logFile)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		errChan <- consumer.Start()
	}()

	select {
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down...", sig)
		consumer.Stop()
	case err := <-errChan:
		if err != nil {
			log.Printf("Consumer error: %v", err)
			consumer.Stop()
			os.Exit(1)
		}
	}

	log.Printf("Consumer stopped")
}
