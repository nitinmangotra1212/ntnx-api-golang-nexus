/*
 * OData Query Parser Error Handler
 * Handles OData parsing errors and converts them to AppMessage format gRPC errors
 * Following Nutanix API error handling patterns (az-manager, guru)
 */

package odata

import (
	"fmt"
	"strings"

	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/errors/grpc_error"
	nexusError "github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/errors/nexus_error"
	log "github.com/sirupsen/logrus"
)

// HandleODataError handles OData parsing/evaluation errors and converts them to AppMessage format
// This provides user-friendly error messages matching Nutanix API standards (categories API pattern)
// Example error: "Failed to list items as an error occurred while parsing the URI parameters. Check the provided query parameters."
func HandleODataError(err error, queryParams interface{}) error {
	if err == nil {
		return nil
	}

	// Log the original error for debugging
	log.Errorf("OData error: %v", err)

	errStr := strings.ToLower(err.Error())

	// Extract query parameter string for error messages
	queryParam := extractQueryParamString(queryParams)

	// Check error type and create appropriate NexusError
	var nexusErr nexusError.NexusErrorInterface

	// Property not found errors (e.g., typo in field name: itemNae instead of itemName)
	// Check for various patterns: "property", "field", "attribute", "column", "not found", "undefined", "unknown"
	if (strings.Contains(errStr, "property") || strings.Contains(errStr, "field") || strings.Contains(errStr, "attribute") || strings.Contains(errStr, "column")) &&
		(strings.Contains(errStr, "not found") || strings.Contains(errStr, "undefined") || strings.Contains(errStr, "unknown")) {
		// Try to extract property name from error message
		propertyName := extractPropertyName(err.Error()) // Use original case for property name
		nexusErr = nexusError.GetODataPropertyNotFoundError(propertyName)
	} else if strings.Contains(errStr, "syntax error") || strings.Contains(errStr, "parse") ||
		strings.Contains(errStr, "invalid odata query") || strings.Contains(errStr, "malformed") ||
		strings.Contains(errStr, "unexpected token") || strings.Contains(errStr, "invalid expression") {
		// Syntax errors - use generic parsing error (matches categories API)
		nexusErr = nexusError.GetODataParsingError("items", queryParam)
	} else if strings.Contains(errStr, "operator") && (strings.Contains(errStr, "unsupported") || strings.Contains(errStr, "invalid")) {
		// Unsupported/invalid operator - use generic parsing error
		nexusErr = nexusError.GetODataParsingError("items", queryParam)
	} else if strings.Contains(errStr, "evaluate") || strings.Contains(errStr, "evaluation") {
		// Evaluation errors
		nexusErr = nexusError.GetODataEvaluationError("listItems")
	} else {
		// Generic parsing error (matches categories API error message exactly)
		// This is the default for any OData parsing error
		nexusErr = nexusError.GetODataParsingError("items", queryParam)
	}

	// Build and return gRPC error with AppMessage
	grpcStatusUtil := grpc_error.GetGrpcStatusUtilImpl()
	return grpcStatusUtil.BuildGrpcError(nexusErr)
}

// extractQueryParamString extracts query parameter string from queryParams
func extractQueryParamString(queryParams interface{}) string {
	// Type switch to handle different queryParams types
	switch qp := queryParams.(type) {
	case *struct {
		Filter  string
		Orderby string
		Select  string
		Expand  string
	}:
		if qp.Filter != "" {
			return fmt.Sprintf("$filter=%s", qp.Filter)
		} else if qp.Orderby != "" {
			return fmt.Sprintf("$orderby=%s", qp.Orderby)
		} else if qp.Select != "" {
			return fmt.Sprintf("$select=%s", qp.Select)
		} else if qp.Expand != "" {
			return fmt.Sprintf("$expand=%s", qp.Expand)
		}
	case map[string]string:
		if filter, ok := qp["filter"]; ok && filter != "" {
			return fmt.Sprintf("$filter=%s", filter)
		}
		if orderby, ok := qp["orderby"]; ok && orderby != "" {
			return fmt.Sprintf("$orderby=%s", orderby)
		}
	}
	return ""
}

// extractPropertyName tries to extract property name from error message
// Handles various error message formats from OData parser
func extractPropertyName(errStr string) string {
	// Common patterns:
	// "property 'itemNae' not found"
	// "undefined property: itemNae"
	// "property itemNae is not defined"
	// "unknown property 'itemNae'"
	// "field 'itemNae' not found"

	// Try to extract between single quotes first (most common)
	if strings.Contains(errStr, "'") {
		parts := strings.Split(errStr, "'")
		if len(parts) >= 2 {
			prop := strings.TrimSpace(parts[1])
			if prop != "" {
				return prop
			}
		}
	}

	// Try double quotes
	if strings.Contains(errStr, "\"") {
		parts := strings.Split(errStr, "\"")
		if len(parts) >= 2 {
			prop := strings.TrimSpace(parts[1])
			if prop != "" {
				return prop
			}
		}
	}

	// Try to extract after colon (e.g., "undefined property: itemNae")
	if strings.Contains(errStr, ":") {
		parts := strings.Split(errStr, ":")
		if len(parts) >= 2 {
			prop := strings.TrimSpace(parts[len(parts)-1])
			// Remove common trailing words
			prop = strings.TrimSuffix(prop, " not found")
			prop = strings.TrimSuffix(prop, " is undefined")
			prop = strings.TrimSpace(prop)
			if prop != "" {
				return prop
			}
		}
	}

	// Try regex-like pattern: "property <name>"
	// This is a fallback - try to find word after "property" or "field"
	lowerErr := strings.ToLower(errStr)
	if strings.Contains(lowerErr, "property") {
		idx := strings.Index(lowerErr, "property")
		remaining := errStr[idx+len("property"):]
		words := strings.Fields(remaining)
		if len(words) > 0 {
			prop := strings.Trim(words[0], "'\"")
			if prop != "" && prop != "not" && prop != "is" {
				return prop
			}
		}
	}

	return "unknown"
}

// HandleODataEvaluationError handles errors during OData to IDF query evaluation
// Now uses AppMessage format via HandleODataError
func HandleODataEvaluationError(err error, queryParam string) error {
	if err == nil {
		return nil
	}

	// Use the unified error handler
	return HandleODataError(err, map[string]string{"queryParam": queryParam})
}

// ValidateODataQueryParams validates basic OData query parameters before parsing
func ValidateODataQueryParams(queryParams *struct {
	Filter  string
	Orderby string
	Select  string
	Expand  string
}) error {
	// Basic validation - check for empty strings that might cause issues
	// More complex validation is done by the OData parser itself

	if queryParams.Filter != "" && len(queryParams.Filter) > 10000 {
		return fmt.Errorf("$filter parameter is too long (max 10000 characters)")
	}

	if queryParams.Orderby != "" && len(queryParams.Orderby) > 1000 {
		return fmt.Errorf("$orderby parameter is too long (max 1000 characters)")
	}

	return nil
}

// WrapODataError wraps an OData parser error with context
// Now uses AppMessage format via HandleODataError
func WrapODataError(err error, operation string, queryParam string) error {
	if err == nil {
		return nil
	}

	// Use the unified error handler
	return HandleODataError(err, map[string]string{
		"operation":  operation,
		"queryParam": queryParam,
	})
}
