/*
 * Copyright (c) 2025 Nutanix Inc. All rights reserved.
 */

package response

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/nutanix-core/ntnx-api-utils-go/responseutils"
	commonConfig "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/config"
	"github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/response"
	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config"
	pbStats "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/stats"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

const (
	HasError          = "hasError"
	IsPaginated       = "isPaginated"
	EnvoyOriginalPath = "x-envoy-original-path" // Header used by Envoy/Adonis for original path
)

// CreateResponseMetadata creates metadata with flags and links
func CreateResponseMetadata(hasError bool, isPaginated bool, paginationLinks []*response.ApiLink, url string, rel string) *response.ApiResponseMetadata {
	links := &response.ApiLinkArrayWrapper{
		Value: paginationLinks,
	}

	if url != "" {
		links = AddToHateOASLinks(links, url, rel)
	}
	return &response.ApiResponseMetadata{
		Flags: CreateMetadataFlags(hasError, isPaginated),
		Links: links,
	}
}

// AddToHateOASLinks adds a link to the HATEOAS links, if the linksWrapper is nil, it creates a new one
func AddToHateOASLinks(linksWrapper *response.ApiLinkArrayWrapper, url string,
	rel string) *response.ApiLinkArrayWrapper {

	if linksWrapper == nil {
		// Create a new link wrapper
		return &response.ApiLinkArrayWrapper{
			Value: []*response.ApiLink{
				{
					Href: proto.String(url),
					Rel:  proto.String(rel),
				},
			},
		}
	}

	// Append the new link to the existing link wrapper
	linksWrapper.Value = append(linksWrapper.Value, &response.ApiLink{
		Href: proto.String(url),
		Rel:  proto.String(rel),
	})
	return linksWrapper
}

// CreateMetadataFlags creates an array of metadata flags
func CreateMetadataFlags(hasError bool, isPaginated bool) *commonConfig.FlagArrayWrapper {
	return &commonConfig.FlagArrayWrapper{
		Value: []*commonConfig.Flag{
			{
				Name:  proto.String(HasError),
				Value: proto.Bool(hasError),
			},
			{
				Name:  proto.String(IsPaginated),
				Value: proto.Bool(isPaginated),
			},
		},
	}
}

// GetApiUrl constructs the API URL from context and query parameters
func GetApiUrl(ctx context.Context, filter, expand, orderby string, limit, page *int32) string {
	apiUrl := *GetSelfLink(ctx).Href + "?"
	if limit == nil {
		limit = proto.Int32(50) // Default limit
	}
	if page == nil {
		page = proto.Int32(0) // Default page
	}
	apiUrl = apiUrl + "$limit=" + strconv.FormatInt(int64(*limit), 10) + "&"
	apiUrl = apiUrl + "$page=" + strconv.FormatInt(int64(*page), 10) + "&"
	if filter != "" {
		apiUrl = apiUrl + "$filter=" + filter + "&"
	}
	if expand != "" {
		apiUrl = apiUrl + "$expand=" + expand
	}

	return apiUrl
}

// GetPathFromGrpcContext extracts the original path from gRPC context metadata
func GetPathFromGrpcContext(ctx context.Context) string {
	uriPath := GetVariableFromGrpcContext(ctx, EnvoyOriginalPath)
	if len(uriPath) > 0 {
		return uriPath[0]
	}
	return ""
}

// GetVariableFromGrpcContext extracts a variable from gRPC context metadata
func GetVariableFromGrpcContext(
	ctx context.Context, varName string) []string {
	if ctx == nil {
		log.Error("gRPC context is nil")
		return []string{}
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		log.Error("gRPC context doesn't have metadata")
		return []string{}
	}
	retVal, ok := md[varName]
	if !ok {
		log.Errorf("gRPC context metadata doesn't have %v", varName)
		return []string{}
	}
	return retVal
}

// GetSelfLink gets the self link from gRPC context
func GetSelfLink(ctx context.Context) *response.ApiLink {
	hostPort, err := responseutils.GetOriginHostPortFromGrpcContext(ctx)
	if err != nil {
		log.Errorf("Error in getting Host Port info from ctx %v", err)
	}
	if hostPort != "" && !strings.HasPrefix(hostPort, "https://") {
		hostPort = "https://" + hostPort
	}
	uriPath, err := url.Parse(GetPathFromGrpcContext(ctx))
	if err != nil {
		log.Errorf("Error in parsing URI path: %v", err)
	}
	uriPath.RawQuery = ""
	uriBase, err := url.Parse(hostPort)
	if err != nil {
		log.Errorf("Error in parsing Host Port for URI: %v", err)
	}
	selfUri := uriBase.ResolveReference(uriPath).String()

	selfLink := &response.ApiLink{
		Href: proto.String(selfUri),
		Rel:  proto.String("self"),
	}
	return selfLink
}

