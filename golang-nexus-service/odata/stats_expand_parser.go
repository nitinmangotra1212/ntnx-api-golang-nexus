/*
 * Stats Expansion Parser for Config+Stats Feature
 * Parses nested OData parameters including time-series params from $expand
 *
 * Supports parsing:
 *   - $startTime, $endTime (ISO 8601 timestamps)
 *   - $samplingInterval (seconds)
 *   - $statType (AVG, SUM, MIN, MAX, etc)
 *   - $filter, $select, $orderby, $limit, $page within expansion
 *
 * Example:
 *   $expand=catStats($startTime=2025-10-28T13:08:00.000Z;$endTime=2025-10-28T14:08:00.000Z;
 *           $statType=AVG;$samplingInterval=30;$select=heartRate,weight;$filter=heartRate gt 70;
 *           $orderby=stats/heartRate desc)
 *
 * Based on Confluence: "Fetch Stats and Config Data Together using V4 APIs"
 */

package odata

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// StatsExpandParams contains all parsed parameters from stats expansion
type StatsExpandParams struct {
	// Time-series parameters
	StartTime        *time.Time // Parsed from ISO 8601 format
	EndTime          *time.Time
	SamplingInterval *int64 // In seconds
	StatType         string // AVG, SUM, MIN, MAX, etc

	// Standard OData parameters within expansion
	Filter  string
	Select  []string
	OrderBy *StatsOrderBy
	Limit   int
	Page    int

	// Query flipping metadata
	IsQueryFlipped    bool   // True if sorting by stats attribute
	FlippedOrderBy    string // Original orderby value that caused flip
	PrimaryEntityKey  string // e.g., "_entity_id_"
	SecondaryEntityKey string // e.g., "_entity_id_"

	// Expansion target info
	ExpansionKey       string // e.g., "catStats", "vmStats"
	LeftEntityKey      string // e.g., "extId"
	RightEntityKey     string // e.g., "catId"
	RightModel         string // e.g., "CatStats"
	MappingType        string // ONE_TO_ONE, ONE_TO_MANY
}

// StatsOrderBy represents ordering for stats
type StatsOrderBy struct {
	Field     string // e.g., "stats/heartRate" or "heartRate"
	Direction string // "asc" or "desc"
	IsStatsField bool // True if field is prefixed with "stats/"
}

// ParseStatsExpand parses the $expand parameter for stats expansion
// Handles both semicolon (;) and ampersand (&) as separators
func ParseStatsExpand(expandParam string) *StatsExpandParams {
	if expandParam == "" {
		return nil
	}

	params := &StatsExpandParams{
		Limit: 50, // Default limit
		Page:  0,
	}

	// Extract expansion key (e.g., "catStats" from "catStats($startTime=...)")
	keyPattern := regexp.MustCompile(`^(\w+)\(`)
	if keyMatches := keyPattern.FindStringSubmatch(expandParam); len(keyMatches) >= 2 {
		params.ExpansionKey = keyMatches[1]
	} else {
		// Simple expand without options (e.g., "$expand=stats")
		params.ExpansionKey = strings.TrimSpace(expandParam)
		return params
	}

	// Extract everything inside the parentheses
	optionsPattern := regexp.MustCompile(`\w+\((.+)\)$`)
	optionsMatches := optionsPattern.FindStringSubmatch(expandParam)
	if len(optionsMatches) < 2 {
		return params
	}
	optionsStr := optionsMatches[1]

	// Parse individual options (separated by ; or &)
	// Split by ; first, then handle each option
	options := splitExpandOptions(optionsStr)

	for key, value := range options {
		switch strings.ToLower(key) {
		case "$starttime", "starttime":
			if t, err := parseISO8601(value); err == nil {
				params.StartTime = &t
				log.Debugf("Parsed $startTime: %v", t)
			} else {
				log.Warnf("Failed to parse $startTime: %s, error: %v", value, err)
			}

		case "$endtime", "endtime":
			if t, err := parseISO8601(value); err == nil {
				params.EndTime = &t
				log.Debugf("Parsed $endTime: %v", t)
			} else {
				log.Warnf("Failed to parse $endTime: %s, error: %v", value, err)
			}

		case "$samplinginterval", "samplinginterval":
			if interval, err := strconv.ParseInt(value, 10, 64); err == nil {
				params.SamplingInterval = &interval
				log.Debugf("Parsed $samplingInterval: %d seconds", interval)
			}

		case "$stattype", "stattype":
			params.StatType = strings.ToUpper(value)
			log.Debugf("Parsed $statType: %s", params.StatType)

		case "$filter", "filter":
			params.Filter = value
			// Check if filter contains "stats/any" pattern (filter on stats attribute)
			if strings.Contains(value, "stats/any") || strings.Contains(value, "stats/") {
				log.Debugf("Detected stats filter: %s", value)
			}
			log.Debugf("Parsed nested $filter: %s", value)

		case "$select", "select":
			params.Select = parseSelectFields(value)
			log.Debugf("Parsed nested $select: %v", params.Select)

		case "$orderby", "orderby":
			params.OrderBy = parseStatsOrderBy(value)
			// Check if ordering by stats attribute - this triggers query flipping
			if params.OrderBy != nil && params.OrderBy.IsStatsField {
				params.IsQueryFlipped = true
				params.FlippedOrderBy = value
				log.Infof("Query flipping detected: ordering by stats field %s", params.OrderBy.Field)
			}
			log.Debugf("Parsed nested $orderby: %+v", params.OrderBy)

		case "$limit", "limit":
			if limit, err := strconv.Atoi(value); err == nil {
				params.Limit = limit
				log.Debugf("Parsed nested $limit: %d", limit)
			}

		case "$page", "page":
			if page, err := strconv.Atoi(value); err == nil {
				params.Page = page
				log.Debugf("Parsed nested $page: %d", page)
			}
		}
	}

	return params
}

