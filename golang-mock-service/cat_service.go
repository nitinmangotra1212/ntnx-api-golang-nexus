// Package service contains business logic for the mock API
package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	codegen "github.com/nutanix/ntnx-api-golang-mock/golang-mock-codegen"
	log "github.com/sirupsen/logrus"
)

// CatService defines the interface for cat operations
type CatService interface {
	GetCats(params *codegen.ListCatsParams) (*codegen.CatListResponse, error)
	GetCatByID(id int) (*codegen.CatResponse, error)
	CreateCat(cat *codegen.CatCreate) (*codegen.CatResponse, error)
	UpdateCatByID(id int, cat *codegen.CatUpdate) (*codegen.CatResponse, error)
	DeleteCatByID(id int) (*codegen.CatResponse, error)
	AddIPv4ToCat(id int, ipv4 *codegen.IPv4) (*codegen.IPv4Response, error)
	AddIPv6ToCat(id int, ipv6 *codegen.IPv6) (*codegen.IPv6Response, error)
	AddIPAddressToCat(id int, ipAddr *codegen.IPAddress) (*codegen.IPAddressResponse, error)
	GetCatStatusByUUID(ntnxID, userID string) (*codegen.TaskResponse, error)
}

// catServiceImpl implements CatService
type catServiceImpl struct {
	minID int
	maxID int
}

// NewCatService creates a new instance of CatService
func NewCatService() CatService {
	return &catServiceImpl{
		minID: 1,
		maxID: 100,
	}
}

// Constants for mock data
const (
	defaultCatNameNotFiltered = "Kitty"
	defaultCatNameFiltered    = "Filtered Cat Name"
	defaultCatType            = "Briman"
	defaultCatDesc            = "Like to play with ball."

	smallCatNameNotFiltered = "KittyKittyKittyKittyKittyKittyKittyKittyKittyKittyKittyKittyKittyKittyKittyKittyKittyKittyKittyKittyKittyKitty"
	smallCatNameFiltered    = "FilteredCatNameFFilteredCatNameilteredCatNameFilteredCatNameFilteredCatName"
	smallCatType            = "BrimanBrimanBrimanBrimanBrimanBrimanBrimanBrimanBrimanBrimanBrimanBrimanBrimanBrimanBrimanBrimanBrimanBrimanBrimanBrimanBrimanBriman"
	smallCatDesc            = "Like to play with ball. Like to play with ball. Like to play with ball. Like to play with ball. Like to play with ball."
)

// GetCats retrieves a list of cats with optional filtering
func (s *catServiceImpl) GetCats(params *codegen.ListCatsParams) (*codegen.CatListResponse, error) {
	log.Info("Called GetCats service")

	// Handle artificial delay
	if params.Delay != nil && *params.Delay > 0 {
		log.Infof("Sleeping for %d milliseconds", *params.Delay)
		time.Sleep(time.Duration(*params.Delay) * time.Millisecond)
	}

	// Set defaults
	page := 1
	if params.Page != nil {
		page = *params.Page
	}

	limit := 10
	if params.Limit != nil {
		limit = *params.Limit
	}

	size := ""
	if params.Size != nil {
		size = *params.Size
	}

	// Determine which cat name to use based on filter
	catName := defaultCatNameNotFiltered
	if params.Filter != nil && *params.Filter != "" {
		catName = defaultCatNameFiltered
	}

	// Generate mock cats
	cats := make([]codegen.Cat, 0, limit)
	for i := 1; i <= limit; i++ {
		cat := s.generateCat(i, catName, size)
		cats = append(cats, cat)
	}

	// Build metadata
	metadata := s.buildMetadata(limit, page, limit, "/mock/v4/config/cats")

	response := &codegen.CatListResponse{
		Data:     cats,
		Metadata: metadata,
	}

	log.Infof("Returning %d cats", len(cats))
	return response, nil
}

// generateCat creates a mock cat with variable size
func (s *catServiceImpl) generateCat(id int, baseName, size string) codegen.Cat {
	var catName, description string

	if size == "" {
		catName = baseName
		description = defaultCatDesc
	} else {
		iterations := s.getSizeIterations(size)
		catName = s.repeatString(smallCatNameNotFiltered, iterations)
		description = s.repeatString(smallCatDesc, iterations)
	}

	catID := id
	return codegen.Cat{
		CatID:       &catID,
		CatName:     catName,
		CatType:     "TYPE1",
		Description: description,
	}
}

