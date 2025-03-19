package main

import (
    "context"
    "fmt"
    "log"
    "net"
    "os"
    "strconv"
    "strings"
    "sync"

    "google.golang.org/grpc"
    pb "Distributed-Lock-Manager/proto" // Replace with the actual package path
)

// lockServer implements the LockServiceServer interface
type lockServer struct {
    pb.UnimplementedLockServiceServer
    mu         sync.Mutex   // Protects shared state
    cond       *sync.Cond   // Condition variable for lock waiting
    lockHolder int32        // ID of the client holding the lock, -1 if free
}

// newLockServer initializes a new lock server
func newLockServer() *lockServer {
    s := &lockServer{
        lockHolder: -1, // No client holds the lock initially
    }
    s.cond = sync.NewCond(&s.mu)
    return s
}

// client_init handles the client initialization RPC
func (s *lockServer) ClientInit(ctx context.Context, args *pb.Int) (*pb.Int, error) {
    // Simple handshake: return 0 to acknowledge
    return &pb.Int{Rc: 0}, nil
}

// LockAcquire handles the lock acquisition RPC
func (s *lockServer) LockAcquire(ctx context.Context, args *pb.LockArgs) (*pb.Response, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Wait until the lock is free (lockHolder == -1)
    for s.lockHolder != -1 {
        s.cond.Wait() // Unlocks mu, waits, then relocks mu when woken
    }

    // Assign the lock to this client
    s.lockHolder = args.ClientId
    return &pb.Response{Status: pb.Status_SUCCESS}, nil
}

// LockRelease handles the lock release RPC
func (s *lockServer) LockRelease(ctx context.Context, args *pb.LockArgs) (*pb.Response, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Check if this client holds the lock
    if s.lockHolder == args.ClientId {
        s.lockHolder = -1          // Free the lock
        s.cond.Broadcast()         // Wake all waiting clients
        return &pb.Response{Status: pb.Status_SUCCESS}, nil
    }
    // Client doesn't hold the lock
    return &pb.Response{Status: pb.Status_FILE_ERROR}, nil
}

// FileAppend handles the file append RPC
func (s *lockServer) FileAppend(ctx context.Context, args *pb.FileArgs) (*pb.Response, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Check if this client holds the lock
    if s.lockHolder != args.ClientId {
        return &pb.Response{Status: pb.Status_FILE_ERROR}, nil
    }

    // Validate filename (must be "file_0" to "file_99")
    if !strings.HasPrefix(args.Filename, "file_") {
        return &pb.Response{Status: pb.Status_FILE_ERROR}, nil
    }
    numStr := strings.TrimPrefix(args.Filename, "file_")
    num, err := strconv.Atoi(numStr)
    if err != nil || num < 0 || num >= 100 {
        return &pb.Response{Status: pb.Status_FILE_ERROR}, nil
    }

    // Append content to the file
    f, err := os.OpenFile(args.Filename, os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        return &pb.Response{Status: pb.Status_FILE_ERROR}, nil
    }
    defer f.Close()

    _, err = f.Write(args.Content)
    if err != nil {
        return &pb.Response{Status: pb.Status_FILE_ERROR}, nil
    }

    return &pb.Response{Status: pb.Status_SUCCESS}, nil
}

// ClientClose handles the client close RPC
func (s *lockServer) ClientClose(ctx context.Context, args *pb.Int) (*pb.Int, error) {
    // Simple acknowledgment: return 0
    return &pb.Int{Rc: 0}, nil
}

// createFiles ensures the 100 files exist
func createFiles() {
    for i := 0; i < 100; i++ {
        filename := fmt.Sprintf("file_%d", i)
        // Create file only if it doesn't exist
        if _, err := os.Stat(filename); os.IsNotExist(err) {
            f, err := os.Create(filename)
            if err != nil {
                log.Fatalf("Failed to create file %s: %v", filename, err)
            }
            f.Close()
        }
    }
}

func main() {
    // Initialize the files
    createFiles()

    // Set up TCP listener
    lis, err := net.Listen("tcp", ":50051")
    if err != nil {
        log.Fatalf("Failed to listen: %v", err)
    }

    // Create gRPC server
    s := grpc.NewServer()
    pb.RegisterLockServiceServer(s, newLockServer())

    log.Printf("Server listening at %v", lis.Addr())
    if err := s.Serve(lis); err != nil {
        log.Fatalf("Failed to serve: %v", err)
    }
}