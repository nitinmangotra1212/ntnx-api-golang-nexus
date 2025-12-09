/*
 * StatsGW Client for GraphQL Query Execution
 * Connects to statsGW service to execute GraphQL queries for $expand functionality
 * Based on categories implementation pattern (RpcClientOnHttp - gRPC over HTTP/1.1)
 */

package statsgw

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

// StatsGWClient handles connection and execution of GraphQL queries via statsGW
// Uses HTTP/1.1 to send protobuf messages (mimicking Java's RpcClientOnHttp)
type StatsGWClient struct {
	baseURL string
	client  *http.Client
}

// NewStatsGWClient creates a new statsGW client connection
// statsGW uses gRPC over HTTP/1.1 (RpcClientOnHttp), not pure gRPC
func NewStatsGWClient(host string, port int) (*StatsGWClient, error) {
	baseURL := fmt.Sprintf("http://%s:%d", host, port)
	log.Infof("Connecting to statsGW at %s (using HTTP/1.1 for gRPC over HTTP)", baseURL)

	// Create HTTP client for gRPC over HTTP/1.1
	httpClient := &http.Client{
		Timeout: 0, // No timeout for long-running queries
	}

	log.Info("✅ StatsGW client initialized (HTTP/1.1 mode)")

	return &StatsGWClient{
		baseURL: baseURL,
		client:  httpClient,
	}, nil
}

// ExecuteGraphql executes a GraphQL query via statsGW and returns the response
// Uses HTTP POST to send protobuf message (mimicking Java's RpcClientOnHttp)
func (c *StatsGWClient) ExecuteGraphql(ctx context.Context, query string) (*GroupsGraphqlRet, error) {
	log.Debugf("Executing GraphQL query via statsGW: %s", query)

	// Create protobuf request
	arg := &GroupsGraphqlArg{
		Query: query,
	}

	// Serialize protobuf to bytes
	reqData, err := proto.Marshal(arg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	// Send HTTP POST request to statsGW
	// Service path: /stats_gateway.graphql_interface.GroupsGraphqlRpcSvc/ExecuteGraphql
	servicePath := "/stats_gateway.graphql_interface.GroupsGraphqlRpcSvc/ExecuteGraphql"
	url := c.baseURL + servicePath

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers for protobuf over HTTP
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Accept", "application/x-protobuf")

	log.Debugf("Sending HTTP POST to %s", url)
	resp, err := c.client.Do(req)
	if err != nil {
		log.Errorf("Failed to execute HTTP request to statsGW: %v", err)
		return nil, fmt.Errorf("statsGW HTTP request failed: %w (check if statsGW is running: netstat -tlnp | grep 8084)", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("statsGW returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Read protobuf response
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read statsGW response: %w", err)
	}

	// Deserialize protobuf response
	ret := &GroupsGraphqlRet{}
	if err := proto.Unmarshal(respData, ret); err != nil {
		return nil, fmt.Errorf("failed to unmarshal GraphQL response: %w", err)
	}

	if ret == nil {
		return nil, fmt.Errorf("statsGW returned nil response")
	}

	log.Debugf("✅ GraphQL query executed successfully, response data length: %d", len(ret.GetData()))
	return ret, nil
}

// Close closes the HTTP client (no-op for HTTP client, but needed for interface compatibility)
func (c *StatsGWClient) Close() error {
	// HTTP client doesn't need explicit closing
	return nil
}