// getSizeIterations returns the number of iterations based on size
func (s *catServiceImpl) getSizeIterations(size string) int {
	switch size {
	case "small":
		return 2
	case "medium":
		return 10
	case "large":
		return 15
	case "huge":
		return 20
	default:
		return 1
	}
}

// repeatString repeats a string n times
func (s *catServiceImpl) repeatString(str string, n int) string {
	var builder strings.Builder
	for i := 0; i < n; i++ {
		builder.WriteString(str)
	}
	return builder.String()
}

// GetCatByID retrieves a single cat by ID
func (s *catServiceImpl) GetCatByID(id int) (*codegen.CatResponse, error) {
	log.Infof("Getting cat with ID: %d", id)

	if id <= s.minID || id >= s.maxID {
		return nil, errors.New("cat not found")
	}

	catID := id
	cat := &codegen.Cat{
		CatID:       &catID,
		CatName:     fmt.Sprintf("AnyName%d", id),
		CatType:     "TYPE1",
		Description: fmt.Sprintf("AnyDescription%d", id),
	}

	metadata := s.buildMetadata(1, 1, 1, fmt.Sprintf("/cat/%d", id))

	response := &codegen.CatResponse{
		Data:     cat,
		Metadata: metadata,
	}

	log.Infof("Retrieved cat: %s", cat.CatName)
	return response, nil
}

// CreateCat creates a new cat
func (s *catServiceImpl) CreateCat(catCreate *codegen.CatCreate) (*codegen.CatResponse, error) {
	log.Info("Creating new cat")

	catID := 1
	cat := &codegen.Cat{
		CatID:       &catID,
		CatName:     catCreate.CatName,
		CatType:     catCreate.CatType,
		Description: catCreate.Description,
	}

	metadata := s.buildMetadata(1, 1, 1, "/cats")

	response := &codegen.CatResponse{
		Data:     cat,
		Metadata: metadata,
	}

	log.Infof("Created cat: %s", cat.CatName)
	return response, nil
}

// UpdateCatByID updates an existing cat
func (s *catServiceImpl) UpdateCatByID(id int, catUpdate *codegen.CatUpdate) (*codegen.CatResponse, error) {
	log.Infof("Updating cat with ID: %d", id)

	if id <= s.minID || id >= s.maxID {
		return nil, errors.New("cat not found")
	}

	catID := id
	cat := &codegen.Cat{
		CatID:       &catID,
		CatName:     fmt.Sprintf("UpdatedName%d", id),
		CatType:     "TYPE1",
		Description: fmt.Sprintf("UpdatedDescription%d", id),
	}

	if catUpdate.CatName != nil {
		cat.CatName = *catUpdate.CatName
	}
	if catUpdate.CatType != nil {
		cat.CatType = *catUpdate.CatType
	}
	if catUpdate.Description != nil {
		cat.Description = *catUpdate.Description
	}

	metadata := s.buildMetadata(1, 1, 1, fmt.Sprintf("/cat/%d", id))

	response := &codegen.CatResponse{
		Data:     cat,
		Metadata: metadata,
	}

	log.Infof("Updated cat: %s", cat.CatName)
	return response, nil
}

// DeleteCatByID deletes a cat by ID
func (s *catServiceImpl) DeleteCatByID(id int) (*codegen.CatResponse, error) {
	log.Infof("Deleting cat with ID: %d", id)

	if id <= s.minID || id >= s.maxID {
		return nil, errors.New("cat not found")
	}

	catID := id
	cat := &codegen.Cat{
		CatID:       &catID,
		CatName:     fmt.Sprintf("DeletedName%d", id),
		CatType:     "TYPE1",
		Description: fmt.Sprintf("DeletedDescription%d", id),
	}

	metadata := s.buildMetadata(1, 1, 1, fmt.Sprintf("/cat/%d", id))

	response := &codegen.CatResponse{
		Data:     cat,
		Metadata: metadata,
	}

	log.Infof("Deleted cat: %s", cat.CatName)
	return response, nil
}

