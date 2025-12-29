/*
 * GraphQL Stats Query Evaluator for Config+Stats Feature
 * Generates GraphQL queries for stats expansion from config APIs
 *
 * Supports:
 *   - Normal query: Config entity with stats expansion
 *   - Flipped query: Stats as primary entity with config expansion
 *   - Time-series parameters ($startTime, $endTime, $samplingInterval)
 *   - Nested OData operations ($filter, $select, $orderby)
 *
 * Based on Confluence: "Fetch Stats and Config Data Together using V4 APIs"
 *
 * Example Normal Query (Config → Stats):
 *   query {
 *     cat {
 *       _entity_id_
 *       cat_name
 *       weight
 *       cat_stats(args: {
 *         left_column: _entity_id_
 *         right_column: _entity_id_
 *         condition_operator: equal
 *         join_type: left_outer_join
 *         interval_start_ms: 1656309600000000
 *         interval_end_ms: 1751006280000000
 *         downsampling_interval_secs: 60
 *       }) {
 *         heart_rate(sampling: AVG, timeseries: true)
 *         weight(sampling: AVG, timeseries: true)
 *         _entity_id_
 *       }
 *     }
 *   }
 *
 * Example Flipped Query (Stats → Config):
 *   query {
 *     cat_stats(args: {
 *       interval_start_ms: 1656309600000000
 *       interval_end_ms: 1751006280000000
 *       downsampling_interval_secs: 60
 *     }) {
 *       heart_rate(sampling: AVG, timeseries: true)
 *       _entity_id_
 *       cat(args: {
 *         left_column: _entity_id_
 *         right_column: _entity_id_
 *         condition_operator: equal
 *         join_type: left_outer_join
 *       }) {
 *         _entity_id_
 *         cat_name
 *         weight
 *       }
 *     }
 *   }
 */

package odata

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

// GraphQLStatsQuery holds the generated GraphQL query info
type GraphQLStatsQuery struct {
	Query         string
	IsFlipped     bool
	PrimaryEntity string
	ExpandEntity  string
}

// StatsEntityConfig defines the entity configuration for query generation
type StatsEntityConfig struct {
	// Config entity info
	ConfigEntityName     string   // IDF table name: "cat"
	ConfigEntityColumns  []string // IDF columns: ["cat_id", "cat_name", "cat_type", "weight", "age", "ext_id"]
	ConfigJoinColumn     string   // Join column: "_entity_id_" or "ext_id"

	// Stats entity info
	StatsEntityName     string   // IDF/StatsGW table name: "cat_stats"
	StatsEntityColumns  []string // Stats columns: ["heart_rate", "food_intake", "sleep_hours", "weight"]
	StatsJoinColumn     string   // Join column: "cat_id" or "_entity_id_"

	// Mapping info from x-expand-items
	MappingType string // "ONE_TO_ONE" or "ONE_TO_MANY"
}

// CatStatsEntityConfig returns the entity configuration for Cat+CatStats
func CatStatsEntityConfig() *StatsEntityConfig {
	return &StatsEntityConfig{
		ConfigEntityName: "cat",
		ConfigEntityColumns: []string{
			"_entity_id_",
			"cat_id",
			"cat_name",
			"cat_type",
			"weight",
			"age",
			"ext_id",
		},
		ConfigJoinColumn: "_entity_id_",

		StatsEntityName: "cat_stats",
		StatsEntityColumns: []string{
			"cat_id",
			"timestamp",
			"heart_rate",
			"food_intake",
			"sleep_hours",
			"weight",
			"age",
		},
		StatsJoinColumn: "cat_id",

		MappingType: "ONE_TO_MANY",
	}
}

// GenerateStatsExpansionQuery generates a GraphQL query for stats expansion
// Returns the query string and whether the query was flipped
func GenerateStatsExpansionQuery(config *StatsEntityConfig, params *StatsExpandParams) (*GraphQLStatsQuery, error) {
	if config == nil {
		return nil, fmt.Errorf("entity config is required")
	}

	// Initialize params with defaults if nil
	if params == nil {
		params = &StatsExpandParams{}
	}

	// Determine if query should be flipped
	if params.ShouldFlipQuery() {
		return generateFlippedQuery(config, params)
	}

	return generateNormalQuery(config, params)
}

