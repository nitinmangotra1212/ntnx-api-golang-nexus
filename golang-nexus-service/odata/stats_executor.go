/*
 * Stats Executor for Config+Stats Feature
 * Executes GraphQL queries against StatsGateway and maps responses
 *
 * Provides both sync and async execution methods:
 *   - ExecuteGraphqlQuerySync: Blocking execution
 *   - ExecuteGraphqlQueryAsync: Non-blocking with channel
 *
 * Based on Confluence: "Fetch Stats and Config Data Together using V4 APIs"
 */

package odata

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/external"
	log "github.com/sirupsen/logrus"
)

// StatsExecutor handles execution of GraphQL queries for stats
type StatsExecutor struct {
}

// NewStatsExecutor creates a new stats executor
func NewStatsExecutor() *StatsExecutor {
	return &StatsExecutor{}
}

// AsyncGraphqlResultChan is a channel wrapper for async GraphQL results
type AsyncGraphqlResultChan struct {
	resultChan chan *GraphqlResult
}

// GraphqlResult holds the result of a GraphQL query
type GraphqlResult struct {
	Data  *StatsGraphqlResponse
	Error error
}

// StatsGraphqlResponse represents the parsed GraphQL response for stats
type StatsGraphqlResponse struct {
	// Raw JSON data
	RawData string

	// Parsed entities
	ConfigEntities []map[string]interface{}
	StatsEntities  []map[string]interface{}

	// Metadata
	TotalCount    int64
	FilteredCount int64
	IsFlipped     bool
}

// CatStatsGraphqlDto represents parsed cat stats from GraphQL
type CatStatsGraphqlDto struct {
	CatId       string
	Timestamp   int64
	HeartRate   int32
	FoodIntake  float64
	SleepHours  float64
	Weight      float64
	Age         int32
}

// ExecuteGraphqlQuerySync executes a GraphQL query synchronously
func (e *StatsExecutor) ExecuteGraphqlQuerySync(ctx context.Context, query string) (*StatsGraphqlResponse, error) {
	log.Debugf("Executing GraphQL query sync:\n%s", query)

	// Get StatsGW client from singleton
	statsGWClient := external.Interfaces().StatsGWClient()
	if statsGWClient == nil {
		log.Warnf("StatsGW client not available, cannot execute GraphQL query")
		return nil, fmt.Errorf("StatsGW client not initialized")
	}

	// Execute the GraphQL query
	result, err := statsGWClient.ExecuteGraphql(ctx, query)
	if err != nil {
		log.Errorf("Failed to execute GraphQL query: %v", err)
		return nil, fmt.Errorf("GraphQL execution failed: %w", err)
	}

	// Parse the response
	response, err := parseGraphqlResponse(result.GetData())
	if err != nil {
		log.Errorf("Failed to parse GraphQL response: %v", err)
		return nil, fmt.Errorf("GraphQL response parsing failed: %w", err)
	}

	log.Infof("✅ GraphQL query executed successfully, got %d entities", len(response.StatsEntities))
	return response, nil
}

// ExecuteGraphqlQueryAsync executes a GraphQL query asynchronously
func (e *StatsExecutor) ExecuteGraphqlQueryAsync(ctx context.Context, query string) *AsyncGraphqlResultChan {
	resultChan := &AsyncGraphqlResultChan{
		resultChan: make(chan *GraphqlResult, 1),
	}

	go func() {
		defer close(resultChan.resultChan)

		result, err := e.ExecuteGraphqlQuerySync(ctx, query)
		resultChan.resultChan <- &GraphqlResult{
			Data:  result,
			Error: err,
		}
	}()

	return resultChan
}

// Await waits for the async result
func (c *AsyncGraphqlResultChan) Await() (*StatsGraphqlResponse, error) {
	result := <-c.resultChan
	if result == nil {
		return nil, fmt.Errorf("no result received")
	}
	return result.Data, result.Error
}

// parseGraphqlResponse parses the raw GraphQL JSON response
func parseGraphqlResponse(rawData string) (*StatsGraphqlResponse, error) {
	if rawData == "" {
		return &StatsGraphqlResponse{
			RawData:        rawData,
			StatsEntities:  []map[string]interface{}{},
			ConfigEntities: []map[string]interface{}{},
		}, nil
	}

	response := &StatsGraphqlResponse{
		RawData: rawData,
	}

	// Parse JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(rawData), &parsed); err != nil {
		log.Warnf("Failed to parse GraphQL JSON: %v, raw data: %s", err, rawData)
		return response, nil // Return empty response, not error
	}

	// Extract data section
	if data, ok := parsed["data"].(map[string]interface{}); ok {
		// Look for entity arrays
		for entityName, entityData := range data {
			if entities, ok := entityData.([]interface{}); ok {
				for _, entity := range entities {
					if entityMap, ok := entity.(map[string]interface{}); ok {
						// Determine if this is a stats or config entity
						if isStatsEntity(entityName) {
							response.StatsEntities = append(response.StatsEntities, entityMap)
						} else {
							response.ConfigEntities = append(response.ConfigEntities, entityMap)
						}
					}
				}
			}
		}
	}

	// Extract metadata if present
	if meta, ok := parsed["metadata"].(map[string]interface{}); ok {
		if total, ok := meta["total_entity_count"].(float64); ok {
			response.TotalCount = int64(total)
		}
		if filtered, ok := meta["filtered_entity_count"].(float64); ok {
			response.FilteredCount = int64(filtered)
		}
	}

	log.Debugf("Parsed GraphQL response: %d stats entities, %d config entities",
		len(response.StatsEntities), len(response.ConfigEntities))

	return response, nil
}

