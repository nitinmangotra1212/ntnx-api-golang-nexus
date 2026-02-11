/*
 * StatsGW Client for GraphQL Query Execution
 * Connects to statsGW service to execute GraphQL queries for $expand functionality
 * Uses gRPC client (like ntnx-api-utils-go) instead of HTTP POST
 */

package statsgw

import (
	"context"
	"flag"
	"fmt"

	graphqlProto "github.com/nutanix-core/go-cache/stats_gateway/graphql_interface"
	util_net "github.com/nutanix-core/go-cache/util-go/net"
	log "github.com/sirupsen/logrus"
)

var (
	// RpcQueryTimeoutInSecs is the timeout for RPC calls (default 60 seconds)
	RpcQueryTimeoutInSecs = flag.Int64("statsGWQueryTimeoutInSecs", 60, "Query Timeout for statsGW RPC Call in seconds")
)

// StatsGWClient handles connection and execution of GraphQL queries via statsGW
// Uses gRPC client (like ntnx-api-utils-go)
type StatsGWClient struct {
	client graphqlProto.IGroupsGraphqlRpcClient
}

// NewStatsGWClient creates a new statsGW client connection using gRPC
func NewStatsGWClient(host string, port int) (*StatsGWClient, error) {
	log.Infof("Connecting to statsGW at %s:%d (using gRPC client)", host, port)

	// Create gRPC client using the same approach as ntnx-api-utils-go
	protobufRPCClient := util_net.NewProtobufRPCClient(host, uint16(port))
	protobufRPCClient.SetRequestTimeout(*RpcQueryTimeoutInSecs)
	graphqlClient := graphqlProto.NewGroupsGraphqlRpcClient(protobufRPCClient)

	log.Info("✅ StatsGW client initialized (gRPC mode)")

	return &StatsGWClient{
		client: graphqlClient,
	}, nil
}

// ExecuteGraphql executes a GraphQL query via statsGW and returns the response
// Uses gRPC client (like ntnx-api-utils-go)
func (c *StatsGWClient) ExecuteGraphql(ctx context.Context, query string) (*GroupsGraphqlRet, error) {
	log.Debugf("Executing GraphQL query via statsGW: %s", query)

	// Create protobuf request
	groupsArg := graphqlProto.GroupsGraphqlArg{
		Query: &query,
	}

	// Execute GraphQL via gRPC client
	// Note: go-cache's IGroupsGraphqlRpcClient.ExecuteGraphql doesn't take context
	response, err := c.client.ExecuteGraphql(&groupsArg)
	if err != nil {
		return nil, fmt.Errorf("statsGW gRPC call failed: %w (check if statsGW is running: netstat -tlnp | grep 8084)", err)
	}

	if response == nil {
		return nil, fmt.Errorf("statsGW returned nil response")
	}

	// Convert from go-cache's GroupsGraphqlRet to our local GroupsGraphqlRet
	// go-cache's Data is *string, our local Data is string
	ret := &GroupsGraphqlRet{}
	if response.Data != nil {
		ret.Data = *response.Data
	}

	log.Debugf("✅ GraphQL query executed successfully, response data length: %d", len(ret.GetData()))
	return ret, nil
}

// Close closes the HTTP client (no-op for HTTP client, but needed for interface compatibility)
func (c *StatsGWClient) Close() error {
	// HTTP client doesn't need explicit closing
	return nil
}