// generateNormalQuery generates a normal query (Config → Stats)
func generateNormalQuery(config *StatsEntityConfig, params *StatsExpandParams) (*GraphQLStatsQuery, error) {
	var sb strings.Builder

	// Get time range
	startMs, endMs := params.GetTimeRangeMs()
	samplingInterval := params.GetSamplingIntervalSecs()
	statType := params.GetStatTypeSampling()

	// Start query
	sb.WriteString("query {\n")

	// Primary entity (config)
	sb.WriteString(fmt.Sprintf("  %s {\n", config.ConfigEntityName))

	// Config entity columns
	for _, col := range config.ConfigEntityColumns {
		sb.WriteString(fmt.Sprintf("    %s\n", col))
	}

	// Stats expansion with join args
	sb.WriteString(fmt.Sprintf("    %s(\n", config.StatsEntityName))
	sb.WriteString("      args: {\n")
	sb.WriteString(fmt.Sprintf("        left_column: %s\n", config.ConfigJoinColumn))
	sb.WriteString(fmt.Sprintf("        right_column: %s\n", config.StatsJoinColumn))
	sb.WriteString("        condition_operator: equal\n")
	sb.WriteString("        join_type: left_outer_join\n")
	sb.WriteString(fmt.Sprintf("        interval_start_ms: %d\n", startMs))
	sb.WriteString(fmt.Sprintf("        interval_end_ms: %d\n", endMs))
	sb.WriteString(fmt.Sprintf("        downsampling_interval_secs: %d\n", samplingInterval))
	sb.WriteString("      }\n")
	sb.WriteString("    ) {\n")

	// Stats columns with sampling
	statsColumns := config.StatsEntityColumns
	if len(params.Select) > 0 {
		// Use selected columns only
		statsColumns = mapToIdfColumns(params.Select)
	}

	for _, col := range statsColumns {
		if col == "_entity_id_" || col == "cat_id" || col == "timestamp" {
			// ID/timestamp columns don't need sampling
			sb.WriteString(fmt.Sprintf("      %s\n", col))
		} else {
			// Metric columns with sampling
			sb.WriteString(fmt.Sprintf("      %s(sampling: %s, timeseries: true)\n", col, statType))
		}
	}

	sb.WriteString("    }\n") // Close stats entity
	sb.WriteString("  }\n")   // Close config entity
	sb.WriteString("}\n")     // Close query

	query := sb.String()
	log.Debugf("Generated normal GraphQL query:\n%s", query)

	return &GraphQLStatsQuery{
		Query:         query,
		IsFlipped:     false,
		PrimaryEntity: config.ConfigEntityName,
		ExpandEntity:  config.StatsEntityName,
	}, nil
}

// generateFlippedQuery generates a flipped query (Stats → Config)
// This is used when sorting by stats attributes
func generateFlippedQuery(config *StatsEntityConfig, params *StatsExpandParams) (*GraphQLStatsQuery, error) {
	var sb strings.Builder

	// Get time range
	startMs, endMs := params.GetTimeRangeMs()
	samplingInterval := params.GetSamplingIntervalSecs()
	statType := params.GetStatTypeSampling()

	// Start query
	sb.WriteString("query {\n")

	// Primary entity (stats) - with time-series args at top level
	sb.WriteString(fmt.Sprintf("  %s(\n", config.StatsEntityName))
	sb.WriteString("    args: {\n")
	sb.WriteString(fmt.Sprintf("      interval_start_ms: %d\n", startMs))
	sb.WriteString(fmt.Sprintf("      interval_end_ms: %d\n", endMs))
	sb.WriteString(fmt.Sprintf("      downsampling_interval_secs: %d\n", samplingInterval))
	sb.WriteString("    }\n")
	sb.WriteString("  ) {\n")

	// Stats columns with sampling
	statsColumns := config.StatsEntityColumns
	if len(params.Select) > 0 {
		statsColumns = mapToIdfColumns(params.Select)
	}

	for _, col := range statsColumns {
		if col == "_entity_id_" || col == "cat_id" || col == "timestamp" {
			sb.WriteString(fmt.Sprintf("    %s\n", col))
		} else {
			sb.WriteString(fmt.Sprintf("    %s(sampling: %s, timeseries: true)\n", col, statType))
		}
	}

	// Config expansion (as child entity)
	sb.WriteString(fmt.Sprintf("    %s(\n", config.ConfigEntityName))
	sb.WriteString("      args: {\n")
	sb.WriteString(fmt.Sprintf("        left_column: %s\n", config.StatsJoinColumn))
	sb.WriteString(fmt.Sprintf("        right_column: %s\n", config.ConfigJoinColumn))
	sb.WriteString("        condition_operator: equal\n")
	sb.WriteString("        join_type: left_outer_join\n")
	sb.WriteString("      }\n")
	sb.WriteString("    ) {\n")

	// Config columns
	for _, col := range config.ConfigEntityColumns {
		sb.WriteString(fmt.Sprintf("      %s\n", col))
	}

	sb.WriteString("    }\n") // Close config entity
	sb.WriteString("  }\n")   // Close stats entity
	sb.WriteString("}\n")     // Close query

	query := sb.String()
	log.Infof("Generated FLIPPED GraphQL query (Stats → Config):\n%s", query)

	return &GraphQLStatsQuery{
		Query:         query,
		IsFlipped:     true,
		PrimaryEntity: config.StatsEntityName,
		ExpandEntity:  config.ConfigEntityName,
	}, nil
}

