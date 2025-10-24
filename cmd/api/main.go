package main

import (
	"log"
	"sync"

	"dni/pkg/config"

	"layeh.com/radius"
)

func main() {
	// Load configuration from environment variables
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize all dependencies
	deps, err := InitializeDependencies(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize dependencies: %v", err)
	}
	defer deps.Close()

	// Get secret from configuration
	secret := []byte(cfg.Secret)

	// Create servers using the initialized handlers
	authServer := &radius.PacketServer{
		Addr:         cfg.AuthPort,
		Handler:      radius.HandlerFunc(deps.AuthHandler.Handle),
		SecretSource: radius.StaticSecretSource(secret),
	}

	acctServer := &radius.PacketServer{
		Addr:         cfg.AcctPort,
		Handler:      radius.HandlerFunc(deps.AcctHandler.Handle),
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
