/*
 * StatsGW Client Interface
 * Defines the interface for statsGW client operations
 */

package statsgw

import (
	"context"
)

// StatsGWClientIfc defines the interface for statsGW client
type StatsGWClientIfc interface {
	ExecuteGraphql(ctx context.Context, query string) (*GroupsGraphqlRet, error)
	Close() error
}
