package main

import (
	"log"
	"net"

	"Distributed-Lock-Manager/internal/server"
	pb "Distributed-Lock-Manager/proto"

	"google.golang.org/grpc"
)

func main() {
	// Initialize the files
	server.CreateFiles()

	// Set up TCP listener
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Create gRPC server
	s := grpc.NewServer()
	pb.RegisterLockServiceServer(s, server.NewLockServer())

	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
