package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2866"
)

var redisClient *redis.Client
var ctx = context.Background()

func initRedis() error {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}

	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", redisHost, redisPort),
		Password: "",
		DB:       0, // default DB
	})

	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %v", err)
	}

	log.Printf("Connected to Redis at %s:%s", redisHost, redisPort)
	return nil
}

func storeAccountingData(username, nasIPAddress, acctSessionId string, data map[string]interface{}) error {
	key := fmt.Sprintf("radius:acct:%s:%s", username, acctSessionId)

	log.Printf("[REDIS] Storing accounting data with key: %s", key)

	err := redisClient.HMSet(ctx, key, data).Err()
	if err != nil {
		return fmt.Errorf("failed to store accounting data: %v", err)
	}

	err = redisClient.Expire(ctx, key, 10*time.Minute).Err()
	if err != nil {
		log.Printf("[REDIS] Warning: failed to set TTL for key %s: %v", key, err)
	}

	log.Printf("[REDIS] Successfully stored accounting data for key: %s", key)
	return nil
}

func publishStreamNotification(username, key string) error {
	streamKey := fmt.Sprintf("radius:updates:%s", username)

	values := map[string]interface{}{
		"key":       key,
		"timestamp": time.Now().Unix(),
		"username":  username,
	}

	log.Printf("[REDIS] Publishing to stream: %s", streamKey)

	_, err := redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: values,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to publish to stream %s: %v", streamKey, err)
	}

	log.Printf("[REDIS] Successfully published notification to stream: %s", streamKey)
	return nil
}

func main() {
	// Initialize Redis connection
	if err := initRedis(); err != nil {
		log.Fatalf("Failed to initialize Redis: %v", err)
	}
	defer redisClient.Close()

	// Shared secret as specified in requirements
	secret := []byte("testing123")

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

		// Prepare data for Redis storage (convert all RADIUS types to basic types)
		accountingData := map[string]interface{}{
			"username":           username,
			"nas_ip_address":     nasIPAddress.String(),
			"nas_port":           fmt.Sprintf("%d", nasPort),
			"acct_status_type":   fmt.Sprintf("%d", acctStatusType),
			"acct_session_id":    acctSessionId,
			"framed_ip_address":  framedIPAddress.String(),
			"calling_station_id": callingStationId,
			"called_station_id":  calledStationId,
			"packet_type":        "Accounting-Request",
			"timestamp":          fmt.Sprintf("%d", time.Now().Unix()),
		}

		if acctStatusType == rfc2866.AcctStatusType_Value_Stop {
			acctInputOctets := rfc2866.AcctInputOctets_Get(r.Packet)
			acctOutputOctets := rfc2866.AcctOutputOctets_Get(r.Packet)
			acctSessionTime := rfc2866.AcctSessionTime_Get(r.Packet)

			log.Printf("[ACCT] Acct-Input-Octets: %d", acctInputOctets)
			log.Printf("[ACCT] Acct-Output-Octets: %d", acctOutputOctets)
			log.Printf("[ACCT] Acct-Session-Time: %d seconds", acctSessionTime)

			accountingData["acct_input_octets"] = fmt.Sprintf("%d", acctInputOctets)
			accountingData["acct_output_octets"] = fmt.Sprintf("%d", acctOutputOctets)
			accountingData["acct_session_time"] = fmt.Sprintf("%d", acctSessionTime)
		}

		if err := storeAccountingData(username, nasIPAddress.String(), acctSessionId, accountingData); err != nil {
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
		Addr:         ":1812",
		Handler:      radius.HandlerFunc(authHandler),
		SecretSource: radius.StaticSecretSource(secret),
	}

	acctServer := &radius.PacketServer{
		Addr:         ":1813",
		Handler:      radius.HandlerFunc(acctHandler),
		SecretSource: radius.StaticSecretSource(secret),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Start authentication server (port 1812)
	go func() {
		defer wg.Done()
		log.Printf("Starting Authentication server on :1812")
		if err := authServer.ListenAndServe(); err != nil {
			log.Fatalf("Authentication server error: %v", err)
		}
	}()

	// Start accounting server (port 1813)
	go func() {
		defer wg.Done()
		log.Printf("Starting Accounting server on :1813")
		if err := acctServer.ListenAndServe(); err != nil {
			log.Fatalf("Accounting server error: %v", err)
		}
	}()

	wg.Wait()
}
