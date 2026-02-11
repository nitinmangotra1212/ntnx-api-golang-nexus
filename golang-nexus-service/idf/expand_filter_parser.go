/*
 * Expand Query Parser
 * Parses nested OData query options from $expand parameter
 * Examples:
 *   - $expand=associations($filter=entityType eq 'vm')
 *   - $expand=associations($select=entityType,count)
 *   - $expand=associations($orderby=entityType asc)
 */

package idf

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config"
	log "github.com/sirupsen/logrus"
)

// ExpandOptions contains all parsed options from nested expand
type ExpandOptions struct {
	Filter  *ExpandFilter
	Select  *ExpandSelect
	OrderBy *ExpandOrderBy
	// Time-series parameters for stats/metrics
	StartTime        *int64  // Start time in milliseconds (Unix timestamp)
	EndTime          *int64  // End time in milliseconds (Unix timestamp)
	StatType         *string // Aggregation type: AVG, MIN, MAX, LAST, SUM, COUNT
	SamplingInterval *int32  // Sampling interval in seconds
}

// ExpandFilter represents a parsed filter from nested expand
type ExpandFilter struct {
	Field    string // e.g., "entityType"
	Operator string // e.g., "eq"
	Value    string // e.g., "vm"
}

// ExpandSelect represents parsed select fields from nested expand
type ExpandSelect struct {
	Fields []string // e.g., ["entityType", "count"]
}

// ExpandOrderBy represents parsed orderby clause from nested expand
type ExpandOrderBy struct {
	Field     string // e.g., "entityType"
	Direction string // e.g., "asc" or "desc"
}

