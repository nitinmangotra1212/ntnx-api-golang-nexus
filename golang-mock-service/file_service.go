package service

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/sftp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// FileService defines the interface for file operations
type FileService interface {
	DownloadFile(fileID int, config *SFTPConfig) (string, error)
	UploadFile(reader io.Reader, filename string, config *SFTPConfig) (string, error)
	GetFileNameByID(fileID int) (string, error)
}

// SFTPConfig holds SFTP connection configuration
type SFTPConfig struct {
	RemoteHost     string
	Username       string
	Password       string
	RemoteFilePath string
	DownloadDir    string
	UploadDir      string
	UploadURL      string
}

type fileServiceImpl struct{}

// NewFileService creates a new instance of FileService
func NewFileService() FileService {
	return &fileServiceImpl{}
}

// GetFileNameByID maps file IDs to filenames
func (s *fileServiceImpl) GetFileNameByID(fileID int) (string, error) {
	fileMap := map[int]string{
		2: "file1.txt",
		3: "file2.txt",
		4: "file3.pdf",
		5: "file4.doc",
		6: "file5.zip",
		7: "file6.jpg",
	}

	filename, ok := fileMap[fileID]
	if !ok {
		return "", fmt.Errorf("file with ID %d not found", fileID)
	}

	return filename, nil
}

// DownloadFile downloads a file from SFTP server
func (s *fileServiceImpl) DownloadFile(fileID int, config *SFTPConfig) (string, error) {
	log.Infof("Starting file download for file ID: %d", fileID)

	// Get filename from ID
	filename, err := s.GetFileNameByID(fileID)
	if err != nil {
		return "", err
	}

	// Create SSH client configuration
	sshConfig := &ssh.ClientConfig{
		User: config.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(config.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Use proper host key checking in production
	}

	// Connect to SSH server
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", config.RemoteHost), sshConfig)
	if err != nil {
		log.Errorf("Failed to connect to SSH server: %v", err)
		return "", fmt.Errorf("failed to connect to SSH server: %w", err)
	}
	defer conn.Close()

	// Create SFTP client
	client, err := sftp.NewClient(conn)
	if err != nil {
		log.Errorf("Failed to create SFTP client: %v", err)
		return "", fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer client.Close()

	// Construct remote and local file paths
	remoteFilePath := filepath.Join(config.RemoteFilePath, filename)
	localFilePath := filepath.Join(config.DownloadDir, filename)

	// Ensure download directory exists
	if err := os.MkdirAll(config.DownloadDir, 0755); err != nil {
		log.Errorf("Failed to create download directory: %v", err)
		return "", fmt.Errorf("failed to create download directory: %w", err)
	}

	// Check if remote file exists
	_, err = client.Stat(remoteFilePath)
	if err != nil {
		log.Errorf("Remote file does not exist: %v", err)
		return "", fmt.Errorf("remote file does not exist: %w", err)
	}

	// Open remote file
	srcFile, err := client.Open(remoteFilePath)
	if err != nil {
		log.Errorf("Failed to open remote file: %v", err)
		return "", fmt.Errorf("failed to open remote file: %w", err)
	}
	defer srcFile.Close()

	// Create local file
	dstFile, err := os.Create(localFilePath)
	if err != nil {
		log.Errorf("Failed to create local file: %v", err)
		return "", fmt.Errorf("failed to create local file: %w", err)
	}
	defer dstFile.Close()

	// Copy file content
	bytesWritten, err := io.Copy(dstFile, srcFile)
	if err != nil {
		log.Errorf("Failed to copy file content: %v", err)
		return "", fmt.Errorf("failed to copy file content: %w", err)
	}

	log.Infof("Successfully downloaded file: %s (%d bytes)", filename, bytesWritten)
	return localFilePath, nil
}

// UploadFile uploads a file to SFTP server
func (s *fileServiceImpl) UploadFile(reader io.Reader, filename string, config *SFTPConfig) (string, error) {
	log.Infof("Starting file upload for file: %s", filename)

	if filename == "" {
		return "", errors.New("filename cannot be empty")
	}

	// Create SSH client configuration
	sshConfig := &ssh.ClientConfig{
		User: config.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(config.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Use proper host key checking in production
	}

	// Connect to SSH server
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", config.RemoteHost), sshConfig)
	if err != nil {
		log.Errorf("Failed to connect to SSH server: %v", err)
		return "", fmt.Errorf("failed to connect to SSH server: %w", err)
	}
	defer conn.Close()

	// Create SFTP client
	client, err := sftp.NewClient(conn)
	if err != nil {
		log.Errorf("Failed to create SFTP client: %v", err)
		return "", fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer client.Close()

	// Construct remote file path
	remoteFilePath := filepath.Join(config.UploadDir, filename)

	// Create remote file
	dstFile, err := client.Create(remoteFilePath)
	if err != nil {
		log.Errorf("Failed to create remote file: %v", err)
		return "", fmt.Errorf("failed to create remote file: %w", err)
	}
	defer dstFile.Close()

	// Copy file content
	bytesWritten, err := io.Copy(dstFile, reader)
	if err != nil {
		log.Errorf("Failed to upload file content: %v", err)
		return "", fmt.Errorf("failed to upload file content: %w", err)
	}

	fileURL := fmt.Sprintf("%s%s", config.UploadURL, filename)
	log.Infof("Successfully uploaded file: %s (%d bytes) to %s", filename, bytesWritten, fileURL)
	return fileURL, nil
}
