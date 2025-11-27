/*
 * gRPC Service Implementation for Item Service
 * Following Nutanix patterns from ntnx-api-guru
 */

package grpc

import (
	"context"
	"fmt"
	"strings"

	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/db"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/models"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/utils/query"
	responseUtils "github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/utils/response"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ItemGrpcService implements the gRPC ItemService
type ItemGrpcService struct {
	pb.UnimplementedItemServiceServer
	itemRepository db.ItemRepository
}

// NewItemGrpcService creates a new gRPC Item service with IDF repository
func NewItemGrpcService(itemRepository db.ItemRepository) *ItemGrpcService {
	service := &ItemGrpcService{
		itemRepository: itemRepository,
	}
	log.Info("✅ Initialized gRPC Item Service with IDF repository")
	return service
}

// ListItems implements the gRPC ListItems RPC
func (s *ItemGrpcService) ListItems(ctx context.Context, req *pb.ListItemsArg) (*pb.ListItemsRet, error) {
	log.Infof("gRPC: ListItems called")
	log.Debugf("gRPC: ListItems request details: %+v", req)

	// Extract query parameters from context (OData params from HTTP request)
	queryParams := query.ExtractQueryParamsFromContext(ctx)

	// Call repository to fetch items from IDF (with OData parsing)
	items, totalCount, err := s.itemRepository.ListItems(queryParams)
	if err != nil {
		log.Errorf("❌ Failed to list items from IDF: %v", err)
		// Handle OData parsing errors with user-friendly messages
		return nil, handleODataError(err, queryParams)
	}

	log.Debugf("gRPC: Retrieved %d items from IDF (total: %d)", len(items), totalCount)

	// Determine if paginated
	isPaginated := queryParams.Page > 0 || queryParams.Limit > 0

	// Get pagination links
	apiUrl := responseUtils.GetApiUrl(ctx, queryParams.Filter, queryParams.Expand, queryParams.Orderby, &queryParams.Limit, &queryParams.Page)
	paginationLinks := responseUtils.GetPaginationLinks(totalCount, apiUrl)

	// Create response with metadata
	response := responseUtils.CreateListItemsResponse(items, paginationLinks, isPaginated, int32(totalCount))

	log.Infof("✅ gRPC: Returning %d items with metadata (total: %d)", len(items), totalCount)
	if log.GetLevel() == log.DebugLevel {
		log.Debugf("gRPC: Metadata: %+v", response.Content.Metadata)
	}

	return response, nil
}

// handleODataError handles OData parsing errors and converts them to appropriate gRPC status errors
func handleODataError(err error, queryParams *models.QueryParams) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Check if it's already a gRPC status error
	if st, ok := status.FromError(err); ok {
		return st.Err()
	}

	// Check for OData parsing errors
	if strings.Contains(errStr, "invalid OData query") || strings.Contains(errStr, "parse") {
		queryParam := ""
		if queryParams.Filter != "" {
			queryParam = fmt.Sprintf("$filter=%s", queryParams.Filter)
		} else if queryParams.Orderby != "" {
			queryParam = fmt.Sprintf("$orderby=%s", queryParams.Orderby)
		} else if queryParams.Select != "" {
			queryParam = fmt.Sprintf("$select=%s", queryParams.Select)
		}

		return status.Errorf(codes.InvalidArgument,
			"Invalid OData query syntax in '%s': %v. Please check your expression.",
			queryParam, err)
	}

	if strings.Contains(errStr, "property") && strings.Contains(errStr, "not found") {
		return status.Errorf(codes.InvalidArgument,
			"Unknown property in OData query: %v. Please check field names (itemId, itemName, itemType, extId).",
			err)
	}

	if strings.Contains(errStr, "operator") || strings.Contains(errStr, "unsupported") {
		return status.Errorf(codes.InvalidArgument,
			"Unsupported operator in OData query: %v. Please check your expression.",
			err)
	}

	if strings.Contains(errStr, "evaluate") || strings.Contains(errStr, "evaluation") {
		return status.Errorf(codes.Internal,
			"Failed to evaluate OData query: %v. Please try a simpler query.",
			err)
	}

	// Generic error for unknown errors
	return status.Errorf(codes.Internal,
		"Failed to process query: %v", err)
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
