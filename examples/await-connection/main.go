//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func main() {
	// Create cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handler for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, shutting down...")
		cancel()
	}()

	// Initialize client with context
	client := simconnect.New("GO Example - SimConnect Await Connection",
		engine.WithContext(ctx),
	)

	// Retry connection until simulator is running
	fmt.Println("Waiting for simulator to start...")
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Cancelled while waiting for simulator")
			return
		default:
			if err := client.Connect(); err != nil {
				fmt.Printf("Connection attempt failed: %v, retrying in 2 seconds...\n", err)
				time.Sleep(2 * time.Second)
				continue
			}
			fmt.Println("Connected to SimConnect!")
			goto connected
		}
	}

connected:
	defer func() {
		if err := client.Disconnect(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}

		fmt.Println("Disconnected from SimConnect...")

	}()

	fmt.Println("Connected to SimConnect...")
	fmt.Println("Waiting for connection ready...")

	// Wait for SIMCONNECT_RECV_ID_OPEN message to confirm connection is ready
	stream := client.Stream()
	connectionReady := false

	for msg := range stream {
		if msg.Err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", msg.Err)
			continue
		}

		if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_OPEN {
			connectionReady = true
			fmt.Println("Connection is ready!")
			break
		}
	}

	if !connectionReady {
		fmt.Println("Failed to establish connection")
		return
	}

	// Continue streaming and display all messages
	fmt.Println("Listening for messages... (Press Ctrl+C to exit)")
	for msg := range stream {
		if msg.Err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", msg.Err)
			continue
		}

		fmt.Printf("Message received - ID: %d, Size: %d bytes\n", msg.DwID, msg.Size)
	}

	fmt.Println("Stream closed, exiting...")
}
