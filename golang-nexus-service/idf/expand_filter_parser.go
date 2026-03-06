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
	"strconv"
	"strings"
	"time"

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

	// Generic pattern: match any entity name (associations, itemStats, etc.)
	// \w+ matches the entity name before the parentheses
	entityPattern := `\w+`

	// Extract filter: entity($filter=field eq 'value')
	filterPattern := regexp.MustCompile(entityPattern + `\(.*?\$filter=([^;&)]+)[;&)]?`)
	if filterMatches := filterPattern.FindStringSubmatch(expandParam); len(filterMatches) >= 2 {
		filterExpr := strings.TrimSpace(filterMatches[1])
		filterExprPattern := regexp.MustCompile(`(\w+)\s+(eq|ne|gt|ge|lt|le)\s+['"]?([^'"]+)['"]?`)
		if filterExprMatches := filterExprPattern.FindStringSubmatch(filterExpr); len(filterExprMatches) >= 4 {
			options.Filter = &ExpandFilter{
				Field:    filterExprMatches[1],
				Operator: filterExprMatches[2],
				Value:    filterExprMatches[3],
			}
			log.Debugf("Parsed nested filter: %s %s %s", options.Filter.Field, options.Filter.Operator, options.Filter.Value)
		}
	}

	// Extract select: entity($select=field1,field2)
	selectPattern := regexp.MustCompile(entityPattern + `\(.*?\$select=([^;&)]+)[;&)]?`)
	if selectMatches := selectPattern.FindStringSubmatch(expandParam); len(selectMatches) >= 2 {
		selectExpr := strings.TrimSpace(selectMatches[1])
		fields := strings.Split(selectExpr, ",")
		for i, field := range fields {
			fields[i] = strings.TrimSpace(field)
		}
		options.Select = &ExpandSelect{
			Fields: fields,
		}
		log.Debugf("Parsed nested select: %v", options.Select.Fields)
	}

	// Extract orderby: entity($orderby=field asc)
	orderbyPattern := regexp.MustCompile(entityPattern + `\(.*?\$orderby=([^;&)]+)[;&)]?`)
	if orderbyMatches := orderbyPattern.FindStringSubmatch(expandParam); len(orderbyMatches) >= 2 {
		orderbyExpr := strings.TrimSpace(orderbyMatches[1])
		orderbyParts := strings.Fields(orderbyExpr)
		if len(orderbyParts) >= 1 {
			direction := "asc"
			if len(orderbyParts) >= 2 {
				direction = strings.ToLower(orderbyParts[1])
				if direction != "asc" && direction != "desc" {
					direction = "asc"
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

// Client-side filtering/select/orderby for expanded entities has been removed.
// Nested OData options ($filter, $select, $orderby) within $expand are passed
// through the OData library to the backend (IDF/GraphQL) for server-side handling,
// following the same pattern as the categories service.

