/*
 * gRPC Error Handler for Nexus API
 *
 * Builds gRPC status errors carrying an AppMessage payload.
 * We construct the AppMessage directly rather than going through
 * errorutils.AppMessageBuilder, because the library's template
 * validator (updateMessageForLessArguments) rejects messages whose
 * substituted text happens to contain unresolved-looking patterns
 * even when all arguments are provided.  Building the struct inline
 * is the same approach categories uses in GetErrorMessage — the
 * only difference is we skip the file-based template lookup.
 */

package grpc_error

import (
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/nutanix-core/ntnx-api-utils-go/errorutils"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/constants"
	nexusError "github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/errors/nexus_error"
)

var (
	grpcStatusUtilImpl     GrpcStatusUtil
	grpcStatusUtilImplOnce sync.Once
)

// GetGrpcStatusUtilImpl returns the singleton instance.
func GetGrpcStatusUtilImpl() GrpcStatusUtil {
	grpcStatusUtilImplOnce.Do(func() {
		if grpcStatusUtilImpl == nil {
			grpcStatusUtilImpl = &GrpcStatusUtilImpl{}
		}
	})
	return grpcStatusUtilImpl
}

// SetGrpcStatusUtil sets the singleton (for testing).
func SetGrpcStatusUtil(grpcStatusUtil GrpcStatusUtil) {
	grpcStatusUtilImpl = grpcStatusUtil
}

// GrpcStatusUtil interface for building gRPC errors.
type GrpcStatusUtil interface {
	BuildGrpcError(nexusError.NexusErrorInterface) error
}

// GrpcStatusUtilImpl implements GrpcStatusUtil.
type GrpcStatusUtilImpl struct{}

// BuildGrpcError builds a gRPC error with an AppMessage payload.
// The AppMessage is constructed directly from the NexusError fields
// so we are not dependent on external metadata / properties files.
func (e *GrpcStatusUtilImpl) BuildGrpcError(
	nexusErr nexusError.NexusErrorInterface,
) error {
	grpcCode := mapNexusErrorCodeToGrpcCode(nexusErr.GetErrorCode())

	message := nexusErr.GetErrorDetail()
	code := fmt.Sprintf(
		"%s-%d", constants.NexusErrorPrefix,
		nexusErr.GetErrorCode())
	locale := constants.EnglishLocale
	severity := errorutils.MESSAGESEVERITY_ERROR

	appMessage := &errorutils.AppMessage{
		Message:      &message,
		Code:         &code,
		Locale:       &locale,
		Severity:     &severity,
		ArgumentsMap: nexusErr.GetArgMap(),
	}

	grpcStatus, err := errorutils.BuildGrpcStatus(
		int32(grpcCode), appMessage)
	if err != nil {
		log.Errorf(
			"Failed to build gRPC status from AppMessage: %s", err)
		return status.Errorf(grpcCode, "%s", message)
	}

	return status.ErrorProto(grpcStatus)
}

// mapNexusErrorCodeToGrpcCode maps Nexus error codes to gRPC
// status codes.
func mapNexusErrorCodeToGrpcCode(errorCode int) codes.Code {
	switch errorCode {
	case constants.ErrorCodeODataParsingError:
		return codes.InvalidArgument
	case constants.ErrorCodeInternalError:
		return codes.Internal
	default:
		log.Warningf(
			"Unable to map Nexus error code '%d' to gRPC code, "+
				"defaulting to Internal", errorCode)
		return codes.Internal
	}
}
