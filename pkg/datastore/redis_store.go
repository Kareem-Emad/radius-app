package datastore

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisStore implements the Datastore interface using Redis hashes
type RedisStore struct {
	client *redis.Client
	ctx    context.Context
}

// NewRedisStore creates a new RedisStore instance
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{
		client: client,
		ctx:    context.Background(),
	}
}

// Save stores an accounting record as a Redis hash with TTL
func (rs *RedisStore) Save(key string, record AccountingRecord, ttl time.Duration) error {
	// Convert AccountingRecord to map for Redis hash storage
	data := map[string]interface{}{
		"username":           record.Username,
		"nas_ip_address":     record.NASIPAddress,
		"nas_port":           record.NASPort,
		"acct_status_type":   record.AcctStatusType,
		"acct_session_id":    record.AcctSessionID,
		"framed_ip_address":  record.FramedIPAddress,
		"calling_station_id": record.CallingStationID,
		"called_station_id":  record.CalledStationID,
		"packet_type":        record.PacketType,
		"timestamp":          record.Timestamp,
	}

	// Add session metrics if they exist (for STOP records)
	if record.AcctInputOctets != "" {
		data["acct_input_octets"] = record.AcctInputOctets
	}
	if record.AcctOutputOctets != "" {
		data["acct_output_octets"] = record.AcctOutputOctets
	}
	if record.AcctSessionTime != "" {
		data["acct_session_time"] = record.AcctSessionTime
	}

	// Store as hash object
	err := rs.client.HMSet(rs.ctx, key, data).Err()
	if err != nil {
		return fmt.Errorf("failed to store data in Redis: %v", err)
	}

	// Set TTL
	err = rs.client.Expire(rs.ctx, key, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set TTL for key %s: %v", key, err)
	}

	return nil
}
