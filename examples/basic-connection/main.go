package main

import (
	"fmt"

	"github.com/mrlm-net/simconnect/pkg/client"
)

func main() {
	cli := client.New("TEST Clinet")

	// Connect to the SimConnect service
	if err := cli.Connect(); err != nil {
		panic(err)
	}

	for event := range cli.Listen() {
		// Process the event
		fmt.Println("Received event:", event)
	}
}
