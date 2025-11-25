/*
 * Copyright (c) 2025 Nutanix Inc. All rights reserved.
 */

package logging

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

// InitLogger initializes the logger with hot-reloading capability
// logLevel can be: "debug", "info", "warn", "error" (default: "info")
func InitLogger(logLevel string) {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		ForceColors:     true,
	})
	log.SetOutput(os.Stdout)

	// Set log level based on input
	level := parseLogLevel(logLevel)
	log.SetLevel(level)

	if level == log.DebugLevel {
		log.Debug("Debug logging enabled")
	}
}

// parseLogLevel converts string to logrus.Level
func parseLogLevel(level string) log.Level {
	switch strings.ToLower(level) {
	case "debug":
		return log.DebugLevel
	case "info":
		return log.InfoLevel
	case "warn", "warning":
		return log.WarnLevel
	case "error":
		return log.ErrorLevel
	default:
		return log.InfoLevel
	}
}
