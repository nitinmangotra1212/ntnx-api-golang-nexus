/*
 * Task Server (Port 9010)
 * Following Nutanix gRPC Gateway pattern
 *
 * This server handles:
 * - Task storage and retrieval
 * - Task status updates
 * - Polling requests from clients
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

	log.Info("🚀 Starting Task Server (gRPC Gateway Pattern)")
	log.Info("================================================")
	log.Infof("Port: %s", global.TaskServerPort)
	log.Infof("API Server: localhost:%s", global.APIServerPort)
	log.Info("================================================")

	// Create service implementation with AUTO-GENERATED DTOs (same impl, different endpoints)
	catServiceDTO := services.NewCatServiceWithDTO()

	// Wrap implementation
	wrapper := config.NewCat_endpointsImplWrapper(catServiceDTO)

	// Setup router for Task Server
	router := routes.SetupTaskServerRouter(wrapper)

	// Start server
	address := fmt.Sprintf(":%s", global.TaskServerPort)
	log.Infof("✅ Task Server listening on %s", address)
	log.Info("")
	log.Info("📝 Available Endpoints:")
	log.Info("  Task Operations:")
	log.Info("    POST   /tasks         - Create task (from API Server)")
	log.Info("    PUT    /tasks/{taskId} - Update task (from API Server)")
	log.Info("    GET    /tasks/{taskId} - Poll task status (from Client)")
	log.Info("  Health:")
	log.Info("    GET    /health        - Health check")
	log.Info("")
	log.Info("💡 Tip: Clients poll this server to check async task progress")
	log.Info("")
	log.Info("Press CTRL+C to stop the server")
	log.Info("")

	if err := http.ListenAndServe(address, router); err != nil {
		log.Fatalf("❌ Failed to start Task Server: %v", err)
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
