/*
 * Copyright (c) 2025 Nutanix Inc. All rights reserved.
 */

package main

import (
	"flag"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/db"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/external"
	externalIdf "github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/external/idf"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/grpc"
	idfRepo "github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/idf"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/utils/logging"
)

var (
	port     = flag.Int("port", 9090, "The server port")
	logLevel = flag.String("log-level", "info", "Log level: debug, info, warn, error (default: info)")
	idfHost  = flag.String("idf-host", "127.0.0.1", "IDF service host")
	idfPort  = flag.Int("idf-port", 2027, "IDF service port")
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

	// Initialize IDF client via singleton (following az-manager pattern)
	log.Infof("Initializing IDF client: %s:%d", *idfHost, *idfPort)
	idfClient := externalIdf.NewIdfClient(*idfHost, uint16(*idfPort))
	external.SetSingletonServices(idfClient)
	log.Info("✅ IDF client initialized via singleton")

	// Create IDF repository (no client needed - uses singleton)
	itemRepository := idfRepo.NewItemRepository()
	log.Info("✅ IDF repository initialized")

	// Start gRPC server with IDF
	startGRPCServer(itemRepository)

	// Wait for all goroutines to complete
	waitGroup.Wait()
}

func startGRPCServer(itemRepository db.ItemRepository) {
	log.Info(fmt.Sprintf("Starting GRPC Server on port %v", *port))
	grpcServer := grpc.NewServer(uint64(*port))

	// Create ItemService with IDF repository
	itemService := grpc.NewItemGrpcService(itemRepository)

	// Register ItemService with gRPC server
	grpcServer.RegisterItemService(itemService)

	grpcServer.Start(&waitGroup)
}
