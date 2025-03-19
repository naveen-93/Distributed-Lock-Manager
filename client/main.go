package main

import (
    "context"
    "fmt"
    "log"

    "google.golang.org/grpc"
    pb "Distributed-Lock-Manager/proto" // Replace with the actual package path
)

func main() {
    // Establish a connection to the server at localhost:50051
    conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
    if err != nil {
        log.Fatalf("Failed to connect to server: %v", err)
    }
    defer conn.Close()

    // Create a gRPC client instance
    client := pb.NewLockServiceClient(conn)

    // Define a unique client ID (hardcoded as 1 for this example)
    clientID := int32(1)

    // Step 1: Initialize the client
    _, err = client.ClientInit(context.Background(), &pb.Int{Rc: 0})
    if err != nil {
        log.Fatalf("ClientInit failed: %v", err)
    }
    fmt.Println("Client initialized successfully")

    // Step 2: Acquire the lock
    lockArgs := &pb.LockArgs{ClientId: clientID}
    resp, err := client.LockAcquire(context.Background(), lockArgs)
    if err != nil {
        log.Fatalf("LockAcquire failed: %v", err)
    }
    if resp.Status == pb.Status_SUCCESS {
        fmt.Println("Lock acquired successfully")
    } else {
        log.Fatalf("LockAcquire failed with status: %v", resp.Status)
    }

    // Step 3: Append data to a file
    fileArgs := &pb.FileArgs{
        Filename: "file_0",
        Content:  []byte("Hello, World!"),
        ClientId: clientID,
    }
    resp, err = client.FileAppend(context.Background(), fileArgs)
    if err != nil {
        log.Fatalf("FileAppend failed: %v", err)
    }
    if resp.Status == pb.Status_SUCCESS {
        fmt.Println("File appended successfully")
    } else {
        log.Fatalf("FileAppend failed with status: %v", resp.Status)
    }

    // Step 4: Release the lock
    resp, err = client.LockRelease(context.Background(), lockArgs)
    if err != nil {
        log.Fatalf("LockRelease failed: %v", err)
    }
    if resp.Status == pb.Status_SUCCESS {
        fmt.Println("Lock released successfully")
    } else {
        log.Fatalf("LockRelease failed with status: %v", resp.Status)
    }

    // Step 5: Close the client
    _, err = client.ClientClose(context.Background(), &pb.Int{Rc: 0})
    if err != nil {
        log.Fatalf("ClientClose failed: %v", err)
    }
    fmt.Println("Client closed successfully")
}