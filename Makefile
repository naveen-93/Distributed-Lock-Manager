# Makefile for Distributed Lock Manager
# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get

# Binary names
SERVER_BIN=bin/server
CLIENT_BIN=bin/client

# Source files
SERVER_SRC=cmd/server/main.go
CLIENT_SRC=cmd/client/main.go

# Directories
BIN_DIR=bin
DATA_DIR=data

# Default target
all: clean setup build

# Setup directories
setup:
	@mkdir -p $(BIN_DIR)
	@mkdir -p $(DATA_DIR)

# Build the server and client
build: build-server build-client

build-server:
	$(GOBUILD) -o $(SERVER_BIN) $(SERVER_SRC)
	@echo "Server built successfully"

build-client:
	$(GOBUILD) -o $(CLIENT_BIN) $(CLIENT_SRC)
	@echo "Client built successfully"

# Run the server
run-server:
	@$(SERVER_BIN)

# Run the client
run-client:
	@$(CLIENT_BIN)

# Run multiple clients concurrently
run-multi-clients:
	@echo "Running 5 clients concurrently..."
	@for i in 1 2 3 4 5; do \
		$(GORUN) cmd/client/main.go $$i & \
	done
	@echo "All clients launched"

# Test correctness with multiple clients writing to the same file
test-correctness:
	@echo "Testing correctness with multiple clients..."
	@# Create file if it doesn't exist, but don't remove existing content
	@mkdir -p $(DATA_DIR)
	@touch $(DATA_DIR)/file_0
	@echo "Starting correctness test (appending to existing file)..."
	@for i in 1 2 3 4 5; do \
		$(GORUN) cmd/client/main.go $$i "Client $$i writing" & \
	done
	@echo "Waiting for clients to complete..."
	@sleep 5
	@echo "Contents of file_0:"
	@cat $(DATA_DIR)/file_0

# Alternative test that starts with a clean file
test-correctness-clean:
	@echo "Testing correctness with multiple clients (clean start)..."
	@rm -f $(DATA_DIR)/file_0
	@touch $(DATA_DIR)/file_0
	@for i in 1 2 3 4 5; do \
		$(GORUN) cmd/client/main.go $$i "Client $$i writing" & \
	done
	@echo "Waiting for clients to complete..."
	@sleep 5
	@echo "Contents of file_0:"
	@cat $(DATA_DIR)/file_0

# Clean up
clean-bin:
	@rm -rf $(BIN_DIR)
	@echo "Cleaned up binaries"

# Clean data files
clean-data:
	@rm -rf $(DATA_DIR)/*
	@echo "Cleaned up data files"

# Clean everything
clean: clean-bin clean-data

# Install dependencies
deps:
	$(GOGET) google.golang.org/grpc
	$(GOGET) google.golang.org/protobuf/cmd/protoc-gen-go
	$(GOGET) google.golang.org/grpc/cmd/protoc-gen-go-grpc

# Generate protobuf code
proto:
	protoc --go_out=. --go-grpc_out=. proto/lock.proto

# Help
help:
	@echo "Available commands:"
	@echo " make all - Clean, setup directories, and build binaries"
	@echo " make build - Build server and client binaries"
	@echo " make run-server - Run the server from binary"
	@echo " make run-client - Run the client from binary"
	@echo " make run-multi-clients - Run multiple clients concurrently"
	@echo " make test-correctness - Test lock correctness with multiple clients (append to existing file)"
	@echo " make test-correctness-clean - Test lock correctness with multiple clients (clean start)"
	@echo " make clean-bin - Remove binaries"
	@echo " make clean-data - Remove data files"
	@echo " make clean - Remove binaries and data files"
	@echo " make deps - Install dependencies"
	@echo " make proto - Generate protobuf code"

.PHONY: all setup build build-server build-client run-server run-client run-multi-clients test-correctness test-correctness-clean clean-bin clean-data clean deps proto help