// ParseExpandOptions extracts all nested options from expand parameter
// Examples:
//   - "associations($filter=entityType eq 'vm')" -> ExpandOptions{Filter: ...}
//   - "associations($select=entityType,count)" -> ExpandOptions{Select: ...}
//   - "associations($orderby=entityType asc)" -> ExpandOptions{OrderBy: ...}
func ParseExpandOptions(expandParam string) *ExpandOptions {
	if expandParam == "" {
		return nil
	}

	options := &ExpandOptions{}

	// Extract filter: associations($filter=entityType eq 'vm')
	// Handle semicolon-separated: associations($filter=...;$select=...)
	// Pattern matches until ; or ) or &
	filterPattern := regexp.MustCompile(`associations\(.*?\$filter=([^;&)]+)[;&)]?`)
	if filterMatches := filterPattern.FindStringSubmatch(expandParam); len(filterMatches) >= 2 {
		filterExpr := strings.TrimSpace(filterMatches[1])
		// Parse simple filter: "entityType eq 'vm'"
		filterPattern := regexp.MustCompile(`(\w+)\s+(eq|ne|gt|ge|lt|le)\s+['"]?([^'"]+)['"]?`)
		if filterMatches := filterPattern.FindStringSubmatch(filterExpr); len(filterMatches) >= 4 {
			options.Filter = &ExpandFilter{
				Field:    filterMatches[1],
				Operator: filterMatches[2],
				Value:    filterMatches[3],
			}
			log.Debugf("Parsed nested filter: %s %s %s", options.Filter.Field, options.Filter.Operator, options.Filter.Value)
		}
	}

	// Extract select: associations($select=entityType,count)
	// Handle semicolon-separated: associations($select=...;$orderby=...)
	// Pattern matches until ; or ) or &
	selectPattern := regexp.MustCompile(`associations\(.*?\$select=([^;&)]+)[;&)]?`)
	if selectMatches := selectPattern.FindStringSubmatch(expandParam); len(selectMatches) >= 2 {
		selectExpr := strings.TrimSpace(selectMatches[1])
		fields := strings.Split(selectExpr, ",")
		// Trim whitespace from each field
		for i, field := range fields {
			fields[i] = strings.TrimSpace(field)
		}
		options.Select = &ExpandSelect{
			Fields: fields,
		}
		log.Debugf("Parsed nested select: %v", options.Select.Fields)
	}

	// Extract orderby: associations($orderby=entityType asc)
	// Handle semicolon-separated: associations($orderby=...;$select=...)
	// Pattern matches until ; or ) or &
	orderbyPattern := regexp.MustCompile(`associations\(.*?\$orderby=([^;&)]+)[;&)]?`)
	if orderbyMatches := orderbyPattern.FindStringSubmatch(expandParam); len(orderbyMatches) >= 2 {
		orderbyExpr := strings.TrimSpace(orderbyMatches[1])
		// Parse: "entityType asc" or "entityType desc" or just "entityType" (defaults to asc)
		orderbyParts := strings.Fields(orderbyExpr)
		if len(orderbyParts) >= 1 {
			direction := "asc"
			if len(orderbyParts) >= 2 {
				direction = strings.ToLower(orderbyParts[1])
				if direction != "asc" && direction != "desc" {
					direction = "asc" // Default to asc if invalid
				}
			}
			options.OrderBy = &ExpandOrderBy{
				Field:     orderbyParts[0],
				Direction: direction,
			}
			log.Debugf("Parsed nested orderby: %s %s", options.OrderBy.Field, options.OrderBy.Direction)
		}
	}

	// Extract time-series parameters for itemStats: itemStats($startTime=...;$endTime=...;$statType=AVG;$samplingInterval=10)
	// These are specific to stats/metrics expand
	if strings.Contains(expandParam, "itemStats") {
		// Extract $startTime: itemStats($startTime=2024-01-01T00:00:00Z)
		startTimePattern := regexp.MustCompile(`itemStats\(.*?\$startTime=([^;&)]+)[;&)]?`)
		if startTimeMatches := startTimePattern.FindStringSubmatch(expandParam); len(startTimeMatches) >= 2 {
			startTimeStr := strings.TrimSpace(startTimeMatches[1])
			// Parse RFC3339 datetime to Unix timestamp (milliseconds)
			if startTimeMs, err := parseDateTimeToMs(startTimeStr); err == nil {
				options.StartTime = &startTimeMs
				log.Infof("✅ [ParseExpandOptions] Parsed $startTime: %s -> %d ms (%s)", startTimeStr, startTimeMs,
					time.Unix(startTimeMs/1000, 0).UTC().Format(time.RFC3339))
			} else {
				log.Warnf("❌ [ParseExpandOptions] Failed to parse $startTime: %s, error: %v", startTimeStr, err)
			}
		}

		// Extract $endTime: itemStats($endTime=2024-01-31T23:59:59Z)
		endTimePattern := regexp.MustCompile(`itemStats\(.*?\$endTime=([^;&)]+)[;&)]?`)
		if endTimeMatches := endTimePattern.FindStringSubmatch(expandParam); len(endTimeMatches) >= 2 {
			endTimeStr := strings.TrimSpace(endTimeMatches[1])
			if endTimeMs, err := parseDateTimeToMs(endTimeStr); err == nil {
				options.EndTime = &endTimeMs
				log.Infof("✅ [ParseExpandOptions] Parsed $endTime: %s -> %d ms (%s)", endTimeStr, endTimeMs,
					time.Unix(endTimeMs/1000, 0).UTC().Format(time.RFC3339))
			} else {
				log.Warnf("❌ [ParseExpandOptions] Failed to parse $endTime: %s, error: %v", endTimeStr, err)
			}
		}

		// Extract $statType: itemStats($statType=AVG)
		statTypePattern := regexp.MustCompile(`itemStats\(.*?\$statType=([^;&)]+)[;&)]?`)
		if statTypeMatches := statTypePattern.FindStringSubmatch(expandParam); len(statTypeMatches) >= 2 {
			statTypeStr := strings.TrimSpace(statTypeMatches[1])
			// Validate statType (AVG, MIN, MAX, LAST, SUM, COUNT)
			validStatTypes := map[string]bool{
				"AVG": true, "MIN": true, "MAX": true, "LAST": true, "SUM": true, "COUNT": true,
			}
			if validStatTypes[strings.ToUpper(statTypeStr)] {
				statTypeUpper := strings.ToUpper(statTypeStr)
				options.StatType = &statTypeUpper
				log.Debugf("Parsed $statType: %s", statTypeUpper)
			} else {
				log.Warnf("Invalid $statType: %s, must be one of: AVG, MIN, MAX, LAST, SUM, COUNT", statTypeStr)
			}
		}

		// Extract $samplingInterval: itemStats($samplingInterval=10)
		samplingIntervalPattern := regexp.MustCompile(`itemStats\(.*?\$samplingInterval=([^;&)]+)[;&)]?`)
		if samplingIntervalMatches := samplingIntervalPattern.FindStringSubmatch(expandParam); len(samplingIntervalMatches) >= 2 {
			samplingIntervalStr := strings.TrimSpace(samplingIntervalMatches[1])
			if samplingInterval, err := strconv.ParseInt(samplingIntervalStr, 10, 32); err == nil {
				samplingInterval32 := int32(samplingInterval)
				options.SamplingInterval = &samplingInterval32
				log.Debugf("Parsed $samplingInterval: %d seconds", samplingInterval32)
			} else {
				log.Warnf("Failed to parse $samplingInterval: %s, error: %v", samplingIntervalStr, err)
			}
		}
	}

	if options.Filter == nil && options.Select == nil && options.OrderBy == nil &&
		options.StartTime == nil && options.EndTime == nil && options.StatType == nil && options.SamplingInterval == nil {
		return nil
	}

	return options
}

