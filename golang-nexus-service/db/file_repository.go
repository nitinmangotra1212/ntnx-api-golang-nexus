package db

import (
	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config"
)

// FileRepository defines the interface for file storage operations
type FileRepository interface {
	// SaveFile saves a file and returns its extId
	SaveFile(fileName string, data []byte, contentType string) (string, *pb.File, error)
	// GetFile retrieves a file by extId, returns data, fileName, contentType, and File model
	GetFile(extId string) ([]byte, string, string, *pb.File, error)
	// DeleteFile deletes a file by extId
	DeleteFile(extId string) error
	// ListFiles lists all files (optional, for future use)
	ListFiles() ([]*pb.File, error)
}

