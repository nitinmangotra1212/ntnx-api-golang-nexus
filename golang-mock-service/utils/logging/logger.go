/*
 * Copyright (c) 2025 Nutanix Inc. All rights reserved.
 */

package logging

import (
	"os"

	log "github.com/sirupsen/logrus"
)

// InitLogger initializes the logger with hot-reloading capability
func InitLogger() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		ForceColors:     true,
	})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
}