// parseDateTimeToMs parses RFC3339 datetime string to Unix timestamp in milliseconds
func parseDateTimeToMs(dateTimeStr string) (int64, error) {
	// Parse RFC3339 format: 2024-01-01T00:00:00Z or 2024-01-01T00:00:00+00:00
	// Use time.Parse with RFC3339 layout
	t, err := time.Parse(time.RFC3339, dateTimeStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse datetime: %w", err)
	}
	// Convert to milliseconds
	return t.UnixMilli(), nil
}

// ParseExpandFilter is kept for backward compatibility
// It now uses ParseExpandOptions internally
func ParseExpandFilter(expandParam string) *ExpandFilter {
	options := ParseExpandOptions(expandParam)
	if options != nil {
		return options.Filter
	}
	return nil
}

// ApplyExpandOptions applies all expand options (filter, select, orderby) to associations
func ApplyExpandOptions(associations []*pb.ItemAssociation, options *ExpandOptions) []*pb.ItemAssociation {
	if options == nil || len(associations) == 0 {
		return associations
	}

	result := associations

	// Step 1: Apply filter
	if options.Filter != nil {
		result = ApplyExpandFilter(result, options.Filter)
	}

	// Step 2: Apply orderby (before select, as we need all fields for sorting)
	if options.OrderBy != nil {
		result = ApplyExpandOrderBy(result, options.OrderBy)
	}

	// Step 3: Apply select (after sorting, to return only selected fields)
	if options.Select != nil {
		result = ApplyExpandSelect(result, options.Select)
	}

	return result
}

// ApplyExpandFilter filters associations based on parsed filter
func ApplyExpandFilter(associations []*pb.ItemAssociation, filter *ExpandFilter) []*pb.ItemAssociation {
	if filter == nil || len(associations) == 0 {
		return associations
	}

	filtered := make([]*pb.ItemAssociation, 0, len(associations))
	for _, assoc := range associations {
		if matchesFilter(assoc, filter) {
			filtered = append(filtered, assoc)
		}
	}

	log.Debugf("Filtered associations: %d -> %d (filter: %s %s %s)", len(associations), len(filtered), filter.Field, filter.Operator, filter.Value)
	return filtered
}

// ApplyExpandSelect selects only specified fields in associations
func ApplyExpandSelect(associations []*pb.ItemAssociation, selectOpt *ExpandSelect) []*pb.ItemAssociation {
	if selectOpt == nil || len(selectOpt.Fields) == 0 || len(associations) == 0 {
		return associations
	}

	// Create a set of allowed fields for fast lookup
	allowedFields := make(map[string]bool)
	for _, field := range selectOpt.Fields {
		allowedFields[strings.ToLower(field)] = true
	}

	// Create new associations with only selected fields
	// IMPORTANT: Always include the association if it has at least one selected field set
	selected := make([]*pb.ItemAssociation, 0, len(associations))
	for _, assoc := range associations {
		newAssoc := &pb.ItemAssociation{}
		hasAnyField := false

		// Only include fields that are in the select list
		if allowedFields["entitytype"] && assoc.EntityType != nil {
			newAssoc.EntityType = assoc.EntityType
			hasAnyField = true
		}
		if allowedFields["entityid"] && assoc.EntityId != nil {
			newAssoc.EntityId = assoc.EntityId
			hasAnyField = true
		}
		if allowedFields["count"] && assoc.Count != nil {
			newAssoc.Count = assoc.Count
			hasAnyField = true
		}
		if allowedFields["itemid"] && assoc.ItemId != nil {
			newAssoc.ItemId = assoc.ItemId
			hasAnyField = true
		}

		// Only include associations that have at least one selected field set
		// This prevents empty associations from being included
		if hasAnyField {
			selected = append(selected, newAssoc)
		} else {
			log.Debugf("Skipping association with no selected fields: %+v", assoc)
		}
	}

	log.Debugf("Selected fields in associations: %v, result: %d associations (from %d)", selectOpt.Fields, len(selected), len(associations))
	return selected
}

