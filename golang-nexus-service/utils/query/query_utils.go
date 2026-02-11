package query

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/models"
	responseUtils "github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/utils/response"
	log "github.com/sirupsen/logrus"
)

// getKeys returns all keys from url.Values map
func getKeys(values url.Values) []string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	return keys
}

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
		log.Warn("No path found in context, using defaults")
		return queryParams
	}

	// Parse the URL to extract query parameters
	log.Infof("🔍 Parsing path from context: %s", path)
	parsedURL, err := url.Parse(path)
	if err != nil {
		log.Warnf("Failed to parse path from context: %v, using defaults", err)
		return queryParams
	}

	values := parsedURL.Query()

	// Log all query parameters for debugging
	log.Infof("📋 All query parameters: %+v", values)
	log.Infof("📋 Raw query string: %s", parsedURL.RawQuery)

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

	// Extract $expand - this may contain special characters like parentheses and semicolons
	// values.Get() should handle URL decoding automatically
	if expand := values.Get("$expand"); expand != "" {
		queryParams.Expand = expand
		log.Infof("✅ Extracted $expand parameter: %s", expand)
	} else {
		// Try to get it directly from the raw query string if Get() didn't work
		// This might happen if the parameter contains special characters
		if rawQuery := parsedURL.RawQuery; rawQuery != "" {
			log.Infof("⚠️  $expand not found via Get(), checking raw query: %s", rawQuery)
			// Try to extract $expand manually from raw query
			if expandValues, ok := values["$expand"]; ok && len(expandValues) > 0 {
				queryParams.Expand = expandValues[0]
				log.Infof("✅ Extracted $expand from values map: %s", queryParams.Expand)
			} else {
				log.Warnf("❌ $expand parameter not found in query string. Available keys: %v", getKeys(values))
				// Try manual extraction from raw query string
				if strings.Contains(rawQuery, "$expand=") {
					parts := strings.Split(rawQuery, "$expand=")
					if len(parts) > 1 {
						// Extract until next & or $ (which indicates a new parameter) or end of string
						// BUT: $ can appear inside $expand value (e.g., $startTime), so we need to track parentheses
						expandValue := parts[1]
						log.Infof("🔍 [ExtractQueryParams] Raw expandValue after split: %s", expandValue)
						
						// Track parentheses depth to know if we're inside nested parameters
						// Only stop at $ if we're outside all parentheses
						parenDepth := 0
						endIdx := len(expandValue)
						
						for i := 0; i < len(expandValue); i++ {
							char := expandValue[i]
							if char == '(' {
								parenDepth++
								log.Debugf("🔍 [ExtractQueryParams] Found '(' at index %d, parenDepth=%d", i, parenDepth)
							} else if char == ')' {
								parenDepth--
								log.Debugf("🔍 [ExtractQueryParams] Found ')' at index %d, parenDepth=%d", i, parenDepth)
								if parenDepth == 0 {
									// Found matching closing paren - check if there's more after it
									// If next char is & or $ (outside parens), stop here
									if i+1 < len(expandValue) {
										nextChar := expandValue[i+1]
										if nextChar == '&' || nextChar == '$' {
											endIdx = i + 1
											log.Infof("🔍 [ExtractQueryParams] Found end at index %d (after ')' followed by '%c')", endIdx, nextChar)
											break
										}
									} else {
										// End of string after ')'
										endIdx = i + 1
										log.Infof("🔍 [ExtractQueryParams] Found end at index %d (end of string after ')')", endIdx)
										break
									}
								}
							} else if parenDepth == 0 && char == '&' {
								// Outside parentheses, found & - this is a new parameter
								endIdx = i
								log.Infof("🔍 [ExtractQueryParams] Found end at index %d (outside parens, found '&')", endIdx)
								break
							} else if parenDepth == 0 && char == '$' {
								// Outside parentheses, found $ - this might be a new parameter
								// But check if it's at the start of a new parameter (not part of current expand)
								// If we're at depth 0 and see $, it's likely a new parameter
								endIdx = i
								log.Infof("🔍 [ExtractQueryParams] Found end at index %d (outside parens, found '$')", endIdx)
								break
							}
						}
						
						log.Infof("🔍 [ExtractQueryParams] Extracted expandValue: %s (length: %d, endIdx: %d)", expandValue[:endIdx], len(expandValue), endIdx)
						expandValue = expandValue[:endIdx]
						
						// URL decode
						if decoded, err := url.QueryUnescape(expandValue); err == nil {
							queryParams.Expand = decoded
							log.Infof("✅ Manually extracted $expand from raw query: %s", queryParams.Expand)
						} else {
							log.Warnf("⚠️  Failed to URL decode $expand value: %s, error: %v", expandValue, err)
							// Use raw value if decoding fails
							queryParams.Expand = expandValue
							log.Infof("✅ Using raw $expand value (decoding failed): %s", queryParams.Expand)
						}
					}
				}
			}
		} else {
			log.Warnf("❌ No raw query string available")
		}
	}

	// Extract $apply - OData $apply parameter for GroupBy and Aggregations
	// This may contain special characters like parentheses: groupby(itemType)
	if apply := values.Get("$apply"); apply != "" {
		queryParams.Apply = apply
		log.Infof("✅ Extracted $apply parameter: %s", apply)
	} else {
		// Try to get it directly from the raw query string if Get() didn't work
		// This might happen if the parameter contains special characters
		if rawQuery := parsedURL.RawQuery; rawQuery != "" {
			log.Infof("⚠️  $apply not found via Get(), checking raw query: %s", rawQuery)
			// Try to extract $apply manually from raw query
			if applyValues, ok := values["$apply"]; ok && len(applyValues) > 0 {
				queryParams.Apply = applyValues[0]
				log.Infof("✅ Extracted $apply from values map: %s", queryParams.Apply)
			} else {
				log.Warnf("❌ $apply parameter not found in query string. Available keys: %v", getKeys(values))
				// Try manual extraction from raw query string
				if strings.Contains(rawQuery, "$apply=") {
					parts := strings.Split(rawQuery, "$apply=")
					if len(parts) > 1 {
						// Extract until next & or end of string
						applyValue := parts[1]
						if idx := strings.Index(applyValue, "&"); idx != -1 {
							applyValue = applyValue[:idx]
						}
						// URL decode
						if decoded, err := url.QueryUnescape(applyValue); err == nil {
							queryParams.Apply = decoded
							log.Infof("✅ Manually extracted $apply from raw query: %s", queryParams.Apply)
						}
					}
				}
			}
		} else {
			log.Warnf("❌ No raw query string available for $apply extraction")
		}
	}

	log.Infof("📥 Extracted query params: Page=%d, Limit=%d, Filter=%s, Expand=%s, Orderby=%s, Select=%s, Apply=%s",
		queryParams.Page, queryParams.Limit, queryParams.Filter, queryParams.Expand, queryParams.Orderby, queryParams.Select, queryParams.Apply)
	return queryParams
}