// AddIPv4ToCat adds an IPv4 address to a cat
func (s *catServiceImpl) AddIPv4ToCat(id int, ipv4 *codegen.IPv4) (*codegen.IPv4Response, error) {
	log.Infof("Adding IPv4 to cat with ID: %d", id)

	if id <= s.minID || id >= s.maxID {
		return nil, errors.New("cat not found")
	}

	catID := id
	ipv4Data := &codegen.IPv4{
		CatID: &catID,
		Value: ipv4.Value,
	}

	if ipv4.PrefixLength != nil {
		ipv4Data.PrefixLength = ipv4.PrefixLength
	} else {
		defaultPrefix := 32
		ipv4Data.PrefixLength = &defaultPrefix
	}

	metadata := s.buildMetadata(1, 1, 1, fmt.Sprintf("/cat/%d/ipv4", id))

	response := &codegen.IPv4Response{
		Data:     ipv4Data,
		Metadata: metadata,
	}

	log.Infof("Added IPv4: %s to cat %d", ipv4Data.Value, id)
	return response, nil
}

// AddIPv6ToCat adds an IPv6 address to a cat
func (s *catServiceImpl) AddIPv6ToCat(id int, ipv6 *codegen.IPv6) (*codegen.IPv6Response, error) {
	log.Infof("Adding IPv6 to cat with ID: %d", id)

	if id <= s.minID || id >= s.maxID {
		return nil, errors.New("cat not found")
	}

	catID := id
	ipv6Data := &codegen.IPv6{
		CatID: &catID,
		Value: ipv6.Value,
	}

	if ipv6.PrefixLength != nil {
		ipv6Data.PrefixLength = ipv6.PrefixLength
	} else {
		defaultPrefix := 128
		ipv6Data.PrefixLength = &defaultPrefix
	}

	metadata := s.buildMetadata(1, 1, 1, fmt.Sprintf("/cat/%d/ipv6", id))

	response := &codegen.IPv6Response{
		Data:     ipv6Data,
		Metadata: metadata,
	}

	log.Infof("Added IPv6: %s to cat %d", ipv6Data.Value, id)
	return response, nil
}

// AddIPAddressToCat adds an IP address (IPv4 or IPv6) to a cat
func (s *catServiceImpl) AddIPAddressToCat(id int, ipAddr *codegen.IPAddress) (*codegen.IPAddressResponse, error) {
	log.Infof("Adding IP address to cat with ID: %d", id)

	if id <= s.minID || id >= s.maxID {
		return nil, errors.New("cat not found")
	}

	ipAddrData := &codegen.IPAddress{
		IPv4: ipAddr.IPv4,
		IPv6: ipAddr.IPv6,
	}

	metadata := s.buildMetadata(1, 1, 1, fmt.Sprintf("/cat/%d/ipaddress", id))

	response := &codegen.IPAddressResponse{
		Data:     ipAddrData,
		Metadata: metadata,
	}

	log.Infof("Added IP address to cat %d", id)
	return response, nil
}

// GetCatStatusByUUID retrieves task status by UUID
func (s *catServiceImpl) GetCatStatusByUUID(ntnxID, userID string) (*codegen.TaskResponse, error) {
	log.Infof("Getting cat status for NTNX-ID: %s, USER-ID: %s", ntnxID, userID)

	taskID := uuid.New().String()
	task := &codegen.Task{
		ExtID: taskID,
	}

	metadata := s.buildMetadata(1, 1, 1, "/cat/status")

	response := &codegen.TaskResponse{
		Data:     task,
		Metadata: metadata,
	}

	log.Infof("Generated task UUID: %s", taskID)
	return response, nil
}

// buildMetadata creates standard API response metadata
func (s *catServiceImpl) buildMetadata(total, page, limit int, path string) *codegen.ApiResponseMetadata {
	links := []codegen.ApiLink{
		{
			Rel:  "self",
			Href: fmt.Sprintf("http://localhost:9009%s", path),
		},
	}

	flags := []codegen.Flag{
		{
			Name:  "isPaginated",
			Value: true,
		},
	}

	totalResults := total
	now := time.Now()

	return &codegen.ApiResponseMetadata{
		TotalAvailableResults: &totalResults,
		Links:                 links,
		Flags:                 flags,
		Timestamp:             &now,
	}
}
