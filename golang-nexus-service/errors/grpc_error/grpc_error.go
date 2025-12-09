/*
 * gRPC Error Handler for Nexus API
 * Builds gRPC status errors with AppMessage format
 * Following patterns from az-manager and guru
 */

package grpc_error

import (
	"strconv"
	"sync"

	log "github.com/sirupsen/logrus"
	statusPb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/nutanix-core/ntnx-api-utils-go/errorutils"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/constants"
	nexusError "github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/errors/nexus_error"
)

// singleton instance of GrpcStatusUtil
var (
	grpcStatusUtilImpl     GrpcStatusUtil
	grpcStatusUtilImplOnce sync.Once
)

// GetGrpcStatusUtilImpl returns the singleton instance
func GetGrpcStatusUtilImpl() GrpcStatusUtil {
	grpcStatusUtilImplOnce.Do(func() {
		if grpcStatusUtilImpl == nil {
			grpcStatusUtilImpl = NewGrpcStatusUtilImpl()
		}
	})
	return grpcStatusUtilImpl
}

// SetGrpcStatusUtil sets the singleton (for testing)
func SetGrpcStatusUtil(grpcStatusUtil GrpcStatusUtil) {
	grpcStatusUtilImpl = grpcStatusUtil
}

// GrpcStatusUtil interface for building gRPC errors
type GrpcStatusUtil interface {
	BuildGrpcError(nexusError.NexusErrorInterface) error
}

// GrpcStatusUtilImpl implements GrpcStatusUtil
type GrpcStatusUtilImpl struct {
	errorutils.AppMessageBuilderInterface
}

// NewGrpcStatusUtilImpl creates a new GrpcStatusUtilImpl
func NewGrpcStatusUtilImpl() *GrpcStatusUtilImpl {
	return &GrpcStatusUtilImpl{
		AppMessageBuilderInterface: errorutils.NewAppMessageBuilder(),
	}
}

// BuildGrpcError builds a gRPC error with AppMessage format from a NexusError
// This gRPC error will be sent back to Adonis as a response
// The Client will see the appropriate error based on the mapping of Nexus Error Code -> gRPC Code
func (e *GrpcStatusUtilImpl) BuildGrpcError(nexusErr nexusError.NexusErrorInterface) error {
	// Build a default internal error status
	internalErrStatus := &statusPb.Status{
		Code:    int32(codes.Internal),
		Message: nexusErr.GetErrorDetail(),
	}

	// Build AppMessage using errorutils
	appMessage, err := e.AppMessageBuilderInterface.BuildAppMessage(
		constants.EnglishLocale,
		constants.NexusNamespace,
		constants.NexusErrorPrefix,
		strconv.Itoa(nexusErr.GetErrorCode()),
		nexusErr.ConvertToAppMessagePb().GetArgumentsMap().GetValue(),
	)
	if err != nil {
		log.Errorf("Failed to build app message: %s", err)
		// Fallback to internal error
		appMessage, err = e.AppMessageBuilderInterface.BuildAppMessage(
			constants.EnglishLocale,
			constants.NexusNamespace,
			constants.NexusErrorPrefix,
			strconv.Itoa(constants.ErrorCodeInternalError),
			nil,
		)
		if err != nil {
			log.Errorf("Failed to build app message for internal error: %s", err)
			return status.ErrorProto(internalErrStatus)
		}
	}

	// Map error code to gRPC code
	grpcCode := mapNexusErrorCodeToGrpcCode(nexusErr.GetErrorCode())

	// Build gRPC status with AppMessage
	grpcStatus, err := errorutils.BuildGrpcStatus(int32(grpcCode), appMessage)
	if err != nil {
		log.Errorf("Failed to build gRPC status from app message: %s", err)
		return status.ErrorProto(internalErrStatus)
	}

	return status.ErrorProto(grpcStatus)
}

// mapNexusErrorCodeToGrpcCode maps Nexus error codes to gRPC status codes
func mapNexusErrorCodeToGrpcCode(errorCode int) codes.Code {
	switch errorCode {
	case constants.ErrorCodeODataParsingError,
		constants.ErrorCodeODataPropertyNotFound,
		constants.ErrorCodeODataInvalidSyntax:
		return codes.InvalidArgument
	case constants.ErrorCodeODataEvaluationError:
		return codes.Internal
	case constants.ErrorCodeInternalError:
		return codes.Internal
	default:
		log.Warningf("Unable to map Nexus error code '%d' to gRPC code, defaulting to Internal", errorCode)
		return codes.Internal
	}
}
