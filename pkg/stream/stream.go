package stream

// StreamMessage represents a message to be sent to a stream
type StreamMessage struct {
	Key      string
	Username string
	Data     map[string]interface{}
}

// Stream interface defines methods for publishing messages to streams
type Stream interface {
	Push(streamKey string, message StreamMessage) error
}
