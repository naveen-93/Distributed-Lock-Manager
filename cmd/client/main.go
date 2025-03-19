package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"Distributed-Lock-Manager/internal/client"
)

func main() {
	// Default values
	clientID := int32(1)
	message := "Hello, World!"

	// Parse command-line arguments if provided
	if len(os.Args) > 1 {
		id, err := strconv.Atoi(os.Args[1])
		if err == nil {
			clientID = int32(id)
		}
	}

	if len(os.Args) > 2 {
		message = os.Args[2]
	}

	// Create a new client with the specified ID
	c, err := client.NewLockClient("localhost:50051", clientID)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	// Step 1: Initialize the client
	if err := c.Initialize(); err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}
	fmt.Printf("Client %d initialized successfully\n", clientID)

	// Step 2: Acquire the lock
	if err := c.AcquireLock(); err != nil {
		log.Fatalf("Failed to acquire lock: %v", err)
	}
	fmt.Printf("Client %d acquired lock successfully\n", clientID)

	// Step 3: Append data to a file
	content := fmt.Sprintf("%s from client %d\n", message, clientID)
	if err := c.AppendFile("file_0", []byte(content)); err != nil {
		log.Fatalf("Failed to append to file: %v", err)
	}
	fmt.Printf("Client %d appended to file successfully\n", clientID)

	// Step 4: Release the lock
	if err := c.ReleaseLock(); err != nil {
		log.Fatalf("Failed to release lock: %v", err)
	}
	fmt.Printf("Client %d released lock successfully\n", clientID)

	fmt.Printf("Client %d closed successfully\n", clientID)
}
