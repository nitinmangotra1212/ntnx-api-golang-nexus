/*
 * gRPC Service Implementation for Cat Service
 * Following Nutanix patterns from ntnx-api-guru
 */

package grpc

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	pb "github.com/nutanix/ntnx-api-golang-mock-pc/generated-code/protobuf/mock/v4/config"
	"github.com/nutanix/ntnx-api-golang-mock/golang-mock-service/global"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CatGrpcService implements the gRPC CatService
type CatGrpcService struct {
	pb.UnimplementedCatServiceServer
	catMutex sync.RWMutex
	cats     map[int32]*pb.Cat
}

// NewCatGrpcService creates a new gRPC Cat service
func NewCatGrpcService() *CatGrpcService {
	service := &CatGrpcService{
		cats: make(map[int32]*pb.Cat),
	}

	// Initialize with mock data
	service.initializeMockCats()

	return service
}

// Initialize mock cats
func (s *CatGrpcService) initializeMockCats() {
	log.Info("🎯 Initializing gRPC Cat Service with mock data")

	s.catMutex.Lock()
	defer s.catMutex.Unlock()

	for i := int32(1); i <= 100; i++ {
		cat := &pb.Cat{
			CatId:       i,
			CatName:     fmt.Sprintf("Cat-%d", i),
			CatType:     "TYPE1",
			Description: "A fluffy cat",
		}

		// Add location for even numbered cats
		if i%2 == 0 {
			cat.Location = &pb.Location{
				City: "San Francisco",
				Country: &pb.Country{
					State: "California",
				},
			}
		}

		s.cats[i] = cat
	}

	log.Infof("✅ Initialized %d cats in gRPC service", len(s.cats))
}

// ListCats implements the gRPC ListCats RPC
func (s *CatGrpcService) ListCats(ctx context.Context, req *pb.ListCatsRequest) (*pb.ListCatsResponse, error) {
	log.Infof("gRPC: ListCats called (page=%d, limit=%d)", req.Page, req.Limit)

	s.catMutex.RLock()
	defer s.catMutex.RUnlock()

	// Set defaults
	page := req.Page
	if page <= 0 {
		page = 1
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}

	// Collect all cats
	allCats := make([]*pb.Cat, 0, len(s.cats))
	for _, cat := range s.cats {
		allCats = append(allCats, cat)
	}

	// Calculate pagination
	totalCount := int32(len(allCats))
	startIdx := (page - 1) * limit
	endIdx := startIdx + limit

	if startIdx >= totalCount {
		return &pb.ListCatsResponse{
			Cats:       []*pb.Cat{},
			TotalCount: totalCount,
			Page:       page,
			Limit:      limit,
		}, nil
	}

	if endIdx > totalCount {
		endIdx = totalCount
	}

	paginatedCats := allCats[startIdx:endIdx]

	log.Infof("✅ gRPC: Returning %d cats (page %d, limit %d)", len(paginatedCats), page, limit)

	return &pb.ListCatsResponse{
		Cats:       paginatedCats,
		TotalCount: totalCount,
		Page:       page,
		Limit:      limit,
	}, nil
}

// GetCat implements the gRPC GetCat RPC
func (s *CatGrpcService) GetCat(ctx context.Context, req *pb.GetCatRequest) (*pb.GetCatResponse, error) {
	log.Infof("gRPC: GetCat called (catId=%d)", req.CatId)

	s.catMutex.RLock()
	defer s.catMutex.RUnlock()

	cat, found := s.cats[req.CatId]
	if !found {
		log.Errorf("❌ gRPC: Cat %d not found", req.CatId)
		return nil, status.Errorf(codes.NotFound, "Cat with ID %d not found", req.CatId)
	}

	log.Infof("✅ gRPC: Returning cat %d", req.CatId)

	return &pb.GetCatResponse{
		Cat: cat,
	}, nil
}

// CreateCat implements the gRPC CreateCat RPC
func (s *CatGrpcService) CreateCat(ctx context.Context, req *pb.CreateCatRequest) (*pb.CreateCatResponse, error) {
	log.Infof("gRPC: CreateCat called (name=%s)", req.Cat.CatName)

	s.catMutex.Lock()
	defer s.catMutex.Unlock()

	// Generate new ID
	newID := int32(len(s.cats) + 1)
	req.Cat.CatId = newID

	s.cats[newID] = req.Cat

	log.Infof("✅ gRPC: Created cat %d", newID)

	return &pb.CreateCatResponse{
		Cat: req.Cat,
	}, nil
}

