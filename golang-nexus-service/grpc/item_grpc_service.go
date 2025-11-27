/*
 * gRPC Service Implementation for Item Service
 * Following Nutanix patterns from ntnx-api-guru
 */

package grpc

import (
	"context"
	"fmt"
	"sync"

	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config"
	responseUtils "github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/utils/response"
	log "github.com/sirupsen/logrus"
)

// ItemGrpcService implements the gRPC ItemService
type ItemGrpcService struct {
	pb.UnimplementedItemServiceServer
	itemMutex sync.RWMutex
	items     map[int32]*pb.Item
}

// NewItemGrpcService creates a new gRPC Item service
func NewItemGrpcService() *ItemGrpcService {
	service := &ItemGrpcService{
		items: make(map[int32]*pb.Item),
	}

	// Initialize with mock data
	service.initializeMockItems()

	return service
}

// Initialize mock items
func (s *ItemGrpcService) initializeMockItems() {
	log.Info("🎯 Initializing gRPC Item Service with mock data")

	s.itemMutex.Lock()
	defer s.itemMutex.Unlock()

	for i := int32(1); i <= 100; i++ {
		itemName := fmt.Sprintf("Item-%d", i)
		itemType := "TYPE1"
		description := "A fluffy item"
		item := &pb.Item{
			ItemId:      &i,
			ItemName:    &itemName,
			ItemType:    &itemType,
			Description: &description,
		}

		// Add location for even numbered items
		if i%2 == 0 {
			city := "San Francisco"
			state := "California"
			item.Location = &pb.Location{
				City: &city,
				Country: &pb.Country{
					State: &state,
				},
			}
		}

		s.items[i] = item
	}

	log.Infof("✅ Initialized %d items in gRPC service", len(s.items))
}

// ListItems implements the gRPC ListItems RPC
func (s *ItemGrpcService) ListItems(ctx context.Context, req *pb.ListItemsArg) (*pb.ListItemsRet, error) {
	log.Infof("gRPC: ListItems called")
	log.Debugf("gRPC: ListItems request details: %+v", req)

	s.itemMutex.RLock()
	defer s.itemMutex.RUnlock()

	log.Debugf("gRPC: Total items in memory: %d", len(s.items))

	// Collect all items
	allItems := make([]*pb.Item, 0, len(s.items))
	for _, item := range s.items {
		allItems = append(allItems, item)
	}

	totalCount := int32(len(allItems))

	// Determine if paginated (for now, always false since we don't have page/limit in ListItemsArg yet)
	isPaginated := false

	// Get pagination links (even if not paginated, we still want self link)
	apiUrl := responseUtils.GetApiUrl(ctx, "", "", "", nil, nil)
	paginationLinks := responseUtils.GetPaginationLinks(int64(totalCount), apiUrl)

	// Create response with metadata
	response := responseUtils.CreateListItemsResponse(allItems, paginationLinks, isPaginated, totalCount)

	log.Infof("✅ gRPC: Returning %d items with metadata", totalCount)
	if log.GetLevel() == log.DebugLevel {
		log.Debugf("gRPC: Returning items: %+v", allItems)
		log.Debugf("gRPC: Metadata: %+v", response.Content.Metadata)
	}

	return response, nil
}

// NOTE: The following methods (GetItem, CreateItem, UpdateItem, DeleteItem, GetItemAsync) are not yet
// defined in the proto file. They are commented out until the proto definitions are added.
// Uncomment and update them once the corresponding RPCs are added to item_service.proto.

