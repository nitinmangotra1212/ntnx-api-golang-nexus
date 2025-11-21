/*
 * Copyright (c) 2025 Nutanix Inc. All rights reserved.
 */

package main

import (
	"flag"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/nutanix/ntnx-api-golang-mock/golang-mock-service/grpc"
	"github.com/nutanix/ntnx-api-golang-mock/golang-mock-service/utils/logging"
)

var (
	port = flag.Int("port", 9090, "The server port")
)

var (
	waitGroup sync.WaitGroup
)

func main() {
	// Initialize logger with hot-reloading capability
	logging.InitLogger()

	log.Info("Starting Golang Mock Service...")

	// Start gRPC server
	startGRPCServer()

	// Wait for all goroutines to complete
	waitGroup.Wait()
}

func startGRPCServer() {
	log.Info(fmt.Sprintf("Starting GRPC Server on port %v", *port))
	grpcServer := grpc.NewServer(uint64(*port))
	grpcServer.Start(&waitGroup)
}