// GetPaginationLinks gets pagination links
func GetPaginationLinks(total int64, completeUrl string) []*response.ApiLink {
	paginationLinksList := []*response.ApiLink{}
	paginationLinks, err := responseutils.GetPaginationLinks(int(total), completeUrl)
	if err != nil {
		log.Errorf("Error while getting pagination links for %s: %v", completeUrl, err)
	}
	log.Debugf("paginationLinks: %+v", paginationLinks)

	for linkType, link := range paginationLinks {
		apiLink := &response.ApiLink{
			Href: proto.String(link),
			Rel:  proto.String(linkType),
		}
		paginationLinksList = append(paginationLinksList, apiLink)
	}
	return paginationLinksList
}

// CreateListItemsResponse creates a response for ListItems API with metadata
func CreateListItemsResponse(items []*pb.Item, paginationLinks []*response.ApiLink, isPaginated bool, totalAvailableResults int32) *pb.ListItemsRet {
	resp := &pb.ListItemsRet{
		Content: &pb.ListItemsApiResponse{
			Data: &pb.ListItemsApiResponse_ItemArrayData{
				ItemArrayData: &pb.ItemArrayWrapper{
					Value: items,
				},
			},
			Metadata: CreateResponseMetadata(false, isPaginated, paginationLinks, "", ""),
		},
	}
	resp.Content.Metadata.TotalAvailableResults = proto.Int32(totalAvailableResults)
	return resp
}

// CreateListItemsGroupResponse creates a response for ListItems API with GroupBy (ItemGroup objects)
func CreateListItemsGroupResponse(itemGroups []*pb.ItemGroup, paginationLinks []*response.ApiLink, isPaginated bool, totalAvailableResults int32) *pb.ListItemsRet {
	resp := &pb.ListItemsRet{
		Content: &pb.ListItemsApiResponse{
			Data: &pb.ListItemsApiResponse_ItemGroupArrayData{
				ItemGroupArrayData: &pb.ItemGroupArrayWrapper{
					Value: itemGroups,
				},
			},
			Metadata: CreateResponseMetadata(false, isPaginated, paginationLinks, "", ""),
		},
	}
	resp.Content.Metadata.TotalAvailableResults = proto.Int32(totalAvailableResults)
	return resp
}

// CreateListItemStatsResponse creates a response for ListItemStats API with metadata
func CreateListItemStatsResponse(stats []*pbStats.ItemStats, paginationLinks []*response.ApiLink, isPaginated bool, totalAvailableResults int32) *pbStats.ListItemStatsRet {
	resp := &pbStats.ListItemStatsRet{
		Content: &pbStats.ListItemStatsApiResponse{
			Data: &pbStats.ListItemStatsApiResponse_ItemStatsArrayData{
				ItemStatsArrayData: &pbStats.ItemStatsArrayWrapper{
					Value: stats,
				},
			},
			Metadata: CreateResponseMetadata(false, isPaginated, paginationLinks, "", ""),
		},
	}
	resp.Content.Metadata.TotalAvailableResults = proto.Int32(totalAvailableResults)
	return resp
}

// CreateListItemStatsGroupResponse creates a GroupBy response for ListItemStats API
func CreateListItemStatsGroupResponse(itemGroups []*pbStats.ItemStatsGroup, paginationLinks []*response.ApiLink, isPaginated bool, totalAvailableResults int32) *pbStats.ListItemStatsRet {
	resp := &pbStats.ListItemStatsRet{
		Content: &pbStats.ListItemStatsApiResponse{
			Data: &pbStats.ListItemStatsApiResponse_ItemStatsGroupArrayData{
				ItemStatsGroupArrayData: &pbStats.ItemStatsGroupArrayWrapper{
					Value: itemGroups,
				},
			},
			Metadata: CreateResponseMetadata(false, isPaginated, paginationLinks, "", ""),
		},
	}
	resp.Content.Metadata.TotalAvailableResults = proto.Int32(totalAvailableResults)
	return resp
}
