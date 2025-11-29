//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"

	"github.com/mrlm-net/simconnect"
)

func main() {
	// This is a placeholder main function.
	// The actual implementation would go here.
	client := simconnect.New("GO Example - SimConnect Read Messages")

	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer func() {
		if err := client.Disconnect(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		fmt.Println("Disconnected from SimConnect...")
	}()

	// Application logic would go here.
	fmt.Println("Connected to SimConnect...")

	client.SubscribeToSystemEvent(1001, "6Hz")

	for msg := range client.Stream() {
		fmt.Println(msg.SIMCONNECT_RECV)
	}

}
