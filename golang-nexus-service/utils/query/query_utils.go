package query

import (
	"context"
	"net/url"
	"strconv"

	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/models"
	responseUtils "github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/utils/response"
	log "github.com/sirupsen/logrus"
)

// ExtractQueryParamsFromContext extracts query parameters from gRPC context metadata
// This looks for OData query parameters in the original HTTP request path
func ExtractQueryParamsFromContext(ctx context.Context) *models.QueryParams {
	queryParams := &models.QueryParams{
		Page:  0,
		Limit: 50, // Default limit
	}

	// Get the original path from context
	path := responseUtils.GetPathFromGrpcContext(ctx)
	if path == "" {
		log.Debug("No path found in context, using defaults")
		return queryParams
	}

	// Parse the URL to extract query parameters
	parsedURL, err := url.Parse(path)
	if err != nil {
		log.Warnf("Failed to parse path from context: %v, using defaults", err)
		return queryParams
	}

	values := parsedURL.Query()

	// Extract $page
	if pageStr := values.Get("$page"); pageStr != "" {
		if page, err := strconv.ParseInt(pageStr, 10, 32); err == nil {
			queryParams.Page = int32(page)
		}
	}

	// Extract $limit
	if limitStr := values.Get("$limit"); limitStr != "" {
		if limit, err := strconv.ParseInt(limitStr, 10, 32); err == nil {
			queryParams.Limit = int32(limit)
		}
	}

	// Extract $filter
	if filter := values.Get("$filter"); filter != "" {
		queryParams.Filter = filter
	}

	// Extract $orderby
	if orderby := values.Get("$orderby"); orderby != "" {
		queryParams.Orderby = orderby
	}

	// Extract $select
	if selectParam := values.Get("$select"); selectParam != "" {
		queryParams.Select = selectParam
	}

	// Extract $expand
	if expand := values.Get("$expand"); expand != "" {
		queryParams.Expand = expand
	}

	log.Debugf("Extracted query params: Page=%d, Limit=%d, Filter=%s", queryParams.Page, queryParams.Limit, queryParams.Filter)
	return queryParams
}
