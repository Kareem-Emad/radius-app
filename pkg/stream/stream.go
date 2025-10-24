package stream

// StreamMessage represents a message to be sent to a stream
type StreamMessage struct {
	Key      string
	Username string
	Data     map[string]interface{}
}

// ConsumerConfig holds configuration for stream consumers
type ConsumerConfig struct {
	StreamKey     string
	ConsumerGroup string
	ConsumerName  string
}

// Stream interface defines methods for publishing and consuming messages from streams
type Stream interface {
	Push(streamKey string, message StreamMessage) error
	Pull(config ConsumerConfig) ([]string, error)
}