// isStatsEntity checks if the entity name is a stats entity
func isStatsEntity(entityName string) bool {
	// Stats entities typically have "stats" suffix or are known stats tables
	statsEntities := map[string]bool{
		"cat_stats":    true,
		"vm":           true, // vm in StatsGW context is stats
		"vm_stats":     true,
		"disk_stats":   true,
		"host_stats":   true,
		// Add more as needed
	}
	return statsEntities[entityName]
}

// ExtractStatsFromGraphql extracts CatStats from GraphQL response
func ExtractStatsFromGraphql(entities []map[string]interface{}) []CatStatsGraphqlDto {
	stats := make([]CatStatsGraphqlDto, 0, len(entities))

	for _, entity := range entities {
		stat := CatStatsGraphqlDto{}

		// Extract fields with type assertions
		if catId, ok := getArrayFirst[string](entity, "cat_id"); ok {
			stat.CatId = catId
		}
		if timestamp, ok := getArrayFirst[float64](entity, "timestamp"); ok {
			stat.Timestamp = int64(timestamp)
		}
		if heartRate, ok := getArrayFirst[float64](entity, "heart_rate"); ok {
			stat.HeartRate = int32(heartRate)
		}
		if foodIntake, ok := getArrayFirst[float64](entity, "food_intake"); ok {
			stat.FoodIntake = foodIntake
		}
		if sleepHours, ok := getArrayFirst[float64](entity, "sleep_hours"); ok {
			stat.SleepHours = sleepHours
		}
		if weight, ok := getArrayFirst[float64](entity, "weight"); ok {
			stat.Weight = weight
		}
		if age, ok := getArrayFirst[float64](entity, "age"); ok {
			stat.Age = int32(age)
		}

		stats = append(stats, stat)
	}

	return stats
}

// getArrayFirst extracts the first element from an array field
// GraphQL returns arrays for each column (parallel arrays pattern)
func getArrayFirst[T any](entity map[string]interface{}, key string) (T, bool) {
	var zero T
	if arr, ok := entity[key].([]interface{}); ok && len(arr) > 0 {
		if val, ok := arr[0].(T); ok {
			return val, true
		}
	}
	// Try direct value (not array)
	if val, ok := entity[key].(T); ok {
		return val, true
	}
	return zero, false
}

// MergeConfigAndStats merges config entities with their stats
// This is used for normal queries (Config → Stats)
func MergeConfigAndStats(configEntities []map[string]interface{}, statsEntities []map[string]interface{}, joinKey string) []map[string]interface{} {
	// Build stats lookup by join key
	statsLookup := make(map[string][]map[string]interface{})
	for _, stats := range statsEntities {
		if key, ok := getStringValue(stats, joinKey); ok {
			statsLookup[key] = append(statsLookup[key], stats)
		}
	}

	// Merge stats into config entities
	for i := range configEntities {
		if key, ok := getStringValue(configEntities[i], joinKey); ok {
			if relatedStats, found := statsLookup[key]; found {
				configEntities[i]["stats"] = relatedStats
			}
		}
	}

	return configEntities
}

// getStringValue extracts a string value from entity
func getStringValue(entity map[string]interface{}, key string) (string, bool) {
	// Try array first (GraphQL pattern)
	if arr, ok := entity[key].([]interface{}); ok && len(arr) > 0 {
		if str, ok := arr[0].(string); ok {
			return str, true
		}
	}
	// Try direct value
	if str, ok := entity[key].(string); ok {
		return str, true
	}
	return "", false
}

// TransformFlippedResponse transforms a flipped query response back to normal format
// This is used when the query was flipped (Stats → Config) but response should be Config → Stats
func TransformFlippedResponse(response *StatsGraphqlResponse) *StatsGraphqlResponse {
	if !response.IsFlipped {
		return response
	}

	// Swap the entities
	response.ConfigEntities, response.StatsEntities = response.StatsEntities, response.ConfigEntities
	response.IsFlipped = false

	log.Debugf("Transformed flipped response: %d config, %d stats",
		len(response.ConfigEntities), len(response.StatsEntities))

	return response
}

