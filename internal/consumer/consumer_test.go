package consumer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dni/pkg/config"
	"dni/pkg/stream"
)

// mockStream implements stream.Stream interface for testing
type mockStream struct {
	pullResults [][]string // each pullResult is a slice of message keys
	pullErrors  []error
	callCount   int
}

func (m *mockStream) Push(streamKey string, message stream.StreamMessage) error {
	return nil
}

func (m *mockStream) Pull(config stream.ConsumerConfig) ([]string, error) {
	if m.callCount >= len(m.pullResults) {
		return []string{}, nil
	}

	result := m.pullResults[m.callCount]
	var err error
	if m.callCount < len(m.pullErrors) {
		err = m.pullErrors[m.callCount]
	}

	m.callCount++
	return result, err
}

func (m *mockStream) CreateConsumerGroup(streamKey, groupName string) error {
	return nil
}

func TestConsumer_StartStop(t *testing.T) {
	t.Run("consumer can be started and stopped with cancel signal", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "consumer_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		logFile := filepath.Join(tempDir, "test.log")

		cfg := &config.ConsumerConfig{
			Username:      "testuser",
			LogFile:       logFile,
			StreamKey:     "radius:updates:testuser",
			ConsumerGroup: "test-group",
			ConsumerName:  "test-consumer",
		}

		mockStreamClient := &mockStream{
			pullResults: [][]string{{}}, // Empty result to avoid infinite loop
			pullErrors:  []error{nil},
		}

		consumer := New(cfg, mockStreamClient)

		done := make(chan error, 1)
		go func() {
			done <- consumer.Start()
		}()

		time.Sleep(200 * time.Millisecond)

		consumer.Stop()

		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Consumer.Start() returned error: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("Consumer did not stop within timeout")
		}
	})
}

func TestConsumer_MessageProcessing(t *testing.T) {
	tests := []struct {
		name             string
		pullResults      [][]string
		pullErrors       []error
		expectedLogParts []string
		expectLogFile    bool
	}{
		{
			name: "single message processing",
			pullResults: [][]string{
				{"radius:acct:testuser:session123"},
				{}, // Empty to stop the loop
			},
			pullErrors: []error{nil, nil},
			expectedLogParts: []string{
				"Received update for key: radius:acct:testuser:session123",
			},
			expectLogFile: true,
		},
		{
			name: "multiple messages processing",
			pullResults: [][]string{
				{"radius:acct:testuser:session123", "radius:acct:testuser:session456"},
				{}, // Empty to stop the loop
			},
			pullErrors: []error{nil, nil},
			expectedLogParts: []string{
				"Received update for key: radius:acct:testuser:session123",
				"Received update for key: radius:acct:testuser:session456",
			},
			expectLogFile: true,
		},
		{
			name: "stream pull error handling",
			pullResults: [][]string{
				{}, // Empty result when error occurs
			},
			pullErrors: []error{
				fmt.Errorf("Redis connection failed"),
			},
			expectedLogParts: []string{},
			expectLogFile:    false,
		},
		{
			name: "no messages",
			pullResults: [][]string{
				{}, // Empty result
			},
			pullErrors:       []error{nil},
			expectedLogParts: []string{},
			expectLogFile:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "consumer_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			logFile := filepath.Join(tempDir, "test.log")

			cfg := &config.ConsumerConfig{
				Username:      "testuser",
				LogFile:       logFile,
				StreamKey:     "radius:updates:testuser",
				ConsumerGroup: "test-group",
				ConsumerName:  "test-consumer",
			}

			mockStreamClient := &mockStream{
				pullResults: tt.pullResults,
				pullErrors:  tt.pullErrors,
			}

			consumer := New(cfg, mockStreamClient)

			done := make(chan error, 1)
			go func() {
				done <- consumer.Start()
			}()

			// Let it process messages
			time.Sleep(300 * time.Millisecond)

			consumer.Stop()

			// Wait for consumer to finish
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("Consumer.Start() returned error: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Error("Consumer did not stop within timeout")
			}

			if tt.expectLogFile {
				if _, err := os.Stat(logFile); os.IsNotExist(err) {
					t.Error("Expected log file to be created, but it doesn't exist")
					return
				}

				content, err := os.ReadFile(logFile)
				if err != nil {
					t.Fatalf("Failed to read log file: %v", err)
				}

				logContent := string(content)

				for _, expectedPart := range tt.expectedLogParts {
					// avoiding exact timestamp matching
					if !strings.Contains(logContent, expectedPart) {
						t.Errorf("Expected log content to contain: %s\nGot: %s", expectedPart, logContent)
					}
				}

				lines := strings.Split(strings.TrimSpace(logContent), "\n")
				if len(tt.expectedLogParts) > 0 && len(lines) != len(tt.expectedLogParts) {
					t.Errorf("Expected %d log lines, got %d", len(tt.expectedLogParts), len(lines))
				}
			} else {
				if _, err := os.Stat(logFile); !os.IsNotExist(err) {
					t.Error("Expected log file to not exist, but it does")
				}
			}
		})
	}
}
