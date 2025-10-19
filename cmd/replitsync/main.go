package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.Printf("Starting sync service...")

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create ticker for periodic sync
	interval := time.Duration(60) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Service started. Syncing every %v", interval)

	// Initial sync
	if err := syncEvents(); err != nil {
		log.Printf("Initial sync error: %v", err)
	}

	// Main loop
	for {
		select {
		case <-ticker.C:
			if err := syncEvents(); err != nil {
				log.Printf("Sync error: %v", err)
			}
		case sig := <-sigChan:
			log.Printf("Received signal %v, shutting down...", sig)
			return
		}
	}
}