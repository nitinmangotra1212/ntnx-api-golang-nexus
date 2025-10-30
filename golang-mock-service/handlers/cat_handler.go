// Package handlers contains HTTP request handlers
package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	codegen "github.com/nutanix/ntnx-api-golang-mock/golang-mock-codegen"
	service "github.com/nutanix/ntnx-api-golang-mock/golang-mock-service"
	log "github.com/sirupsen/logrus"
)

// CatHandler handles HTTP requests for cat operations
type CatHandler struct {
	catService  service.CatService
	fileService service.FileService
	sftpConfig  *service.SFTPConfig
}

// NewCatHandler creates a new CatHandler
func NewCatHandler(catService service.CatService, fileService service.FileService, sftpConfig *service.SFTPConfig) *CatHandler {
	return &CatHandler{
		catService:  catService,
		fileService: fileService,
		sftpConfig:  sftpConfig,
	}
}

// ListCats handles GET /cats
func (h *CatHandler) ListCats(c *gin.Context) {
	log.Info("Request reached the mockrest backend getCats handler")

	var params codegen.ListCatsParams
	if err := c.ShouldBindQuery(&params); err != nil {
		log.Errorf("Failed to bind query parameters: %v", err)
		c.JSON(http.StatusBadRequest, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "INVALID_PARAMETERS",
				Message: "Invalid query parameters",
				Details: err.Error(),
			},
		})
		return
	}

	response, err := h.catService.GetCats(&params)
	if err != nil {
		log.Errorf("Failed to get cats: %v", err)
		c.JSON(http.StatusInternalServerError, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to retrieve cats",
				Details: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetCatByID handles GET /cat/:catId
func (h *CatHandler) GetCatByID(c *gin.Context) {
	log.Info("Request reached the mock backend getCatById handler")

	catIDStr := c.Param("catId")
	catID, err := strconv.Atoi(catIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "INVALID_CAT_ID",
				Message: "Invalid cat ID",
				Details: "Cat ID must be an integer",
			},
		})
		return
	}

	response, err := h.catService.GetCatByID(catID)
	if err != nil {
		log.Errorf("Cat not found: %v", err)
		c.JSON(http.StatusNotFound, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "NOT_FOUND",
				Message: "Cat not found",
				Details: fmt.Sprintf("Cat with ID %d does not exist", catID),
			},
		})
		return
	}

	log.Infof("Getting cat with ID: %d", catID)
	c.JSON(http.StatusOK, response)
}

// CreateCat handles POST /cats
func (h *CatHandler) CreateCat(c *gin.Context) {
	log.Info("Request reached the createCat handler")

	var catCreate codegen.CatCreate
	if err := c.ShouldBindJSON(&catCreate); err != nil {
		log.Errorf("Failed to bind JSON: %v", err)
		c.JSON(http.StatusBadRequest, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request body",
				Details: err.Error(),
			},
		})
		return
	}

	response, err := h.catService.CreateCat(&catCreate)
	if err != nil {
		log.Errorf("Failed to create cat: %v", err)
		c.JSON(http.StatusInternalServerError, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to create cat",
				Details: err.Error(),
			},
		})
		return
	}

	log.Infof("Created cat: %s", catCreate.CatName)
	c.JSON(http.StatusCreated, response)
}

// UpdateCatByID handles PUT /cat/:catId
func (h *CatHandler) UpdateCatByID(c *gin.Context) {
	log.Info("Request reached the updateCatById handler")

	catIDStr := c.Param("catId")
	catID, err := strconv.Atoi(catIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "INVALID_CAT_ID",
				Message: "Invalid cat ID",
				Details: "Cat ID must be an integer",
			},
		})
		return
	}

	var catUpdate codegen.CatUpdate
	if err := c.ShouldBindJSON(&catUpdate); err != nil {
		log.Errorf("Failed to bind JSON: %v", err)
		c.JSON(http.StatusBadRequest, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request body",
				Details: err.Error(),
			},
		})
		return
	}

	response, err := h.catService.UpdateCatByID(catID, &catUpdate)
	if err != nil {
		log.Errorf("Cat not found: %v", err)
		c.JSON(http.StatusNotFound, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "NOT_FOUND",
				Message: "Cat not found",
				Details: fmt.Sprintf("Cat with ID %d does not exist", catID),
			},
		})
		return
	}

	log.Infof("Updated cat with ID: %d", catID)
	c.JSON(http.StatusOK, response)
}

