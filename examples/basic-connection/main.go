//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/mrlm-net/simconnect"
)

func main() {
	// This is a placeholder main function.
	// The actual implementation would go here.
	client := simconnect.New("GO Example - SimConnect Basic Connection")

	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer func() {
		if err := client.Disconnect(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}

		fmt.Println("Disconnected")

	}()

	// Application logic would go here.
	fmt.Println("Connected to SimConnect")
	fmt.Println("Sleeping for 2 seconds...")
	time.Sleep(2 * time.Second)

}
