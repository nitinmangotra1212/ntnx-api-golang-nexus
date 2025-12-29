/*
 * gRPC Service Implementation for CatStats Service
 * This is a SEPARATE service from CatService, required because Java Adonis
 * generates a separate CatStatsService from the ApiEndpoint(CatStats) tag
 */

package grpc

import (
	"context"

	"github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/response"
	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/idf"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/utils/odata"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/utils/query"
	responseUtils "github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/utils/response"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

// CatStatsGrpcService implements the separate CatStatsService
type CatStatsGrpcService struct {
	pb.UnimplementedCatStatsServiceServer
	catRepository *idf.CatRepositoryImpl
}

// NewCatStatsGrpcService creates a new gRPC CatStats service
func NewCatStatsGrpcService() *CatStatsGrpcService {
	service := &CatStatsGrpcService{
		catRepository: &idf.CatRepositoryImpl{},
	}
	log.Info("✅ Initialized gRPC CatStats Service with IDF repository")
	return service
}

// ListCatStats implements the gRPC ListCatStats RPC for CatStatsService
func (s *CatStatsGrpcService) ListCatStats(ctx context.Context, req *pb.ListCatStatsArg) (*pb.ListCatStatsRet, error) {
	log.Infof("gRPC: CatStatsService.ListCatStats called")

	// Extract query parameters from context
	queryParams := query.ExtractQueryParamsFromContext(ctx)
	log.Infof("📥 Extracted query params: Filter=%s, Orderby=%s, Page=%d, Limit=%d",
		queryParams.Filter, queryParams.Orderby, queryParams.Page, queryParams.Limit)

	// Call repository to fetch cat stats from IDF
	stats, totalCount, err := s.catRepository.ListCatStats(ctx, queryParams)
	if err != nil {
		log.Errorf("❌ Failed to list cat stats from IDF: %v", err)
		return nil, odata.HandleODataError(err, queryParams)
	}

	log.Debugf("gRPC: Retrieved %d cat stats from IDF (total: %d)", len(stats), totalCount)

	// Determine if paginated
	isPaginated := queryParams.Page > 0 || queryParams.Limit > 0

	// Get pagination links
	apiUrl := responseUtils.GetApiUrl(ctx, queryParams.Filter, "", queryParams.Orderby, &queryParams.Limit, &queryParams.Page)
	paginationLinks := responseUtils.GetPaginationLinks(totalCount, apiUrl)

	// Create response with metadata
	response := createCatStatsServiceResponse(stats, paginationLinks, isPaginated, int32(totalCount))

	log.Infof("✅ gRPC: CatStatsService returning %d cat stats with metadata (total: %d)", len(stats), totalCount)
	return response, nil
}

// createCatStatsServiceResponse creates a response for ListCatStats API with metadata
func createCatStatsServiceResponse(stats []*pb.CatStats, paginationLinks []*response.ApiLink, isPaginated bool, totalAvailableResults int32) *pb.ListCatStatsRet {
	resp := &pb.ListCatStatsRet{
		Content: &pb.ListCatStatsApiResponse{
			Data: &pb.ListCatStatsApiResponse_CatStatsArrayData{
				CatStatsArrayData: &pb.CatStatsListArrayWrapper{
					Value: stats,
				},
			},
			Metadata: responseUtils.CreateResponseMetadata(false, isPaginated, paginationLinks, "", ""),
		},
	}
	resp.Content.Metadata.TotalAvailableResults = proto.Int32(totalAvailableResults)
	return resp
}