// DeleteCatByID handles DELETE /cat/:catId
func (h *CatHandler) DeleteCatByID(c *gin.Context) {
	log.Info("Request reached the deleteCatById handler")

	catIDStr := c.Param("catId")
	catID, err := strconv.Atoi(catIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "INVALID_CAT_ID",
				Message: "Invalid cat ID",
				Details: "Cat ID must be an integer",
			},
		})
		return
	}

	response, err := h.catService.DeleteCatByID(catID)
	if err != nil {
		log.Errorf("Cat not found: %v", err)
		c.JSON(http.StatusNotFound, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "NOT_FOUND",
				Message: "Cat not found",
				Details: fmt.Sprintf("Cat with ID %d does not exist", catID),
			},
		})
		return
	}

	log.Infof("Deleted cat with ID: %d", catID)
	c.JSON(http.StatusOK, response)
}

// AddIPv4ToCat handles POST /cat/:catId/ipv4
func (h *CatHandler) AddIPv4ToCat(c *gin.Context) {
	log.Info("Request reached the addIPv4ToCat handler")

	catIDStr := c.Param("catId")
	catID, err := strconv.Atoi(catIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "INVALID_CAT_ID",
				Message: "Invalid cat ID",
				Details: "Cat ID must be an integer",
			},
		})
		return
	}

	var ipv4 codegen.IPv4
	if err := c.ShouldBindJSON(&ipv4); err != nil {
		log.Errorf("Failed to bind JSON: %v", err)
		c.JSON(http.StatusBadRequest, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request body",
				Details: err.Error(),
			},
		})
		return
	}

	response, err := h.catService.AddIPv4ToCat(catID, &ipv4)
	if err != nil {
		log.Errorf("Cat not found: %v", err)
		c.JSON(http.StatusNotFound, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "NOT_FOUND",
				Message: "Cat not found",
				Details: fmt.Sprintf("Cat with ID %d does not exist", catID),
			},
		})
		return
	}

	log.Infof("Added IPv4 to cat with ID: %d", catID)
	c.JSON(http.StatusCreated, response)
}

// AddIPv6ToCat handles POST /cat/:catId/ipv6
func (h *CatHandler) AddIPv6ToCat(c *gin.Context) {
	log.Info("Request reached the addIPv6ToCat handler")

	catIDStr := c.Param("catId")
	catID, err := strconv.Atoi(catIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "INVALID_CAT_ID",
				Message: "Invalid cat ID",
				Details: "Cat ID must be an integer",
			},
		})
		return
	}

	var ipv6 codegen.IPv6
	if err := c.ShouldBindJSON(&ipv6); err != nil {
		log.Errorf("Failed to bind JSON: %v", err)
		c.JSON(http.StatusBadRequest, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request body",
				Details: err.Error(),
			},
		})
		return
	}

	response, err := h.catService.AddIPv6ToCat(catID, &ipv6)
	if err != nil {
		log.Errorf("Cat not found: %v", err)
		c.JSON(http.StatusNotFound, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "NOT_FOUND",
				Message: "Cat not found",
				Details: fmt.Sprintf("Cat with ID %d does not exist", catID),
			},
		})
		return
	}

	log.Infof("Added IPv6 to cat with ID: %d", catID)
	c.JSON(http.StatusCreated, response)
}

// AddIPAddressToCat handles POST /cat/:catId/ipaddress
func (h *CatHandler) AddIPAddressToCat(c *gin.Context) {
	log.Info("Request reached the addIPAddressToCat handler")

	catIDStr := c.Param("catId")
	catID, err := strconv.Atoi(catIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "INVALID_CAT_ID",
				Message: "Invalid cat ID",
				Details: "Cat ID must be an integer",
			},
		})
		return
	}

	var ipAddress codegen.IPAddress
	if err := c.ShouldBindJSON(&ipAddress); err != nil {
		log.Errorf("Failed to bind JSON: %v", err)
		c.JSON(http.StatusBadRequest, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request body",
				Details: err.Error(),
			},
		})
		return
	}

	response, err := h.catService.AddIPAddressToCat(catID, &ipAddress)
	if err != nil {
		log.Errorf("Cat not found: %v", err)
		c.JSON(http.StatusNotFound, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "NOT_FOUND",
				Message: "Cat not found",
				Details: fmt.Sprintf("Cat with ID %d does not exist", catID),
			},
		})
		return
	}

	log.Infof("Added IP address to cat with ID: %d", catID)
	c.JSON(http.StatusCreated, response)
}