// mapToIdfColumns converts OData field names (camelCase) to IDF column names (snake_case)
func mapToIdfColumns(odataFields []string) []string {
	// Mapping from camelCase to snake_case
	mapping := map[string]string{
		"catId":       "cat_id",
		"catName":     "cat_name",
		"catType":     "cat_type",
		"weight":      "weight",
		"age":         "age",
		"extId":       "ext_id",
		"heartRate":   "heart_rate",
		"foodIntake":  "food_intake",
		"sleepHours":  "sleep_hours",
		"timestamp":   "timestamp",
		// Add more mappings as needed
	}

	result := make([]string, 0, len(odataFields))
	for _, field := range odataFields {
		if idfCol, ok := mapping[field]; ok {
			result = append(result, idfCol)
		} else {
			// Assume it's already in snake_case or use as-is
			result = append(result, field)
		}
	}

	// Always include entity ID for joining
	hasEntityId := false
	for _, col := range result {
		if col == "_entity_id_" || col == "cat_id" {
			hasEntityId = true
			break
		}
	}
	if !hasEntityId {
		result = append(result, "_entity_id_")
	}

	return result
}

// GenerateGraphQLFilter generates a GraphQL filter clause from OData filter
// Handles "stats/any(a:a/field op value)" pattern
func GenerateGraphQLFilter(odataFilter string) string {
	if odataFilter == "" {
		return ""
	}

	// Parse "stats/any(a:a/heartRate gt 70)" pattern
	// Convert to GraphQL: where: { heart_rate: { _gt: 70 } }
	// This is a simplified version - full implementation would need proper parser

	// For now, log that we detected a filter
	log.Debugf("OData filter for stats: %s", odataFilter)

	// TODO: Implement full OData to GraphQL filter conversion
	return ""
}

// GenerateGraphQLOrderBy generates a GraphQL order_by clause from OData orderby
func GenerateGraphQLOrderBy(orderBy *StatsOrderBy) string {
	if orderBy == nil || orderBy.Field == "" {
		return ""
	}

	// Map field name to IDF column
	fieldMapping := map[string]string{
		"heartRate":   "heart_rate",
		"foodIntake":  "food_intake",
		"sleepHours":  "sleep_hours",
		"weight":      "weight",
		"timestamp":   "timestamp",
	}

	idfField := orderBy.Field
	if mapped, ok := fieldMapping[orderBy.Field]; ok {
		idfField = mapped
	}

	direction := "asc"
	if orderBy.Direction == "desc" {
		direction = "desc"
	}

	// GraphQL order_by syntax
	return fmt.Sprintf("order_by: { %s: %s }", idfField, direction)
}

// QueryMetadata contains metadata about the parsed query
type QueryMetadata struct {
	IsQueryFlipped bool
	PrimaryModule  string // "config" or "stats"
	ExpandModule   string // "stats" or "config"
}

// GetQueryMetadata returns metadata about the query
func GetQueryMetadata(params *StatsExpandParams) *QueryMetadata {
	if params == nil {
		return &QueryMetadata{
			IsQueryFlipped: false,
			PrimaryModule:  "config",
			ExpandModule:   "stats",
		}
	}

	if params.ShouldFlipQuery() {
		return &QueryMetadata{
			IsQueryFlipped: true,
			PrimaryModule:  "stats",
			ExpandModule:   "config",
		}
	}

	return &QueryMetadata{
		IsQueryFlipped: false,
		PrimaryModule:  "config",
		ExpandModule:   "stats",
	}
}

