package datastore

import "time"

// AccountingRecord represents accounting data to be stored
type AccountingRecord struct {
	Username         string
	NASIPAddress     string
	NASPort          string
	AcctStatusType   string
	AcctSessionID    string
	FramedIPAddress  string
	CallingStationID string
	CalledStationID  string
	PacketType       string
	Timestamp        string
	// Optional session metrics for STOP records
	AcctInputOctets  string
	AcctOutputOctets string
	AcctSessionTime  string
}

// Datastore interface defines methods for storing accounting records
type Datastore interface {
	Save(key string, record AccountingRecord, ttl time.Duration) error
}