// GetCatStatusByUUID handles GET /cat/status
func (h *CatHandler) GetCatStatusByUUID(c *gin.Context) {
	log.Info("Request reached the getCatStatusByUUID handler")

	ntnxRequestID := c.GetHeader("NTNX-Request-ID")
	userRequestID := c.GetHeader("USER-Request-ID")

	if ntnxRequestID == "" || userRequestID == "" {
		c.JSON(http.StatusBadRequest, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "MISSING_HEADERS",
				Message: "Missing required headers",
				Details: "NTNX-Request-ID and USER-Request-ID headers are required",
			},
		})
		return
	}

	response, err := h.catService.GetCatStatusByUUID(ntnxRequestID, userRequestID)
	if err != nil {
		log.Errorf("Failed to get task status: %v", err)
		c.JSON(http.StatusInternalServerError, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to retrieve task status",
				Details: err.Error(),
			},
		})
		return
	}

	log.Infof("Retrieved task status for NTNX-ID: %s, USER-ID: %s", ntnxRequestID, userRequestID)
	c.JSON(http.StatusOK, response)
}

// DownloadFile handles GET /cat/:fileId/downloadFile
func (h *CatHandler) DownloadFile(c *gin.Context) {
	log.Info("Request reached the downloadFile handler")

	fileIDStr := c.Param("fileId")
	fileID, err := strconv.Atoi(fileIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "INVALID_FILE_ID",
				Message: "Invalid file ID",
				Details: "File ID must be an integer",
			},
		})
		return
	}

	// Download file from SFTP
	localFilePath, err := h.fileService.DownloadFile(fileID, h.sftpConfig)
	if err != nil {
		log.Errorf("Failed to download file: %v", err)
		c.JSON(http.StatusNotFound, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "NOT_FOUND",
				Message: "File not found",
				Details: fmt.Sprintf("File with ID %d does not exist or could not be downloaded", fileID),
			},
		})
		return
	}

	// Get filename from path
	filename := filepath.Base(localFilePath)

	// Serve the file
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/octet-stream")
	c.File(localFilePath)

	log.Infof("File downloaded successfully: %s", filename)
}

// UploadFile handles POST /cat/:catId/uploadFile
func (h *CatHandler) UploadFile(c *gin.Context) {
	log.Info("Entered the POST call method of uploadFile in the backend handler")

	catIDStr := c.Param("catId")
	_, err := strconv.Atoi(catIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "INVALID_CAT_ID",
				Message: "Invalid cat ID",
				Details: "Cat ID must be an integer",
			},
		})
		return
	}

	// Get filename from Content-Disposition header
	contentDisposition := c.GetHeader("Content-Disposition")
	filename := extractFilename(contentDisposition)
	if filename == "" {
		log.Error("Filename not found in Content-Disposition header")
		c.JSON(http.StatusBadRequest, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "MISSING_FILENAME",
				Message: "Filename not provided",
				Details: "Content-Disposition header must include filename",
			},
		})
		return
	}

	log.Infof("Extracted filename: %s", filename)

	// Get request body as reader
	reader := c.Request.Body
	defer reader.Close()

	// Upload file to SFTP
	fileURL, err := h.fileService.UploadFile(reader, filename, h.sftpConfig)
	if err != nil {
		log.Errorf("Failed to upload file: %v", err)
		c.JSON(http.StatusInternalServerError, codegen.ErrorResponse{
			Error: codegen.ErrorDetail{
				Code:    "UPLOAD_FAILED",
				Message: "File upload failed",
				Details: err.Error(),
			},
		})
		return
	}

	log.Infof("File uploaded successfully: %s", fileURL)
	c.String(http.StatusOK, "File uploaded successfully: %s", fileURL)
}

// extractFilename extracts filename from Content-Disposition header
func extractFilename(contentDisposition string) string {
	if contentDisposition == "" {
		return ""
	}

	// Simple extraction - in production use proper parsing
	// Format: filename="test.txt" or filename=test.txt
	start := -1
	end := -1

	for i := 0; i < len(contentDisposition); i++ {
		if contentDisposition[i] == '"' {
			if start == -1 {
				start = i + 1
			} else {
				end = i
				break
			}
		}
	}

	if start > 0 && end > start {
		return contentDisposition[start:end]
	}

	// Try without quotes
	if idx := len("filename="); len(contentDisposition) > idx {
		return contentDisposition[idx:]
	}

	return ""
}
