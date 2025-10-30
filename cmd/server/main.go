// Package main is the entry point for the mock REST API server
package main

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	service "github.com/nutanix/ntnx-api-golang-mock/golang-mock-service"
	"github.com/nutanix/ntnx-api-golang-mock/golang-mock-service/handlers"
	"github.com/nutanix/ntnx-api-golang-mock/internal/config"
	"github.com/nutanix/ntnx-api-golang-mock/internal/middleware"
	log "github.com/sirupsen/logrus"
)

func main() {
	// Initialize logger
	initLogger()

	log.Info("Starting Mock REST API Server...")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Infof("Configuration loaded successfully")
	log.Infof("Server will listen on port: %d", cfg.Server.Port)

	// Initialize services
	catService := service.NewCatService()
	fileService := service.NewFileService()

	// Create SFTP configuration
	sftpConfig := &service.SFTPConfig{
		RemoteHost:     cfg.Mockrest.FileServer.RemoteHost,
		Username:       cfg.Mockrest.FileServer.Username,
		Password:       cfg.Mockrest.FileServer.Password,
		RemoteFilePath: cfg.Mockrest.FileServer.RemoteFilePath,
		DownloadDir:    cfg.Mockrest.FileServer.DownloadDir,
		UploadDir:      cfg.Mockrest.FileServer.UploadDir,
		UploadURL:      cfg.Mockrest.FileServer.UploadURL,
	}

	// Initialize handlers
	catHandler := handlers.NewCatHandler(catService, fileService, sftpConfig)

	// Setup router
	router := setupRouter(catHandler)

	// Start server
	address := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Infof("Mock REST API Server is running on %s", address)
	log.Info("Press CTRL+C to stop the server")

	if err := router.Run(address); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// setupRouter configures the Gin router with all routes and middleware
func setupRouter(catHandler *handlers.CatHandler) *gin.Engine {
	// Set Gin mode (release in production)
	gin.SetMode(gin.DebugMode)

	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(middleware.Logger())
	router.Use(middleware.CORS())

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "UP",
			"service": "ntnx-api-golang-mock",
		})
	})

	// API routes - base path
	api := router.Group("/mock/v4/config")
	{
		// Cat operations
		api.GET("/cats", catHandler.ListCats)
		api.POST("/cats", catHandler.CreateCat)

		// Task status (must come before :catId routes to avoid conflicts)
		api.GET("/cat/status", catHandler.GetCatStatusByUUID)

		// File operations (must come before :catId routes to avoid conflicts)
		api.GET("/cat/file/:fileId/download", catHandler.DownloadFile)
		api.POST("/cat/:catId/uploadFile", catHandler.UploadFile)

		// Cat by ID operations (wildcard routes should come last)
		api.GET("/cat/:catId", catHandler.GetCatByID)
		api.PUT("/cat/:catId", catHandler.UpdateCatByID)
		api.DELETE("/cat/:catId", catHandler.DeleteCatByID)

		// IP address operations
		api.POST("/cat/:catId/ipv4", catHandler.AddIPv4ToCat)
		api.POST("/cat/:catId/ipv6", catHandler.AddIPv6ToCat)
		api.POST("/cat/:catId/ipaddress", catHandler.AddIPAddressToCat)
	}

	return router
}

// initLogger initializes the logger with desired settings
func initLogger() {
	// Set log format
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		ForceColors:     true,
	})

	// Set log output
	log.SetOutput(os.Stdout)

	// Set log level
	log.SetLevel(log.InfoLevel)
}
