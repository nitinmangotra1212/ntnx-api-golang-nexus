/*
 * gRPC Service Implementation for Item Service
 * Following Nutanix patterns from ntnx-api-guru
 */

package grpc

import (
	"context"

	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/db"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/utils/odata"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/utils/query"
	responseUtils "github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/utils/response"
	log "github.com/sirupsen/logrus"
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
	log.Infof("📥 Extracted query params: Expand=%s, Filter=%s, Orderby=%s, Page=%d, Limit=%d, Apply=%s",
		queryParams.Expand, queryParams.Filter, queryParams.Orderby, queryParams.Page, queryParams.Limit, queryParams.Apply)

	// Handle GroupBy queries separately (they return ItemGroup objects)
	if queryParams.Apply != "" {
		log.Infof("🔀 GroupBy query detected: $apply=%s", queryParams.Apply)
		itemGroups, totalCount, err := s.itemRepository.ListItemsWithGroupBy(queryParams)
		if err != nil {
			log.Errorf("❌ Failed to list items with GroupBy from IDF: %v", err)
			return nil, odata.HandleODataError(err, queryParams)
		}

		log.Debugf("gRPC: Retrieved %d ItemGroups from IDF (total: %d)", len(itemGroups), totalCount)

		// Determine if paginated
		isPaginated := queryParams.Page > 0 || queryParams.Limit > 0

		// Get pagination links
		apiUrl := responseUtils.GetApiUrl(ctx, queryParams.Filter, queryParams.Expand, queryParams.Orderby, &queryParams.Limit, &queryParams.Page)
		paginationLinks := responseUtils.GetPaginationLinks(totalCount, apiUrl)

		// Create response with ItemGroup data
		response := responseUtils.CreateListItemsGroupResponse(itemGroups, paginationLinks, isPaginated, int32(totalCount))

		log.Infof("✅ gRPC: Returning %d ItemGroups with metadata (total: %d)", len(itemGroups), totalCount)
		return response, nil
	}

	// Regular query (no GroupBy)
	items, totalCount, err := s.itemRepository.ListItems(queryParams)
	if err != nil {
		log.Errorf("❌ Failed to list items from IDF: %v", err)
		// Handle OData parsing errors with AppMessage format
		return nil, odata.HandleODataError(err, queryParams)
	}

	log.Debugf("gRPC: Retrieved %d items from IDF (total: %d)", len(items), totalCount)

	// Debug: Check if itemStats and list fields are set before creating response
	itemStatsCount := 0
	listFieldsCount := 0
	for i, item := range items {
		if item.ItemStats != nil {
			itemStatsCount++
			if i < 3 { // Log first 3 items with itemStats for debugging
				log.Infof("🔍 [gRPC DEBUG] Item[%d] extId=%s has itemStats: statsExtId=%v, age=%v, heartRate=%v, foodIntake=%v",
					i, item.GetExtId(), item.ItemStats.GetStatsExtId(), item.ItemStats.GetAge(), item.ItemStats.GetHeartRate(), item.ItemStats.GetFoodIntake())
			}
		}

		// Check for list fields (only int64List now)
		hasListFields := false
		if item.Int64List != nil && len(item.Int64List.Value) > 0 {
			hasListFields = true
			if i < 3 {
				log.Infof("🔍 [gRPC DEBUG] Item[%d] extId=%s has Int64List: %v", i, item.GetExtId(), item.Int64List.Value)
			}
		}
		if hasListFields {
			listFieldsCount++
		}
	}
	log.Infof("🔍 [gRPC DEBUG] %d out of %d items have itemStats set before response creation", itemStatsCount, len(items))
	log.Infof("🔍 [gRPC DEBUG] %d out of %d items have list fields set before response creation", listFieldsCount, len(items))

	// Determine if paginated
	isPaginated := queryParams.Page > 0 || queryParams.Limit > 0

	// Get pagination links
	apiUrl := responseUtils.GetApiUrl(ctx, queryParams.Filter, queryParams.Expand, queryParams.Orderby, &queryParams.Limit, &queryParams.Page)
	paginationLinks := responseUtils.GetPaginationLinks(totalCount, apiUrl)

	// Create response with metadata
	response := responseUtils.CreateListItemsResponse(items, paginationLinks, isPaginated, int32(totalCount))

	// Debug: Verify itemStats and list fields are still in response protobuf
	if response != nil && response.Content != nil && response.Content.Data != nil {
		if itemArrayData, ok := response.Content.Data.(*pb.ListItemsApiResponse_ItemArrayData); ok && itemArrayData.ItemArrayData != nil {
			itemStatsCountAfter := 0
			listFieldsCountAfter := 0
			for i, item := range itemArrayData.ItemArrayData.Value {
				if item.ItemStats != nil {
					itemStatsCountAfter++
					if i < 3 { // Log first 3 items with itemStats for debugging
						log.Infof("🔍 [gRPC DEBUG] Response Item[%d] extId=%s has itemStats: statsExtId=%v",
							i, item.GetExtId(), item.ItemStats.GetStatsExtId())
						// Check if time-value pair arrays are set
						if item.ItemStats.GetAge() != nil && len(item.ItemStats.GetAge().GetValue()) > 0 {
							log.Infof("  ✅ age has %d time-value pairs", len(item.ItemStats.GetAge().GetValue()))
						} else {
							log.Warnf("  ⚠️  age is nil or empty")
						}
						if item.ItemStats.GetHeartRate() != nil && len(item.ItemStats.GetHeartRate().GetValue()) > 0 {
							log.Infof("  ✅ heartRate has %d time-value pairs", len(item.ItemStats.GetHeartRate().GetValue()))
						} else {
							log.Warnf("  ⚠️  heartRate is nil or empty")
						}
						if item.ItemStats.GetFoodIntake() != nil && len(item.ItemStats.GetFoodIntake().GetValue()) > 0 {
							log.Infof("  ✅ foodIntake has %d time-value pairs", len(item.ItemStats.GetFoodIntake().GetValue()))
						} else {
							log.Warnf("  ⚠️  foodIntake is nil or empty")
						}
					}
				}

				// Check for list fields in response protobuf (only int64List now)
				hasListFields := false
				if item.Int64List != nil && len(item.Int64List.Value) > 0 {
					hasListFields = true
					if i < 3 {
						log.Infof("🔍 [gRPC DEBUG] Response Item[%d] extId=%s has Int64List in protobuf: %v", i, item.GetExtId(), item.Int64List.Value)
					}
				}
				if hasListFields {
					listFieldsCountAfter++
				}
			}
			log.Infof("🔍 [gRPC DEBUG] %d out of %d items have itemStats set in response protobuf", itemStatsCountAfter, len(itemArrayData.ItemArrayData.Value))
			log.Infof("🔍 [gRPC DEBUG] %d out of %d items have list fields set in response protobuf", listFieldsCountAfter, len(itemArrayData.ItemArrayData.Value))
		}
	}

	log.Infof("✅ gRPC: Returning %d items with metadata (total: %d)", len(items), totalCount)
	if log.GetLevel() == log.DebugLevel {
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
