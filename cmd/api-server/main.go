/*
 * API Handler Server (Port 9009)
 * Following Nutanix gRPC Gateway pattern
 *
 * This server handles:
 * - Synchronous Cat CRUD operations
 * - Async operation initiation (returns task IDs)
 * - Communication with Task Server
 */

package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/nutanix/ntnx-api-golang-mock/global"
	"github.com/nutanix/ntnx-api-golang-mock/interfaces/apis/mock/v4/config"
	"github.com/nutanix/ntnx-api-golang-mock/routes"
	"github.com/nutanix/ntnx-api-golang-mock/services"
	log "github.com/sirupsen/logrus"
)

func main() {
	initLogger()

	log.Info("🚀 Starting API Handler Server (gRPC Gateway Pattern)")
	log.Info("================================================")
	log.Infof("Port: %s", global.APIServerPort)
	log.Infof("Task Server: localhost:%s", global.TaskServerPort)
	log.Info("================================================")
	log.Info("")
	log.Info("🎯 Using AUTO-GENERATED DTOs with auto-set $objectType!")
	log.Info("")

	// Create service implementation with AUTO-GENERATED DTOs
	catServiceDTO := services.NewCatServiceWithDTO()

	// Wrap implementation
	wrapper := config.NewCat_endpointsImplWrapper(catServiceDTO)

	// Setup router
	router := routes.SetupAPIServerRouter(wrapper)

	// Start server
	address := fmt.Sprintf(":%s", global.APIServerPort)
	log.Infof("✅ API Handler Server listening on %s", address)
	log.Info("")
	log.Info("📝 Available Endpoints:")
	log.Info("  Synchronous Operations:")
	log.Info("    GET    /mock/v4/config/cats          - List cats")
	log.Info("    GET    /mock/v4/config/cat/{catId}   - Get cat by ID")
	log.Info("    POST   /mock/v4/config/cats          - Create cat")
	log.Info("    PUT    /mock/v4/config/cat/{catId}   - Update cat")
	log.Info("    DELETE /mock/v4/config/cat/{catId}   - Delete cat")
	log.Info("  Asynchronous Operations:")
	log.Info("    GET    /mock/v4/config/cat/{catId}/async - Get cat async (returns task ID)")
	log.Info("  Health:")
	log.Info("    GET    /health                       - Health check")
	log.Info("")
	log.Info("Press CTRL+C to stop the server")
	log.Info("")

	if err := http.ListenAndServe(address, router); err != nil {
		log.Fatalf("❌ Failed to start API Handler Server: %v", err)
	}
}

func initLogger() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		ForceColors:     true,
	})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
}