// UpdateCat implements the gRPC UpdateCat RPC
func (s *CatGrpcService) UpdateCat(ctx context.Context, req *pb.UpdateCatRequest) (*pb.UpdateCatResponse, error) {
	log.Infof("gRPC: UpdateCat called (catId=%d)", req.CatId)

	s.catMutex.Lock()
	defer s.catMutex.Unlock()

	_, found := s.cats[req.CatId]
	if !found {
		log.Errorf("❌ gRPC: Cat %d not found", req.CatId)
		return nil, status.Errorf(codes.NotFound, "Cat with ID %d not found", req.CatId)
	}

	// Update the cat
	req.Cat.CatId = req.CatId
	s.cats[req.CatId] = req.Cat

	log.Infof("✅ gRPC: Updated cat %d", req.CatId)

	return &pb.UpdateCatResponse{
		Cat: req.Cat,
	}, nil
}

// DeleteCat implements the gRPC DeleteCat RPC
func (s *CatGrpcService) DeleteCat(ctx context.Context, req *pb.DeleteCatRequest) (*pb.DeleteCatResponse, error) {
	log.Infof("gRPC: DeleteCat called (catId=%d)", req.CatId)

	s.catMutex.Lock()
	defer s.catMutex.Unlock()

	_, found := s.cats[req.CatId]
	if !found {
		log.Errorf("❌ gRPC: Cat %d not found", req.CatId)
		return nil, status.Errorf(codes.NotFound, "Cat with ID %d not found", req.CatId)
	}

	delete(s.cats, req.CatId)

	log.Infof("✅ gRPC: Deleted cat %d", req.CatId)

	return &pb.DeleteCatResponse{
		Success: true,
		Message: fmt.Sprintf("Cat %d deleted successfully", req.CatId),
	}, nil
}

// GetCatAsync implements the gRPC GetCatAsync RPC (returns task ID immediately)
func (s *CatGrpcService) GetCatAsync(ctx context.Context, req *pb.GetCatAsyncRequest) (*pb.GetCatAsyncResponse, error) {
	log.Infof("gRPC: GetCatAsync called (catId=%d)", req.CatId)

	// Create task ID
	taskID := uuid.New().String()

	// Store task in global state
	global.Mutex.Lock()
	global.Tasks[taskID] = &global.Task{
		TaskId:             taskID,
		PercentageComplete: 0,
		Status:             "PENDING",
		Message:            fmt.Sprintf("Fetching cat %d asynchronously via gRPC", req.CatId),
	}
	global.Mutex.Unlock()

	// Start async processing
	go s.processGetCatAsync(taskID, req.CatId)

	log.Infof("✅ gRPC: Created async task %s", taskID)

	return &pb.GetCatAsyncResponse{
		TaskId:  taskID,
		PollUrl: fmt.Sprintf("http://%s:%s/tasks/%s", global.TaskServerHost, global.TaskServerPort, taskID),
		Message: fmt.Sprintf("Poll Task Server for task %s", taskID),
	}, nil
}

// Process async task (simulates long-running operation)
func (s *CatGrpcService) processGetCatAsync(taskID string, catID int32) {
	log.Infof("gRPC: Starting async processing for task %s (cat ID: %d)", taskID, catID)

	// Simulate work phases
	// Phase 1
	// time.Sleep(3 * time.Second)
	// updateTaskProgress(taskID, 33, "Fetching data from source 1")

	// Phase 2
	// time.Sleep(3 * time.Second)
	// updateTaskProgress(taskID, 67, "Processing data")

	// Phase 3 - Complete
	s.catMutex.RLock()
	_, found := s.cats[catID]
	s.catMutex.RUnlock()

	global.Mutex.Lock()
	if task, exists := global.Tasks[taskID]; exists {
		if found {
			task.PercentageComplete = 100
			task.Status = "COMPLETED"
			task.Message = "Cat retrieved successfully via gRPC"
		} else {
			task.PercentageComplete = 100
			task.Status = "FAILED"
			task.Message = fmt.Sprintf("Cat %d not found", catID)
		}
	}
	global.Mutex.Unlock()

	log.Infof("✅ gRPC: Completed async processing for task %s", taskID)
}
