/*
 * gRPC Service Implementation for File Service
 * Implements file upload (client-streaming) and download (server-streaming)
 * Following patterns from v4 APIs gRPC File Transfer support
 */

package grpc

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/db"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	BYTE_ARRAY_SIZE = 4 * 1024 * 1024 // 4MB chunks
	ONE_GB_IN_BYTES = 1 * 1024 * 1024 * 1024 // 1GB limit
	FILE_IDENTIFIER = "File-Identifier"
	CONTENT_LENGTH  = "Content-Length"
	CONTENT_DISPOSITION = "Content-Disposition"
)

// FileGrpcService implements the gRPC FileService
type FileGrpcService struct {
	pb.UnimplementedFileServiceServer
	fileRepository db.FileRepository
}

// NewFileGrpcService creates a new gRPC File service with file repository
func NewFileGrpcService(fileRepository db.FileRepository) *FileGrpcService {
	service := &FileGrpcService{
		fileRepository: fileRepository,
	}
	log.Info("✅ Initialized gRPC File Service with file repository")
	return service
}

// UploadFile implements the gRPC UploadFile RPC (client-streaming)
// Client sends stream of file chunks, server receives and saves the file
func (s *FileGrpcService) UploadFile(stream grpc.ClientStreamingServer[pb.UploadFileArg, pb.UploadFileRet]) error {
	log.Info("gRPC: UploadFile called (client-streaming)")

	// Get metadata from gRPC headers
	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return status.Errorf(codes.InvalidArgument, "Missing metadata in request")
	}

	// Extract fileName from File-Identifier header
	fileName := ""
	if filenames := md.Get(FILE_IDENTIFIER); len(filenames) > 0 {
		fileName = filenames[0]
	}

	// Extract Content-Disposition header to parse filename if File-Identifier is not set
	if fileName == "" {
		if dispositions := md.Get(CONTENT_DISPOSITION); len(dispositions) > 0 {
			// Parse: attachment;filename="file.txt"
			disposition := dispositions[0]
			if strings.Contains(disposition, "filename=") {
				parts := strings.Split(disposition, "filename=")
				if len(parts) > 1 {
					fileName = strings.Trim(strings.Trim(parts[1], `"`), "'")
				}
			}
		}
	}

	if fileName == "" {
		fileName = "uploaded_file" // Default filename
		log.Warn("No filename found in headers, using default: uploaded_file")
	}

	// Extract Content-Length from headers (if available)
	contentLength := int64(0)
	if lengths := md.Get(CONTENT_LENGTH); len(lengths) > 0 {
		if cl, err := strconv.ParseInt(lengths[0], 10, 64); err == nil {
			contentLength = cl
			if contentLength > ONE_GB_IN_BYTES {
				return status.Errorf(codes.InvalidArgument,
					"File size %d exceeds 1GB limit", contentLength)
			}
		}
	}

	// Extract Content-Type from headers
	contentType := "application/octet-stream"
	if types := md.Get("content-type"); len(types) > 0 {
		contentType = types[0]
	}

	log.Infof("Uploading file: %s (Content-Length: %d, Content-Type: %s)", fileName, contentLength, contentType)

	// Receive stream chunks and accumulate file data
	var fileData []byte
	totalReceived := int64(0)

		for {
		// Check for cancellation
		if stream.Context().Err() != nil {
			return status.Errorf(codes.Canceled, "Request cancelled")
		}

		chunk, err := stream.Recv()
		if err == io.EOF {
			log.Info("Received EOF, file upload complete")
			break
		}
		if err != nil {
			log.Errorf("Error receiving chunk: %v", err)
			return status.Errorf(codes.Internal, "Error receiving chunk: %v", err)
		}

		// Extract data from chunk
		chunkData := chunk.GetData()
		if len(chunkData) > 0 {
			fileData = append(fileData, chunkData...)
			totalReceived += int64(len(chunkData))

			// Validate total size
			if totalReceived > ONE_GB_IN_BYTES {
				return status.Errorf(codes.InvalidArgument,
					"File size %d exceeds 1GB limit", totalReceived)
			}
		}
	}

	log.Infof("Received file: %s (total size: %d bytes)", fileName, totalReceived)

	// Save file using repository
	extId, _, err := s.fileRepository.SaveFile(fileName, fileData, contentType)
	if err != nil {
		log.Errorf("Failed to save file: %v", err)
		return status.Errorf(codes.Internal, "Failed to save file: %v", err)
	}

	log.Infof("✅ File uploaded successfully: extId=%s, fileName=%s, size=%d", extId, fileName, len(fileData))

	// Create file model for response
	fileSize := int64(len(fileData))
	fileModel := &pb.File{
		ExtId:       &extId,
		FileName:    &fileName,
		FileSize:    &fileSize,
		ContentType: &contentType,
	}

	// Create response with File model
	response := &pb.UploadFileRet{
		Content: fileModel,
		Reserved: map[string]string{
			"Location": fmt.Sprintf("/nexus/v4.1/config/download/%s", extId),
		},
	}

	// Send and close the stream
	return stream.SendAndClose(response)
}

