/*
 * gRPC Service Implementation for Cat Service
 * Handles Cat, CatStats, PetFood, PetCare entities
 * Following Nutanix patterns from ntnx-api-guru
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

// CatGrpcService implements the gRPC CatService
type CatGrpcService struct {
	pb.UnimplementedCatServiceServer
	catRepository idf.CatRepository
}

// NewCatGrpcService creates a new gRPC Cat service with IDF repository
func NewCatGrpcService() *CatGrpcService {
	service := &CatGrpcService{
		catRepository: idf.NewCatRepository(),
	}
	log.Info("✅ Initialized gRPC Cat Service with IDF repository")
	return service
}

// ListCats implements the gRPC ListCats RPC
func (s *CatGrpcService) ListCats(ctx context.Context, req *pb.ListCatsArg) (*pb.ListCatsRet, error) {
	log.Infof("gRPC: ListCats called")
	log.Debugf("gRPC: ListCats request details: %+v", req)

	// Extract query parameters from context (OData params from HTTP request)
	queryParams := query.ExtractQueryParamsFromContext(ctx)
	log.Infof("📥 Extracted query params: Expand=%s, Filter=%s, Orderby=%s, Page=%d, Limit=%d",
		queryParams.Expand, queryParams.Filter, queryParams.Orderby, queryParams.Page, queryParams.Limit)

	// Call repository to fetch cats from IDF (with OData parsing)
	cats, totalCount, err := s.catRepository.ListCats(queryParams)
	if err != nil {
		log.Errorf("❌ Failed to list cats from IDF: %v", err)
		return nil, odata.HandleODataError(err, queryParams)
	}

	log.Debugf("gRPC: Retrieved %d cats from IDF (total: %d)", len(cats), totalCount)

	// Determine if paginated
	isPaginated := queryParams.Page > 0 || queryParams.Limit > 0

	// Get pagination links
	apiUrl := responseUtils.GetApiUrl(ctx, queryParams.Filter, queryParams.Expand, queryParams.Orderby, &queryParams.Limit, &queryParams.Page)
	paginationLinks := responseUtils.GetPaginationLinks(totalCount, apiUrl)

	// Create response with metadata
	response := createListCatsResponse(cats, paginationLinks, isPaginated, int32(totalCount))

	log.Infof("✅ gRPC: Returning %d cats with metadata (total: %d)", len(cats), totalCount)
	if log.GetLevel() == log.DebugLevel {
		log.Debugf("gRPC: Metadata: %+v", response.Content.Metadata)
	}

	return response, nil
}

// GetCatById implements the gRPC GetCatById RPC
func (s *CatGrpcService) GetCatById(ctx context.Context, req *pb.GetCatByIdArg) (*pb.GetCatByIdRet, error) {
	log.Infof("gRPC: GetCatById called (extId=%s)", req.GetExtId())

	cat, err := s.catRepository.GetCatById(req.GetExtId())
	if err != nil {
		log.Errorf("❌ Failed to get cat from IDF: %v", err)
		return nil, err
	}

	// Create response
	response := &pb.GetCatByIdRet{
		Content: &pb.GetCatApiResponse{
			Data: &pb.GetCatApiResponse_CatData{
				CatData: cat,
			},
			Metadata: responseUtils.CreateResponseMetadata(false, false, nil, "", ""),
		},
	}

	log.Infof("✅ gRPC: Returning cat with extId=%s", req.GetExtId())
	return response, nil
}

// ListCatStats implements the gRPC ListCatStats RPC
func (s *CatGrpcService) ListCatStats(ctx context.Context, req *pb.ListCatStatsArg) (*pb.ListCatStatsRet, error) {
	log.Infof("gRPC: ListCatStats called")

	// Extract query parameters from context
	queryParams := query.ExtractQueryParamsFromContext(ctx)
	log.Infof("📥 Extracted query params: Filter=%s, Orderby=%s, Page=%d, Limit=%d",
		queryParams.Filter, queryParams.Orderby, queryParams.Page, queryParams.Limit)

	// Get the repository as CatRepositoryImpl to access ListCatStats method
	catRepo, ok := s.catRepository.(*idf.CatRepositoryImpl)
	if !ok {
		log.Errorf("❌ Failed to cast repository to CatRepositoryImpl")
		catRepo = &idf.CatRepositoryImpl{}
	}

	// Call repository to fetch cat stats from IDF
	stats, totalCount, err := catRepo.ListCatStats(ctx, queryParams)
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
	response := createListCatStatsResponse(stats, paginationLinks, isPaginated, int32(totalCount))

	log.Infof("✅ gRPC: Returning %d cat stats with metadata (total: %d)", len(stats), totalCount)
	return response, nil
}

// createListCatsResponse creates a response for ListCats API with metadata
func createListCatsResponse(cats []*pb.Cat, paginationLinks []*response.ApiLink, isPaginated bool, totalAvailableResults int32) *pb.ListCatsRet {
	resp := &pb.ListCatsRet{
		Content: &pb.ListCatsApiResponse{
			Data: &pb.ListCatsApiResponse_CatArrayData{
				CatArrayData: &pb.CatArrayWrapper{
					Value: cats,
				},
			},
			Metadata: responseUtils.CreateResponseMetadata(false, isPaginated, paginationLinks, "", ""),
		},
	}
	resp.Content.Metadata.TotalAvailableResults = proto.Int32(totalAvailableResults)
	return resp
}

// createListCatStatsResponse creates a response for ListCatStats API with metadata
func createListCatStatsResponse(stats []*pb.CatStats, paginationLinks []*response.ApiLink, isPaginated bool, totalAvailableResults int32) *pb.ListCatStatsRet {
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