// splitExpandOptions splits the options string by semicolons or ampersands
// Returns a map of option key -> value
func splitExpandOptions(optionsStr string) map[string]string {
	options := make(map[string]string)

	// Use regex to split by ; or & but not inside quotes
	// Handle patterns like: $startTime=2025-10-28T13:08:00.000Z;$endTime=...
	parts := regexp.MustCompile(`[;&]`).Split(optionsStr, -1)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split by = to get key and value
		idx := strings.Index(part, "=")
		if idx > 0 {
			key := strings.TrimSpace(part[:idx])
			value := strings.TrimSpace(part[idx+1:])
			options[key] = value
		}
	}

	return options
}

// parseISO8601 parses an ISO 8601 timestamp string
func parseISO8601(timeStr string) (time.Time, error) {
	// Try various ISO 8601 formats
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000-07:00",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("failed to parse timestamp: %s", timeStr)
}

// parseSelectFields parses comma-separated select fields
func parseSelectFields(selectStr string) []string {
	if selectStr == "" {
		return nil
	}

	fields := strings.Split(selectStr, ",")
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			result = append(result, field)
		}
	}
	return result
}

// parseStatsOrderBy parses the orderby clause and detects stats field ordering
// Examples:
//   - "stats/heartRate desc" -> IsStatsField=true, Field="heartRate"
//   - "heartRate asc" -> IsStatsField=false, Field="heartRate"
func parseStatsOrderBy(orderByStr string) *StatsOrderBy {
	if orderByStr == "" {
		return nil
	}

	orderBy := &StatsOrderBy{
		Direction: "asc", // Default direction
	}

	parts := strings.Fields(orderByStr)
	if len(parts) == 0 {
		return nil
	}

	fieldPart := parts[0]

	// Check if field is prefixed with "stats/"
	if strings.HasPrefix(fieldPart, "stats/") {
		orderBy.IsStatsField = true
		orderBy.Field = strings.TrimPrefix(fieldPart, "stats/")
	} else {
		orderBy.Field = fieldPart
	}

	// Parse direction if present
	if len(parts) >= 2 {
		dir := strings.ToLower(parts[1])
		if dir == "desc" || dir == "asc" {
			orderBy.Direction = dir
		}
	}

	return orderBy
}

// GetTimeRangeMs returns the time range in milliseconds for GraphQL query
// Returns (startMs, endMs) as microseconds (StatsGW uses microseconds)
func (p *StatsExpandParams) GetTimeRangeMs() (int64, int64) {
	var startMs, endMs int64

	if p.StartTime != nil {
		startMs = p.StartTime.UnixMicro()
	} else {
		// Default: 1 hour ago
		startMs = time.Now().Add(-1 * time.Hour).UnixMicro()
	}

	if p.EndTime != nil {
		endMs = p.EndTime.UnixMicro()
	} else {
		// Default: now
		endMs = time.Now().UnixMicro()
	}

	return startMs, endMs
}

// GetSamplingIntervalSecs returns the sampling interval in seconds
func (p *StatsExpandParams) GetSamplingIntervalSecs() int64 {
	if p.SamplingInterval != nil {
		return *p.SamplingInterval
	}
	// Default: 60 seconds
	return 60
}

// GetStatTypeSampling returns the stats type for GraphQL query (e.g., "AVG", "SUM")
func (p *StatsExpandParams) GetStatTypeSampling() string {
	if p.StatType != "" {
		return p.StatType
	}
	return "AVG" // Default
}

// ShouldFlipQuery returns true if the query should be flipped
// Query flipping is needed when ordering by stats attributes
func (p *StatsExpandParams) ShouldFlipQuery() bool {
	return p.IsQueryFlipped && p.OrderBy != nil && p.OrderBy.IsStatsField
}

// GetExpandFieldNames returns multiple expand field names if comma-separated
// Example: "stats,petFood,petCare" -> ["stats", "petFood", "petCare"]
func GetExpandFieldNames(expandParam string) []string {
	if expandParam == "" {
		return nil
	}

	// First check if it's a complex expand with parentheses
	if strings.Contains(expandParam, "(") {
		// Extract just the field name before parentheses
		// e.g., "catStats($startTime=...)" -> "catStats"
		keyPattern := regexp.MustCompile(`^(\w+)\(`)
		if keyMatches := keyPattern.FindStringSubmatch(expandParam); len(keyMatches) >= 2 {
			return []string{keyMatches[1]}
		}
	}

	// Simple comma-separated expand
	fields := strings.Split(expandParam, ",")
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			result = append(result, field)
		}
	}
	return result
}

// IsStatsExpansion checks if the expand parameter is for stats
// Based on the expansion key name (e.g., "stats", "vmStats", "catStats")
func IsStatsExpansion(expandKey string) bool {
	key := strings.ToLower(expandKey)
	return strings.HasSuffix(key, "stats") || key == "stats"
}

