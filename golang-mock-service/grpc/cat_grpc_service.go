/*
 * gRPC Service Implementation for Cat Service
 * Following Nutanix patterns from ntnx-api-guru
 */

package grpc

import (
	"context"
	"fmt"
	"sync"

	pb "github.com/nutanix/ntnx-api-golang-mock-pc/generated-code/protobuf/mock/v4/config"
	log "github.com/sirupsen/logrus"
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
		catName := fmt.Sprintf("Cat-%d", i)
		catType := "TYPE1"
		description := "A fluffy cat"
		cat := &pb.Cat{
			CatId:       &i,
			CatName:     &catName,
			CatType:     &catType,
			Description: &description,
		}

		// Add location for even numbered cats
		if i%2 == 0 {
			city := "San Francisco"
			state := "California"
			cat.Location = &pb.Location{
				City: &city,
				Country: &pb.Country{
					State: &state,
				},
			}
		}

		s.cats[i] = cat
	}

	log.Infof("✅ Initialized %d cats in gRPC service", len(s.cats))
}

// ListCats implements the gRPC ListCats RPC
func (s *CatGrpcService) ListCats(ctx context.Context, req *pb.ListCatsArg) (*pb.ListCatsRet, error) {
	log.Infof("gRPC: ListCats called")
	log.Debugf("gRPC: ListCats request details: %+v", req)

	s.catMutex.RLock()
	defer s.catMutex.RUnlock()

	log.Debugf("gRPC: Total cats in memory: %d", len(s.cats))

	// Collect all cats
	allCats := make([]*pb.Cat, 0, len(s.cats))
	for _, cat := range s.cats {
		allCats = append(allCats, cat)
	}

	// Create CatArrayWrapper with all cats
	catArrayWrapper := &pb.CatArrayWrapper{
		Value: allCats,
	}

	// Create ListCatsApiResponse with CatArrayData
	apiResponse := &pb.ListCatsApiResponse{
		Data: &pb.ListCatsApiResponse_CatArrayData{
			CatArrayData: catArrayWrapper,
		},
	}

	log.Infof("✅ gRPC: Returning %d cats", len(allCats))
	if log.GetLevel() == log.DebugLevel {
		log.Debugf("gRPC: Returning cats: %+v", allCats)
	}

	// Return ListCatsRet with Content
	return &pb.ListCatsRet{
		Content: apiResponse,
	}, nil
}

// NOTE: The following methods (GetCat, CreateCat, UpdateCat, DeleteCat, GetCatAsync) are not yet
// defined in the proto file. They are commented out until the proto definitions are added.
// Uncomment and update them once the corresponding RPCs are added to cat_service.proto.

/*
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
*/

// Process async task (simulates long-running operation)
// NOTE: This function is commented out along with GetCatAsync since it's not in the proto
/*
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
*/
