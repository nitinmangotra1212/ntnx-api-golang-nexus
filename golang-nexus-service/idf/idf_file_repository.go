/*
 * IDF Repository Implementation for File Entity
 * Stores file metadata in IDF and file data in local filesystem
 */

package idf

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/nutanix-core/go-cache/insights/insights_interface"
	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/db"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/external"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

type FileRepositoryImpl struct {
	fileStoragePath string
}

// IDF Column Names (snake_case)
const (
	fileEntityTypeName = "file"
	fileListPath       = "/files"

	// IDF attribute names (snake_case)
	fileIdAttr      = "file_id"
	fileNameAttr    = "file_name"
	fileSizeAttr    = "file_size"
	contentTypeAttr = "content_type"
	fileExtIdAttr   = "ext_id" // Renamed to avoid conflict with item's extIdAttr
)

// Default file storage path (can be configured via environment variable)
// Using /home/nutanix/Downloads to match licensing service pattern
const defaultFileStoragePath = "/home/nutanix/Downloads"

func NewFileRepository() db.FileRepository {
	storagePath := os.Getenv("NEXUS_FILE_STORAGE_PATH")
	if storagePath == "" {
		storagePath = defaultFileStoragePath
	}

	// Create storage directory if it doesn't exist
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		log.Warnf("Failed to create file storage directory %s: %v", storagePath, err)
	}

	return &FileRepositoryImpl{
		fileStoragePath: storagePath,
	}
}

// SaveFile saves a file to filesystem and metadata to IDF
func (r *FileRepositoryImpl) SaveFile(fileName string, data []byte, contentType string) (string, *pb.File, error) {
	// Generate UUID for extId
	extId := uuid.New().String()

	// Save file data to filesystem
	filePath := filepath.Join(r.fileStoragePath, extId)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		log.Errorf("Failed to save file to filesystem: %v", err)
		return "", nil, fmt.Errorf("failed to save file: %v", err)
	}

	log.Infof("File saved to filesystem: %s (size: %d bytes)", filePath, len(data))

	// Get IDF client from singleton
	idfClient := external.Interfaces().IdfClient()

	// Prepare IDF attributes
	attributeDataArgList := []*insights_interface.AttributeDataArg{}

	// Store file metadata in IDF
	AddAttribute(&attributeDataArgList, fileExtIdAttr, extId)
	AddAttribute(&attributeDataArgList, fileNameAttr, fileName)
	AddAttribute(&attributeDataArgList, fileSizeAttr, int64(len(data)))
	if contentType != "" {
		AddAttribute(&attributeDataArgList, contentTypeAttr, contentType)
	}

	updateArg := &insights_interface.UpdateEntityArg{
		EntityGuid: &insights_interface.EntityGuid{
			EntityTypeName: proto.String(fileEntityTypeName),
			EntityId:       &extId,
		},
		AttributeDataArgList: attributeDataArgList,
	}

	// Save metadata to IDF
	_, err := idfClient.UpdateEntityRet(updateArg)
	if err != nil {
		// Clean up file if IDF save fails
		os.Remove(filePath)
		log.Errorf("Failed to save file metadata to IDF: %v", err)
		return "", nil, fmt.Errorf("failed to save file metadata: %v", err)
	}

	log.Infof("File metadata saved to IDF with extId: %s", extId)

	// Create File protobuf model
	fileSize := int64(len(data))
	file := &pb.File{
		ExtId:      &extId,
		FileName:   &fileName,
		FileSize:   &fileSize,
		ContentType: &contentType,
	}

	return extId, file, nil
}

// GetFile retrieves a file by extId
func (r *FileRepositoryImpl) GetFile(extId string) ([]byte, string, string, *pb.File, error) {
	// First, check if file exists on filesystem
	filePath := filepath.Join(r.fileStoragePath, extId)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Errorf("File not found in filesystem: %s", filePath)
			return nil, "", "", nil, fmt.Errorf("file not found: %s", extId)
		}
		log.Errorf("Failed to read file from filesystem: %v", err)
		return nil, "", "", nil, fmt.Errorf("failed to read file: %v", err)
	}

	// Try to get metadata from IDF (but don't fail if IDF lookup fails)
	fileName := extId // Default to extId if metadata not available
	fileSize := int64(len(data))
	contentType := "application/octet-stream"

	idfClient := external.Interfaces().IdfClient()
	getArg := &insights_interface.GetEntitiesArg{
		EntityGuidList: []*insights_interface.EntityGuid{
			{
				EntityTypeName: proto.String(fileEntityTypeName),
				EntityId:       &extId,
			},
		},
	}

	getRet, err := idfClient.GetEntityRet(getArg)
	if err != nil {
		log.Warnf("Failed to query IDF for file metadata (file exists on disk): %v", err)
		// Continue with default values - file exists on disk
	} else if len(getRet.GetEntity()) > 0 {
		// Extract metadata from IDF entity
		entity := getRet.GetEntity()[0]

		// Extract attributes from AttributeDataMap (NameTimeValuePair format)
		for _, attr := range entity.GetAttributeDataMap() {
			attrName := attr.GetName()
			if attrName == "" {
				continue
			}
			switch attrName {
			case fileNameAttr:
				if attr.GetValue() != nil && attr.GetValue().GetStrValue() != "" {
					fileName = attr.GetValue().GetStrValue()
				}
			case fileSizeAttr:
				if attr.GetValue() != nil {
					fileSize = attr.GetValue().GetInt64Value()
				}
			case contentTypeAttr:
				if attr.GetValue() != nil && attr.GetValue().GetStrValue() != "" {
					contentType = attr.GetValue().GetStrValue()
				}
			}
		}
	} else {
		log.Warnf("File metadata not found in IDF (file exists on disk): %s", extId)
		// Continue with default values - file exists on disk
	}

	// Create File protobuf model
	file := &pb.File{
		ExtId:       &extId,
		FileName:    &fileName,
		FileSize:    &fileSize,
		ContentType: &contentType,
	}

	log.Infof("File retrieved: %s (size: %d bytes)", fileName, len(data))
	return data, fileName, contentType, file, nil
}

// DeleteFile deletes a file by extId
func (r *FileRepositoryImpl) DeleteFile(extId string) error {
	// Delete from filesystem
	filePath := filepath.Join(r.fileStoragePath, extId)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		log.Warnf("Failed to delete file from filesystem: %v", err)
		// Continue to try deleting from IDF
	}

	// TODO: Delete from IDF (if IDF supports delete operations)
	// For now, we just delete from filesystem
	log.Infof("File deleted: %s", extId)
	return nil
}

// ListFiles lists all files (optional, for future use)
func (r *FileRepositoryImpl) ListFiles() ([]*pb.File, error) {
	// TODO: Implement if needed
	return []*pb.File{}, nil
}

