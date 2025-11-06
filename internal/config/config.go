// Package config handles application configuration
package config

import (
	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Mockrest MockrestConfig `mapstructure:"mockrest"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port int `mapstructure:"port"`
}

// MockrestConfig holds mockrest-specific configuration
type MockrestConfig struct {
	FileServer FileServerConfig `mapstructure:"file-server"`
}

// FileServerConfig holds SFTP file server configuration
type FileServerConfig struct {
	RemoteHost     string `mapstructure:"remoteHost"`
	Username       string `mapstructure:"username"`
	Password       string `mapstructure:"password"`
	RemoteFilePath string `mapstructure:"remoteFilePath"`
	DownloadDir    string `mapstructure:"download-directory"`
	UploadDir      string `mapstructure:"upload-directory"`
	UploadURL      string `mapstructure:"upload-url"`
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// Set defaults
	viper.SetDefault("server.port", 9009)
	viper.SetDefault("mockrest.file-server.remoteHost", "10.46.1.165")
	viper.SetDefault("mockrest.file-server.username", "nutanix")
	viper.SetDefault("mockrest.file-server.password", "nutanix/4u")
	viper.SetDefault("mockrest.file-server.remoteFilePath", "/home/nutanix/data/nitin")
	viper.SetDefault("mockrest.file-server.download-directory", "/tmp/downloaded_files")
	viper.SetDefault("mockrest.file-server.upload-directory", "/home/nutanix/data/nitin/uploads/")
	viper.SetDefault("mockrest.file-server.upload-url", "http://10.46.1.165/nitin/uploads/")

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		// Config file not found; ignore error if desired
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	// Allow environment variables to override config file
	viper.AutomaticEnv()

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