// DownloadFile implements the gRPC DownloadFile RPC (server-streaming)
// Server sends stream of file chunks to client
func (s *FileGrpcService) DownloadFile(req *pb.DownloadFileArg, stream grpc.ServerStreamingServer[pb.DownloadFileRet]) error {
	log.Info("gRPC: DownloadFile called (server-streaming)")

	// Get extId from request argument (passed from path parameter via Adonis)
	extId := req.GetExtId()
	if extId == "" {
		// Fallback to metadata if not in request
		md, ok := metadata.FromIncomingContext(stream.Context())
		if ok {
			if extIds := md.Get(FILE_IDENTIFIER); len(extIds) > 0 {
				extId = extIds[0]
			}
		}
		if extId == "" {
			return status.Errorf(codes.InvalidArgument, "File identifier (extId) not found")
		}
	}

	log.Infof("Downloading file: extId=%s", extId)

	// Load file from storage
	fileData, fileName, contentType, _, err := s.fileRepository.GetFile(extId)
	if err != nil {
		log.Errorf("Failed to get file: %v", err)
		return status.Errorf(codes.NotFound, "File not found: %s", extId)
	}

	fileSize := int64(len(fileData))
	log.Infof("File found: %s (size: %d bytes, contentType: %s)", fileName, fileSize, contentType)

	// Validate file size
	if fileSize > ONE_GB_IN_BYTES {
		return status.Errorf(codes.InvalidArgument,
			"File size %d exceeds 1GB limit", fileSize)
	}

	// Set response headers via metadata
	header := metadata.New(map[string]string{
		CONTENT_LENGTH:      strconv.FormatInt(fileSize, 10),
		CONTENT_DISPOSITION: fmt.Sprintf(`attachment; filename="%s"`, fileName),
	})
	if err := stream.SendHeader(header); err != nil {
		log.Errorf("Failed to send headers: %v", err)
		return status.Errorf(codes.Internal, "Failed to send headers: %v", err)
	}

	log.Infof("Sent headers: Content-Length=%d, Content-Disposition=%s", fileSize, header.Get(CONTENT_DISPOSITION)[0])

	// Stream file in chunks (4MB)
	chunkSize := BYTE_ARRAY_SIZE
		for i := 0; i < len(fileData); i += chunkSize {
		// Check for cancellation
		if stream.Context().Err() != nil {
			log.Warn("Download cancelled by client")
			return status.Errorf(codes.Canceled, "Request cancelled")
		}

		end := i + chunkSize
		if end > len(fileData) {
			end = len(fileData)
		}

		chunk := &pb.DownloadFileRet{
			Data: fileData[i:end],
		}

		if err := stream.Send(chunk); err != nil {
			log.Errorf("Failed to send chunk: %v", err)
			return status.Errorf(codes.Internal, "Failed to send chunk: %v", err)
		}
	}

	log.Infof("✅ File download complete: extId=%s, fileName=%s, size=%d", extId, fileName, fileSize)
	return nil
}

