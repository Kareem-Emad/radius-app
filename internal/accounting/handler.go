package accounting

import (
	"fmt"
	"log"
	"time"

	"dni/pkg/datastore"
	"dni/pkg/stream"

	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2866"
)

// Handler handles RADIUS accounting requests
type Handler struct {
	DataStore     datastore.Datastore
	Stream        stream.Stream
	AccountingTTL time.Duration
}

// NewHandler creates a new accounting handler
func NewHandler(dataStore datastore.Datastore, streamClient stream.Stream, accountingTTL time.Duration) *Handler {
	return &Handler{
		DataStore:     dataStore,
		Stream:        streamClient,
		AccountingTTL: accountingTTL,
	}
}

// Handle processes accounting requests
func (h *Handler) Handle(w radius.ResponseWriter, r *radius.Request) {
	username := rfc2865.UserName_GetString(r.Packet)
	nasIPAddress := rfc2865.NASIPAddress_Get(r.Packet)
	nasPort := rfc2865.NASPort_Get(r.Packet)
	acctStatusType := rfc2866.AcctStatusType_Get(r.Packet)
	acctSessionId := rfc2866.AcctSessionID_GetString(r.Packet)
	framedIPAddress := rfc2865.FramedIPAddress_Get(r.Packet)
	callingStationId := rfc2865.CallingStationID_GetString(r.Packet)
	calledStationId := rfc2865.CalledStationID_GetString(r.Packet)

	log.Printf("[ACCT] Received Accounting-Request from %v", r.RemoteAddr)
	log.Printf("[ACCT] Username: %s", username)
	log.Printf("[ACCT] NAS-IP-Address: %v", nasIPAddress)
	log.Printf("[ACCT] NAS-Port: %d", nasPort)
	log.Printf("[ACCT] Acct-Status-Type: %v", acctStatusType)
	log.Printf("[ACCT] Acct-Session-Id: %s", acctSessionId)
	log.Printf("[ACCT] Framed-IP-Address: %v", framedIPAddress)
	log.Printf("[ACCT] Calling-Station-Id: %s", callingStationId)
	log.Printf("[ACCT] Called-Station-Id: %s", calledStationId)

	// Create AccountingRecord struct
	record := datastore.AccountingRecord{
		Username:         username,
		NASIPAddress:     nasIPAddress.String(),
		NASPort:          fmt.Sprintf("%d", nasPort),
		AcctStatusType:   fmt.Sprintf("%d", acctStatusType),
		AcctSessionID:    acctSessionId,
		FramedIPAddress:  framedIPAddress.String(),
		CallingStationID: callingStationId,
		CalledStationID:  calledStationId,
		PacketType:       "Accounting-Request",
		Timestamp:        fmt.Sprintf("%d", time.Now().Unix()),
	}

	if acctStatusType == rfc2866.AcctStatusType_Value_Stop {
		acctInputOctets := rfc2866.AcctInputOctets_Get(r.Packet)
		acctOutputOctets := rfc2866.AcctOutputOctets_Get(r.Packet)
		acctSessionTime := rfc2866.AcctSessionTime_Get(r.Packet)

		log.Printf("[ACCT] Acct-Input-Octets: %d", acctInputOctets)
		log.Printf("[ACCT] Acct-Output-Octets: %d", acctOutputOctets)
		log.Printf("[ACCT] Acct-Session-Time: %d seconds", acctSessionTime)

		record.AcctInputOctets = fmt.Sprintf("%d", acctInputOctets)
		record.AcctOutputOctets = fmt.Sprintf("%d", acctOutputOctets)
		record.AcctSessionTime = fmt.Sprintf("%d", acctSessionTime)
	}

	if err := h.storeAccountingData(record); err != nil {
		log.Printf("[REDIS] Error storing accounting data: %v", err)
	} else {
		recordKey := fmt.Sprintf("radius:acct:%s:%s", username, acctSessionId)

		if err := h.publishStreamNotification(username, recordKey); err != nil {
			log.Printf("[REDIS] Error publishing stream notification: %v", err)
		}
	}

	response := r.Response(radius.CodeAccountingResponse)
	w.Write(response)
	log.Printf("[ACCT] Sent Accounting-Response to %v", r.RemoteAddr)
}

func (h *Handler) storeAccountingData(record datastore.AccountingRecord) error {
	key := fmt.Sprintf("radius:acct:%s:%s", record.Username, record.AcctSessionID)

	log.Printf("[DATASTORE] Storing accounting data with key: %s", key)

	err := h.DataStore.Save(key, record, h.AccountingTTL)
	if err != nil {
		return fmt.Errorf("failed to store accounting data: %v", err)
	}

	log.Printf("[DATASTORE] Successfully stored accounting data for key: %s", key)
	return nil
}

func (h *Handler) publishStreamNotification(username, key string) error {
	streamKey := fmt.Sprintf("radius:updates:%s", username)

	message := stream.StreamMessage{
		Key:      key,
		Username: username,
		Data: map[string]interface{}{
			"timestamp": time.Now().Unix(),
		},
	}

	log.Printf("[REDIS] Publishing to stream: %s", streamKey)

	err := h.Stream.Push(streamKey, message)
	if err != nil {
		return fmt.Errorf("failed to publish to stream %s: %v", streamKey, err)
	}

	log.Printf("[REDIS] Successfully published notification to stream: %s", streamKey)
	return nil
}
