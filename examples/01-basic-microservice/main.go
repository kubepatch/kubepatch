package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Create a ServeMux and register handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]string{
			"status":    "ok",
			"version":   os.Getenv("RESTAPIAPP_VERSION"),
			"log_level": os.Getenv("LOG_LEVEL"),
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	// Create the HTTP server
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Start the server in a goroutine
	go func() {
		log.Println("Starting HTTP server on :8080")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server ListenAndServe: %v", err)
		}
	}()

	// Handle shutdown signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	<-quit
	log.Println("Shutdown signal received")

	// Shutdown with timeout
	ctxTimeout := 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("HTTP server Shutdown: %v", err)
	}

	log.Println("Server exited properly")
}
