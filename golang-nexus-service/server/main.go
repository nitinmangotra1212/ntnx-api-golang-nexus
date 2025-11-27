/*
 * Copyright (c) 2025 Nutanix Inc. All rights reserved.
 */

package main

import (
	"flag"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/grpc"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/utils/logging"
)

var (
	port     = flag.Int("port", 9090, "The server port")
	logLevel = flag.String("log-level", "info", "Log level: debug, info, warn, error (default: info)")
)

var (
	waitGroup sync.WaitGroup
)

func main() {
	// Parse command line flags
	flag.Parse()

	// Initialize logger with hot-reloading capability
	logging.InitLogger(*logLevel)

	log.Info("Starting Golang Nexus Service...")

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
