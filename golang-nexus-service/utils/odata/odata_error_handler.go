/*
 * OData Query Parser Error Handler
 * Handles OData parsing errors and converts them to appropriate gRPC errors
 */

package odata

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HandleODataParseError handles OData parsing errors and converts them to gRPC status errors
// This provides user-friendly error messages for invalid OData queries
func HandleODataParseError(err error, queryParam string) error {
	if err == nil {
		return nil
	}

	// Log the original error for debugging
	log.Errorf("OData parsing error for query '%s': %v", queryParam, err)

	// Check error type and provide appropriate message
	errStr := err.Error()

	// Common OData parsing errors
	if strings.Contains(errStr, "syntax error") || strings.Contains(errStr, "parse") {
		return status.Errorf(codes.InvalidArgument,
			"Invalid OData query syntax in '%s': %v. Please check your $filter or $orderby expression.",
			queryParam, err)
	}

	if strings.Contains(errStr, "property") && strings.Contains(errStr, "not found") {
		return status.Errorf(codes.InvalidArgument,
			"Unknown property in OData query '%s': %v. Please check field names.",
			queryParam, err)
	}

	if strings.Contains(errStr, "operator") || strings.Contains(errStr, "unsupported") {
		return status.Errorf(codes.InvalidArgument,
			"Unsupported operator in OData query '%s': %v. Please check your expression.",
			queryParam, err)
	}

	if strings.Contains(errStr, "type mismatch") || strings.Contains(errStr, "type") {
		return status.Errorf(codes.InvalidArgument,
			"Type mismatch in OData query '%s': %v. Please check value types.",
			queryParam, err)
	}

	// Generic error for unknown parsing errors
	return status.Errorf(codes.InvalidArgument,
		"Failed to parse OData query '%s': %v", queryParam, err)
}

// HandleODataEvaluationError handles errors during OData to IDF query evaluation
func HandleODataEvaluationError(err error, queryParam string) error {
	if err == nil {
		return nil
	}

	log.Errorf("OData evaluation error for query '%s': %v", queryParam, err)

	errStr := err.Error()

	if strings.Contains(errStr, "evaluate") || strings.Contains(errStr, "evaluation") {
		return status.Errorf(codes.Internal,
			"Failed to evaluate OData query '%s': %v. Please try a simpler query.",
			queryParam, err)
	}

	return status.Errorf(codes.Internal,
		"Failed to process OData query '%s': %v", queryParam, err)
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
func WrapODataError(err error, operation string, queryParam string) error {
	if err == nil {
		return nil
	}

	// Check if it's already a gRPC status error
	if _, ok := status.FromError(err); ok {
		return err
	}

	// Wrap with operation context
	return fmt.Errorf("%s failed for query '%s': %w", operation, queryParam, err)
}

