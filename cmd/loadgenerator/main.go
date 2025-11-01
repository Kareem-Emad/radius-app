package main

import (
	mrand "math/rand"
	"os/exec"

	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {

	rps := flag.Int("rps", 10, "Requests per second")
	numberOfRequests := flag.Int("n", 100, "Total number of requests to send")
	username := flag.String("username", "username123", "Username for the requests")

	flag.Parse()

	outputFile := "/test/requests.txt"
	f, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("failed to create output file: %v", err)
	}
	defer f.Close()

	randSource := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	for i := range *numberOfRequests {

		// Randomized fields
		status := randStatus()
		sessionID := fmt.Sprintf("sess%d", 10000+randSource.Intn(90000))
		nasIP := fmt.Sprintf("192.168.%d.%d", randSource.Intn(256), randSource.Intn(256))
		nasPort := 1 + randSource.Intn(65535)

		// Fixed fields
		serviceType := "Framed-User"
		framedProtocol := "PPP"

		fmt.Fprintf(f, "Acct-Status-Type = %s\n", status)
		fmt.Fprintf(f, "User-Name = %q\n", *username)
		fmt.Fprintf(f, "Acct-Session-Id = %q\n", sessionID)
		fmt.Fprintf(f, "NAS-IP-Address = %s\n", nasIP)
		fmt.Fprintf(f, "NAS-Port = %d\n", nasPort)
		fmt.Fprintf(f, "Service-Type = %s\n", serviceType)
		fmt.Fprintf(f, "Framed-Protocol = %s\n", framedProtocol)

		if status == "Stop" {
			fmt.Fprintf(f, "Acct-Session-Time = %d\n", 3600)
		}

		if i != *numberOfRequests-1 {
			fmt.Fprintln(f) // new line between requests
		}
	}

	log.Printf("Wrote %d requests to %s\n", *numberOfRequests, outputFile)

	cmd := exec.Command(
		"radclient",
		"-n", fmt.Sprintf("%d", *rps),
		"-s",
		"-f", outputFile,
		"radius-server:1813",
		"acct",
		"testing123",
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalf("radclient command failed: %v", err)
	}

}

func randStatus() string {
	if mrand.Intn(2) == 0 {
		return "Start"
	}
	return "Stop"
}
