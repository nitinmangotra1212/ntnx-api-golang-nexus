/*
 * OData Query Parser Error Handler
 * Handles OData parsing errors and converts them to AppMessage format
 * gRPC errors. Unlike the categories API (which returns a generic
 * message), we propagate the specific parser error detail so clients
 * can fix their query without guessing.
 */

package odata

import (
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/errors/grpc_error"
	nexusError "github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/errors/nexus_error"
	log "github.com/sirupsen/logrus"
)

// HandleODataError converts any OData parsing or evaluation error
// into a gRPC error carrying AppMessage NEXUS-50019 with the
// specific parser detail embedded.
//
// The second argument is kept for call-site compatibility but is
// no longer used for message construction; the raw err.Error()
// string is forwarded as the {{{detail}}} template argument.
func HandleODataError(
	err error, _ interface{},
) error {
	if err == nil {
		return nil
	}

	log.Errorf("OData error: %v", err)

	nexusErr := nexusError.GetODataParsingError(err.Error())

	grpcStatusUtil := grpc_error.GetGrpcStatusUtilImpl()
	return grpcStatusUtil.BuildGrpcError(nexusErr)
}

// HandleODataEvaluationError handles errors during OData to IDF
// query evaluation. Delegates to HandleODataError.
func HandleODataEvaluationError(
	err error, _ string,
) error {
	if err == nil {
		return nil
	}
	return HandleODataError(err, nil)
}

// WrapODataError wraps an OData parser error with context.
// Delegates to HandleODataError.
func WrapODataError(
	err error, _ string, _ string,
) error {
	if err == nil {
		return nil
	}
	return HandleODataError(err, nil)
}
