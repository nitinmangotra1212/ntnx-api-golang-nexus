/*
 * Nexus Error Implementation
 * Similar to GuruError and AzManagerError patterns
 */

package nexus_error

import (
	"fmt"
	"strconv"

	"github.com/nutanix-core/go-cache/util-go/errors"
	"github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/error"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/constants"
	"google.golang.org/protobuf/proto"
)

// NexusErrorInterface defines the interface for Nexus errors
type NexusErrorInterface interface {
	errors.INtnxError
	ConvertToAppMessagePb() *error.AppMessage
	GetArgMap() map[string]string
}

// NexusError represents a Nexus API error
type NexusError struct {
	*errors.NtnxError
	argMap map[string]string
}

// GetErrorCode returns the error code (implements INtnxError)
// The embedded NtnxError already implements GetErrorCode() from INtnxError interface
// This method is here for clarity, but the embedded struct's method will be used

// GetErrorDetail returns the error detail message (implements INtnxError)
// The embedded NtnxError already implements GetErrorDetail() from INtnxError interface
// This method is here for clarity, but the embedded struct's method will be used

// ConvertToAppMessagePb converts the Nexus error to an AppMessage proto
func (e *NexusError) ConvertToAppMessagePb() *error.AppMessage {
	return &error.AppMessage{
		Code: proto.String(strconv.Itoa(e.GetErrorCode())),
		ArgumentsMap: &error.StringMapWrapper{
			Value: e.argMap,
		},
	}
}

// GetArgMap returns the argument map
func (e *NexusError) GetArgMap() map[string]string {
	return e.argMap
}

// NewNexusError creates a new Nexus error
func NewNexusError(errCode int, argMap map[string]string) *NexusError {
	return &NexusError{
		NtnxError: errors.NewNtnxError("", errCode),
		argMap:    argMap,
	}
}

// NewNexusErrorWithMessage creates a new Nexus error with a message
func NewNexusErrorWithMessage(errMsg string, errCode int, argMap map[string]string) *NexusError {
	return &NexusError{
		NtnxError: errors.NewNtnxError(errMsg, errCode),
		argMap:    argMap,
	}
}

// GetODataParsingError creates an error for OData parsing failures
// Matches categories API error message format exactly:
// "Failed to list items as an error occurred while parsing the URI parameters. Check the provided query parameters."
func GetODataParsingError(operation string, queryParam string) *NexusError {
	argMap := map[string]string{
		"operation":  operation,
		"queryParam": queryParam,
	}
	// Use exact message format from categories API
	message := fmt.Sprintf("Failed to list %s as an error occurred while parsing the URI parameters. Check the provided query parameters.", operation)
	return NewNexusErrorWithMessage(
		message,
		constants.ErrorCodeODataParsingError,
		argMap,
	)
}

// GetODataPropertyNotFoundError creates an error for unknown property in OData query
func GetODataPropertyNotFoundError(propertyName string) *NexusError {
	argMap := map[string]string{
		"property": propertyName,
	}
	return NewNexusErrorWithMessage(
		fmt.Sprintf("Unknown property '%s' in OData query. Please check field names (itemId, itemName, itemType, extId).", propertyName),
		constants.ErrorCodeODataPropertyNotFound,
		argMap,
	)
}

// GetODataInvalidSyntaxError creates an error for invalid OData syntax
func GetODataInvalidSyntaxError(queryParam string) *NexusError {
	argMap := map[string]string{
		"queryParam": queryParam,
	}
	return NewNexusErrorWithMessage(
		fmt.Sprintf("Invalid OData query syntax in '%s'. Please check your $filter or $orderby expression.", queryParam),
		constants.ErrorCodeODataInvalidSyntax,
		argMap,
	)
}

// GetODataEvaluationError creates an error for OData evaluation failures
func GetODataEvaluationError(operation string) *NexusError {
	argMap := map[string]string{
		"operation": operation,
	}
	return NewNexusErrorWithMessage(
		fmt.Sprintf("Failed to evaluate OData query for %s. Please try a simpler query.", operation),
		constants.ErrorCodeODataEvaluationError,
		argMap,
	)
}

// GetInternalError creates an internal server error
func GetInternalError(operation string) *NexusError {
	argMap := map[string]string{
		"operation": operation,
	}
	return NewNexusErrorWithMessage(
		fmt.Sprintf("Internal error occurred while processing %s.", operation),
		constants.ErrorCodeInternalError,
		argMap,
	)
}
