/*
 * gRPC Service Implementation for ItemStats Service (Stats Module)
 * Following Nutanix patterns from ntnx-api-guru
 */

package grpc

import (
	"context"

	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/stats" // Note: stats protobuf
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/db"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/utils/odata"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/utils/query"
	responseUtils "github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/utils/response"
	log "github.com/sirupsen/logrus"
)

// ItemStatsGrpcService implements the gRPC ItemStatsService
type ItemStatsGrpcService struct {
	pb.UnimplementedItemStatsServiceServer
	statsRepository db.ItemStatsRepository
}

// NewItemStatsGrpcService creates a new gRPC ItemStats service with IDF repository
func NewItemStatsGrpcService(statsRepository db.ItemStatsRepository) *ItemStatsGrpcService {
	service := &ItemStatsGrpcService{
		statsRepository: statsRepository,
	}
	log.Info("✅ Initialized gRPC ItemStats Service with IDF repository")
	return service
}

// ListItemStats handles GET /api/nexus/v4.1/stats/item-stats requests
func (s *ItemStatsGrpcService) ListItemStats(ctx context.Context, req *pb.ListItemStatsArg) (*pb.ListItemStatsRet, error) {
	log.Infof("gRPC: ListItemStats called")

	// Extract query parameters from request
	queryParams := query.ExtractQueryParamsFromContext(ctx)

	// Check if this is a GroupBy query
	if queryParams.Apply != "" {
		log.Infof("🔀 GroupBy query detected for stats module: $apply=%s", queryParams.Apply)
		itemGroups, totalCount, err := s.statsRepository.ListItemStatsWithGroupBy(queryParams)
		if err != nil {
			log.Errorf("❌ Failed to list item stats with GroupBy from IDF: %v", err)
			return nil, odata.HandleODataError(err, queryParams)
		}

		// Determine if paginated
		isPaginated := queryParams.Page > 0 || queryParams.Limit > 0

		// Get pagination links
		apiUrl := responseUtils.GetApiUrl(ctx, queryParams.Filter, queryParams.Expand, queryParams.Orderby, &queryParams.Limit, &queryParams.Page)
		paginationLinks := responseUtils.GetPaginationLinks(totalCount, apiUrl)

		// Create response with ItemStatsGroup array
		response := responseUtils.CreateListItemStatsGroupResponse(itemGroups, paginationLinks, isPaginated, int32(totalCount))
		return response, nil
	} else {
		// Regular query (no GroupBy)
		stats, totalCount, err := s.statsRepository.ListItemStats(queryParams)
		if err != nil {
			log.Errorf("❌ Failed to list item stats from IDF: %v", err)
			return nil, odata.HandleODataError(err, queryParams)
		}

		// Determine if paginated
		isPaginated := queryParams.Page > 0 || queryParams.Limit > 0

		// Get pagination links
		apiUrl := responseUtils.GetApiUrl(ctx, queryParams.Filter, queryParams.Expand, queryParams.Orderby, &queryParams.Limit, &queryParams.Page)
		paginationLinks := responseUtils.GetPaginationLinks(totalCount, apiUrl)

		// Create response with ItemStats array
		response := responseUtils.CreateListItemStatsResponse(stats, paginationLinks, isPaginated, int32(totalCount))
		return response, nil
	}
}