// ApplyExpandOrderBy sorts associations based on orderby clause
func ApplyExpandOrderBy(associations []*pb.ItemAssociation, orderBy *ExpandOrderBy) []*pb.ItemAssociation {
	if orderBy == nil || len(associations) == 0 {
		return associations
	}

	// Create a copy to avoid modifying original
	sorted := make([]*pb.ItemAssociation, len(associations))
	copy(sorted, associations)

	// Sort based on field and direction
	sort.Slice(sorted, func(i, j int) bool {
		fieldLower := strings.ToLower(orderBy.Field)

		// Handle numeric fields (count) separately
		if fieldLower == "count" {
			countI := int32(0)
			countJ := int32(0)
			if sorted[i].Count != nil {
				countI = *sorted[i].Count
			}
			if sorted[j].Count != nil {
				countJ = *sorted[j].Count
			}

			if orderBy.Direction == "desc" {
				return countI > countJ
			}
			return countI < countJ
		}

		// Handle string fields
		valI := getFieldValue(sorted[i], orderBy.Field)
		valJ := getFieldValue(sorted[j], orderBy.Field)

		if orderBy.Direction == "desc" {
			return valI > valJ
		}
		return valI < valJ
	})

	log.Debugf("Sorted associations by: %s %s", orderBy.Field, orderBy.Direction)
	return sorted
}

// getFieldValue extracts the value of a field for comparison (string fields only)
func getFieldValue(assoc *pb.ItemAssociation, field string) string {
	fieldLower := strings.ToLower(field)
	switch fieldLower {
	case "entitytype":
		if assoc.EntityType != nil {
			return *assoc.EntityType
		}
		return ""
	case "entityid":
		if assoc.EntityId != nil {
			return *assoc.EntityId
		}
		return ""
	case "itemid":
		if assoc.ItemId != nil {
			return *assoc.ItemId
		}
		return ""
	default:
		return ""
	}
}

// matchesFilter checks if an association matches the filter criteria
func matchesFilter(assoc *pb.ItemAssociation, filter *ExpandFilter) bool {
	switch filter.Field {
	case "entityType":
		if assoc.EntityType == nil {
			return false
		}
		return compareValues(*assoc.EntityType, filter.Operator, filter.Value)

	case "entityId":
		if assoc.EntityId == nil {
			return false
		}
		return compareValues(*assoc.EntityId, filter.Operator, filter.Value)

	case "count":
		if assoc.Count == nil {
			return false
		}
		return compareInt32(*assoc.Count, filter.Operator, filter.Value)

	case "itemId":
		if assoc.ItemId == nil {
			return false
		}
		return compareValues(*assoc.ItemId, filter.Operator, filter.Value)

	default:
		log.Warnf("Unknown filter field: %s", filter.Field)
		return true // Include by default if field unknown
	}
}

// compareValues compares string values based on operator
func compareValues(actual string, operator string, expected string) bool {
	switch operator {
	case "eq":
		return actual == expected
	case "ne":
		return actual != expected
	case "gt":
		return actual > expected
	case "ge":
		return actual >= expected
	case "lt":
		return actual < expected
	case "le":
		return actual <= expected
	default:
		log.Warnf("Unknown operator: %s", operator)
		return true
	}
}

// compareInt32 compares int32 values based on operator
func compareInt32(actual int32, operator string, expectedStr string) bool {
	// Parse expected value as int32
	expected, err := strconv.ParseInt(expectedStr, 10, 32)
	if err != nil {
		log.Warnf("Could not parse filter value as int32: %s", expectedStr)
		return true
	}
	expectedInt32 := int32(expected)

	switch operator {
	case "eq":
		return actual == expectedInt32
	case "ne":
		return actual != expectedInt32
	case "gt":
		return actual > expectedInt32
	case "ge":
		return actual >= expectedInt32
	case "lt":
		return actual < expectedInt32
	case "le":
		return actual <= expectedInt32
	default:
		log.Warnf("Unknown operator: %s", operator)
		return true
	}
}
