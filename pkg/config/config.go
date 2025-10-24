package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration values loaded from environment variables
type Config struct {
	// Redis configuration
	RedisHost     string
	RedisPort     int
	RedisPassword string
	RedisDB       int

	// RADIUS server configuration
	AuthPort string
	AcctPort string
	Secret   string

	// Data retention configuration
	AccountingTTL time.Duration

	// Server configuration
	ServerHost string
}

// LoadConfig reads environment variables and returns a populated Config struct
func LoadConfig() (*Config, error) {
	config := &Config{
		// Default values
		RedisHost:     "localhost",
		RedisPort:     6379,
		RedisPassword: "",
		RedisDB:       0,
		AuthPort:      ":1812",
		AcctPort:      ":1813",
		Secret:        "testing123",
		AccountingTTL: 10 * time.Minute,
		ServerHost:    "",
	}

	// Redis Host
	if host := os.Getenv("REDIS_HOST"); host != "" {
		config.RedisHost = host
	}

	// Redis Port
	if portStr := os.Getenv("REDIS_PORT"); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid REDIS_PORT: %v", err)
		}
		config.RedisPort = port
	}

	// Redis Password
	if password := os.Getenv("REDIS_PASSWORD"); password != "" {
		config.RedisPassword = password
	}

	// Redis DB
	if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
		db, err := strconv.Atoi(dbStr)
		if err != nil {
			return nil, fmt.Errorf("invalid REDIS_DB: %v", err)
		}
		config.RedisDB = db
	}

	// Auth Port
	if port := os.Getenv("AUTH_PORT"); port != "" {
		config.AuthPort = port
	}

	// Accounting Port
	if port := os.Getenv("ACCT_PORT"); port != "" {
		config.AcctPort = port
	}

	// RADIUS Secret
	if secret := os.Getenv("RADIUS_SECRET"); secret != "" {
		config.Secret = secret
	}

	// Accounting TTL
	if ttlStr := os.Getenv("ACCOUNTING_TTL_MINUTES"); ttlStr != "" {
		ttlMinutes, err := strconv.Atoi(ttlStr)
		if err != nil {
			return nil, fmt.Errorf("invalid ACCOUNTING_TTL_MINUTES: %v", err)
		}
		config.AccountingTTL = time.Duration(ttlMinutes) * time.Minute
	}

	// Server Host
	if host := os.Getenv("SERVER_HOST"); host != "" {
		config.ServerHost = host
	}

	return config, nil
}

// ConsumerConfig holds all configuration values for the Redis consumer
type ConsumerConfig struct {
	// Redis configuration
	RedisHost string
	RedisPort int

	// Consumer configuration
	Username string
	LogFile  string

	// Stream configuration
	StreamKey     string
	ConsumerGroup string
	ConsumerName  string
}

// LoadConsumerConfig reads environment variables and returns a populated ConsumerConfig struct
func LoadConsumerConfig() (*ConsumerConfig, error) {
	config := &ConsumerConfig{
		// Default values
		RedisHost: "localhost",
		RedisPort: 6379,
		LogFile:   "/var/log/radius_updates.log",
	}

	// Redis Host
	if host := os.Getenv("REDIS_HOST"); host != "" {
		config.RedisHost = host
	}

	// Redis Port
	if portStr := os.Getenv("REDIS_PORT"); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid REDIS_PORT: %v", err)
		}
		config.RedisPort = port
	}

	// Username (required)
	config.Username = os.Getenv("USERNAME")
	if config.Username == "" {
		return nil, fmt.Errorf("USERNAME environment variable is required")
	}

	// Log File
	if logFile := os.Getenv("LOG_FILE"); logFile != "" {
		config.LogFile = logFile
	}

	// Generate stream-related configuration based on username
	config.StreamKey = fmt.Sprintf("radius:updates:%s", config.Username)
	config.ConsumerGroup = fmt.Sprintf("consumer-group-%s", config.Username)
	config.ConsumerName = fmt.Sprintf("consumer-%s-%d", config.Username, os.Getpid())

	return config, nil
}
