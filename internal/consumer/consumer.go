package consumer

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	clock "go.llib.dev/testcase/clock"

	"dni/pkg/config"
	"dni/pkg/stream"
)

type Consumer struct {
	streamClient stream.Stream
	username     string
	logFile      string
	streamKey    string
	groupName    string
	consumerName string
	ctx          context.Context
	cancel       context.CancelFunc
}

// New creates a new Consumer with provided Redis client and stream client
func New(cfg *config.ConsumerConfig, streamClient stream.Stream) *Consumer {
	ctx, cancel := context.WithCancel(context.Background())

	return &Consumer{
		streamClient: streamClient,
		username:     cfg.Username,
		logFile:      cfg.LogFile,
		streamKey:    cfg.StreamKey,
		groupName:    cfg.ConsumerGroup,
		consumerName: cfg.ConsumerName,
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

	timestamp := clock.Now().Format("2006-01-02 15:04:05.000000")
	logEntry := fmt.Sprintf("%s - %s\n", timestamp, message)

	if _, err := file.WriteString(logEntry); err != nil {
		return fmt.Errorf("failed to write to log file: %v", err)
	}

	return nil
}

func (c *Consumer) processKeys(keys []string) error {
	for _, key := range keys {
		logMessage := fmt.Sprintf("Received update for key: %s", key)
		if err := c.writeLog(logMessage); err != nil {
			return fmt.Errorf("failed to write log: %v", err)
		}
		log.Printf("[%s] %s", c.username, logMessage)
	}
	return nil
}

func (c *Consumer) Start() error {
	log.Printf("Starting Redis consumer for username: %s", c.username)
	log.Printf("Stream key: %s", c.streamKey)
	log.Printf("Log file: %s", c.logFile)

	config := stream.ConsumerConfig{
		StreamKey:     c.streamKey,
		ConsumerGroup: c.groupName,
		ConsumerName:  c.consumerName,
	}

	log.Printf("Consumer group '%s' ready", c.groupName)

	for {
		select {
		case <-c.ctx.Done():
			log.Printf("Consumer shutting down...")
			return nil
		default:
			keys, err := c.streamClient.Pull(config)
			if err != nil {
				log.Printf("Error reading from stream: %v", err)
				time.Sleep(time.Second * 5)
				continue
			}

			if len(keys) > 0 {
				if err := c.processKeys(keys); err != nil {
					log.Printf("Error processing keys: %v", err)
				}
			}

			// Small delay to prevent busy waiting
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (c *Consumer) Stop() {
	log.Printf("Stopping consumer...")
	c.cancel()
}
