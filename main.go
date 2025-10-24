package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"dni/config"
	"dni/datastore"
	"dni/stream"

	"github.com/go-redis/redis/v8"
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2866"
)

var redisClient *redis.Client
var ctx = context.Background()
var datastoreClient datastore.Datastore
var streamClient stream.Stream
var cfg *config.Config

func initRedis(config *config.Config) error {
	redisAddr := fmt.Sprintf("%s:%d", config.RedisHost, config.RedisPort)
	redisClient = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})

	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %v", err)
	}

	// Initialize the interface implementations
	datastoreClient = datastore.NewRedisStore(redisClient)
	streamClient = stream.NewRedisStream(redisClient)

	log.Printf("Connected to Redis at %s", redisAddr)
	return nil
}

func storeAccountingData(record datastore.AccountingRecord) error {
	key := fmt.Sprintf("radius:acct:%s:%s", record.Username, record.AcctSessionID)

	log.Printf("[DATASTORE] Storing accounting data with key: %s", key)

	err := datastoreClient.Save(key, record, cfg.AccountingTTL)
	if err != nil {
		return fmt.Errorf("failed to store accounting data: %v", err)
	}

	log.Printf("[DATASTORE] Successfully stored accounting data for key: %s", key)
	return nil
}

func publishStreamNotification(username, key string) error {
	streamKey := fmt.Sprintf("radius:updates:%s", username)

	message := stream.StreamMessage{
		Key:      key,
		Username: username,
		Data: map[string]interface{}{
			"timestamp": time.Now().Unix(),
		},
	}

	log.Printf("[REDIS] Publishing to stream: %s", streamKey)

	err := streamClient.Push(streamKey, message)
	if err != nil {
		return fmt.Errorf("failed to publish to stream %s: %v", streamKey, err)
	}

	log.Printf("[REDIS] Successfully published notification to stream: %s", streamKey)
	return nil
}

func main() {
	// Load configuration from environment variables
	var err error
	cfg, err = config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize Redis connection
	if err := initRedis(cfg); err != nil {
		log.Fatalf("Failed to initialize Redis: %v", err)
	}
	defer redisClient.Close()

	// Get secret from configuration
	secret := []byte(cfg.Secret)

	// Authentication handler for port 1812
	authHandler := func(w radius.ResponseWriter, r *radius.Request) {
		username := rfc2865.UserName_GetString(r.Packet)
		password := rfc2865.UserPassword_GetString(r.Packet)

		log.Printf("[AUTH] Received Access-Request from %v for user: %s", r.RemoteAddr, username)

		var code radius.Code
		if username == "tim" && password == "12345" {
			code = radius.CodeAccessAccept
			log.Printf("[AUTH] Access granted for user: %s", username)
		} else {
			code = radius.CodeAccessReject
			log.Printf("[AUTH] Access denied for user: %s", username)
		}

		w.Write(r.Response(code))
	}

	acctHandler := func(w radius.ResponseWriter, r *radius.Request) {
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

		if err := storeAccountingData(record); err != nil {
			log.Printf("[REDIS] Error storing accounting data: %v", err)
		} else {
			recordKey := fmt.Sprintf("radius:acct:%s:%s", username, acctSessionId)

			if err := publishStreamNotification(username, recordKey); err != nil {
				log.Printf("[REDIS] Error publishing stream notification: %v", err)
			}
		}

		response := r.Response(radius.CodeAccountingResponse)
		w.Write(response)
		log.Printf("[ACCT] Sent Accounting-Response to %v", r.RemoteAddr)
	}

	authServer := &radius.PacketServer{
		Addr:         cfg.AuthPort,
		Handler:      radius.HandlerFunc(authHandler),
		SecretSource: radius.StaticSecretSource(secret),
	}

	acctServer := &radius.PacketServer{
		Addr:         cfg.AcctPort,
		Handler:      radius.HandlerFunc(acctHandler),
		SecretSource: radius.StaticSecretSource(secret),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Start authentication server
	go func() {
		defer wg.Done()
		log.Printf("Starting Authentication server on %s", cfg.AuthPort)
		if err := authServer.ListenAndServe(); err != nil {
			log.Fatalf("Authentication server error: %v", err)
		}
	}()

	// Start accounting server
	go func() {
		defer wg.Done()
		log.Printf("Starting Accounting server on %s", cfg.AcctPort)
		if err := acctServer.ListenAndServe(); err != nil {
			log.Fatalf("Accounting server error: %v", err)
		}
	}()

	wg.Wait()
}