/*
// GetItem implements the gRPC GetItem RPC
func (s *ItemGrpcService) GetItem(ctx context.Context, req *pb.GetItemRequest) (*pb.GetItemResponse, error) {
	log.Infof("gRPC: GetItem called (itemId=%d)", req.ItemId)

	s.itemMutex.RLock()
	defer s.itemMutex.RUnlock()

	item, found := s.items[req.ItemId]
	if !found {
		log.Errorf("❌ gRPC: Item %d not found", req.ItemId)
		return nil, status.Errorf(codes.NotFound, "Item with ID %d not found", req.ItemId)
	}

	log.Infof("✅ gRPC: Returning item %d", req.ItemId)

	return &pb.GetItemResponse{
		Item: item,
	}, nil
}

// CreateItem implements the gRPC CreateItem RPC
func (s *ItemGrpcService) CreateItem(ctx context.Context, req *pb.CreateItemRequest) (*pb.CreateItemResponse, error) {
	log.Infof("gRPC: CreateItem called (name=%s)", req.Item.ItemName)

	s.itemMutex.Lock()
	defer s.itemMutex.Unlock()

	// Generate new ID
	newID := int32(len(s.items) + 1)
	req.Item.ItemId = newID

	s.items[newID] = req.Item

	log.Infof("✅ gRPC: Created item %d", newID)

	return &pb.CreateItemResponse{
		Item: req.Item,
	}, nil
}

// UpdateItem implements the gRPC UpdateItem RPC
func (s *ItemGrpcService) UpdateItem(ctx context.Context, req *pb.UpdateItemRequest) (*pb.UpdateItemResponse, error) {
	log.Infof("gRPC: UpdateItem called (itemId=%d)", req.ItemId)

	s.itemMutex.Lock()
	defer s.itemMutex.Unlock()

	_, found := s.items[req.ItemId]
	if !found {
		log.Errorf("❌ gRPC: Item %d not found", req.ItemId)
		return nil, status.Errorf(codes.NotFound, "Item with ID %d not found", req.ItemId)
	}

	// Update the item
	req.Item.ItemId = req.ItemId
	s.items[req.ItemId] = req.Item

	log.Infof("✅ gRPC: Updated item %d", req.ItemId)

	return &pb.UpdateItemResponse{
		Item: req.Item,
	}, nil
}

// DeleteItem implements the gRPC DeleteItem RPC
func (s *ItemGrpcService) DeleteItem(ctx context.Context, req *pb.DeleteItemRequest) (*pb.DeleteItemResponse, error) {
	log.Infof("gRPC: DeleteItem called (itemId=%d)", req.ItemId)

	s.itemMutex.Lock()
	defer s.itemMutex.Unlock()

	_, found := s.items[req.ItemId]
	if !found {
		log.Errorf("❌ gRPC: Item %d not found", req.ItemId)
		return nil, status.Errorf(codes.NotFound, "Item with ID %d not found", req.ItemId)
	}

	delete(s.items, req.ItemId)

	log.Infof("✅ gRPC: Deleted item %d", req.ItemId)

	return &pb.DeleteItemResponse{
		Success: true,
		Message: fmt.Sprintf("Item %d deleted successfully", req.ItemId),
	}, nil
}

// GetItemAsync implements the gRPC GetItemAsync RPC (returns task ID immediately)
func (s *ItemGrpcService) GetItemAsync(ctx context.Context, req *pb.GetItemAsyncRequest) (*pb.GetItemAsyncResponse, error) {
	log.Infof("gRPC: GetItemAsync called (itemId=%d)", req.ItemId)

	// Create task ID
	taskID := uuid.New().String()

	// Store task in global state
	global.Mutex.Lock()
	global.Tasks[taskID] = &global.Task{
		TaskId:             taskID,
		PercentageComplete: 0,
		Status:             "PENDING",
		Message:            fmt.Sprintf("Fetching item %d asynchronously via gRPC", req.ItemId),
	}
	global.Mutex.Unlock()

	// Start async processing
	go s.processGetItemAsync(taskID, req.ItemId)

	log.Infof("✅ gRPC: Created async task %s", taskID)

	return &pb.GetItemAsyncResponse{
		TaskId:  taskID,
		PollUrl: fmt.Sprintf("http://%s:%s/tasks/%s", global.TaskServerHost, global.TaskServerPort, taskID),
		Message: fmt.Sprintf("Poll Task Server for task %s", taskID),
	}, nil
}
*/

// Process async task (simulates long-running operation)
// NOTE: This function is commented out along with GetItemAsync since it's not in the proto
/*
func (s *ItemGrpcService) processGetItemAsync(taskID string, itemID int32) {
	log.Infof("gRPC: Starting async processing for task %s (item ID: %d)", taskID, itemID)

	// Simulate work phases
	// Phase 1
	// time.Sleep(3 * time.Second)
	// updateTaskProgress(taskID, 33, "Fetching data from source 1")

	// Phase 2
	// time.Sleep(3 * time.Second)
	// updateTaskProgress(taskID, 67, "Processing data")

	// Phase 3 - Complete
	s.itemMutex.RLock()
	_, found := s.items[itemID]
	s.itemMutex.RUnlock()

	global.Mutex.Lock()
	if task, exists := global.Tasks[taskID]; exists {
		if found {
			task.PercentageComplete = 100
			task.Status = "COMPLETED"
			task.Message = "Item retrieved successfully via gRPC"
		} else {
			task.PercentageComplete = 100
			task.Status = "FAILED"
			task.Message = fmt.Sprintf("Item %d not found", itemID)
		}
	}
	global.Mutex.Unlock()

	log.Infof("✅ gRPC: Completed async processing for task %s", taskID)
}
*/
