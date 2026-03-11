/*
 * IDF Repository Implementation for ItemStats Entity (Stats Module)
 * Maps between protobuf ItemStats model (camelCase) and IDF attributes (snake_case)
 */

package idf

import (
	"fmt"
	"strings"

	"github.com/nutanix-core/go-cache/insights/insights_interface"
	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/stats" // Note: stats protobuf
	"github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/response"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/db"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/external"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/models"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

type ItemStatsRepositoryImpl struct{}

// IDF Column Names for item_stats (snake_case)
const (
	itemStatsEntityTypeName = "item_stats"
	itemStatsListPath       = "/item-stats"
)

func NewItemStatsRepository() db.ItemStatsRepository {
	return &ItemStatsRepositoryImpl{}
}

// ListItemStats retrieves a list of item stats from IDF
// Uses OData parser to handle $filter, $orderby, $select, $expand
// Note: GroupBy queries ($apply) should use ListItemStatsWithGroupBy instead
func (r *ItemStatsRepositoryImpl) ListItemStats(queryParams *models.QueryParams) ([]*pb.ItemStats, int64, error) {
	// Don't handle GroupBy here - the gRPC service will call ListItemStatsWithGroupBy
	if queryParams.Apply != "" {
		log.Warnf("ListItemStats called with $apply - should use ListItemStatsWithGroupBy instead")
		// Return empty result - gRPC service should handle this
		return []*pb.ItemStats{}, 0, nil
	}

	// Use OData parser to generate IDF query
	queryArg, err := GenerateListQuery(queryParams, itemStatsListPath, itemStatsEntityTypeName, "item_ext_id")
	if err != nil {
		log.Errorf("Failed to generate IDF query from OData params: %v", err)
		return nil, 0, fmt.Errorf("failed to parse OData query: %w", err)
	}

	// Query IDF
	idfClient := external.Interfaces().IdfClient()
	queryResponse, err := idfClient.GetEntitiesWithMetricsRet(queryArg)
	if err != nil {
		log.Errorf("Failed to query IDF: %v", err)
		return nil, 0, err
	}

	// Convert IDF entities to ItemStats protobufs
	groupResults := queryResponse.GetGroupResultsList()
	if len(groupResults) == 0 {
		return []*pb.ItemStats{}, 0, nil
	}

	var stats []*pb.ItemStats
	entitiesWithMetric := groupResults[0].GetRawResults()
	entities := ConvertEntitiesWithMetricToEntities(entitiesWithMetric)
	for _, entity := range entities {
		stat := r.mapIdfAttributeToItemStats(entity)
		stats = append(stats, stat)
	}

	totalCount := groupResults[0].GetTotalEntityCount()
	log.Infof("✅ Retrieved %d item stats from IDF (total: %d)", len(stats), totalCount)

	return stats, totalCount, nil
}

// ListItemStatsWithGroupBy handles GroupBy queries for stats module
// Returns ItemStatsGroup objects with group keys and aggregated data
func (r *ItemStatsRepositoryImpl) ListItemStatsWithGroupBy(queryParams *models.QueryParams) ([]*pb.ItemStatsGroup, int64, error) {
	log.Infof("Executing GroupBy query for stats module with $apply: %s", queryParams.Apply)

	// Use OData parser to generate IDF query (handles $apply via IDFApplyEvaluator)
	queryArg, err := GenerateListQuery(queryParams, itemStatsListPath, itemStatsEntityTypeName, "item_ext_id")
	if err != nil {
		log.Errorf("Failed to generate IDF GroupBy query from OData params: %v", err)
		return nil, 0, fmt.Errorf("failed to parse $apply query: %w", err)
	}

	// Query IDF with GroupBy
	idfClient := external.Interfaces().IdfClient()
	queryResponse, err := idfClient.GetEntitiesWithMetricsRet(queryArg)
	if err != nil {
		log.Errorf("Failed to execute GroupBy query in IDF: %v", err)
		return nil, 0, err
	}

	// Convert grouped results to ItemStatsGroup objects
	groupResults := queryResponse.GetGroupResultsList()
	if len(groupResults) == 0 {
		log.Infof("No grouped results returned from IDF")
		return []*pb.ItemStatsGroup{}, 0, nil
	}

	var itemGroups []*pb.ItemStatsGroup

	// Process each group result
	for _, groupResult := range groupResults {
		entityCount := groupResult.GetTotalEntityCount()

		// Get entities in this group
		entitiesWithMetric := groupResult.GetRawResults()
		entities := ConvertEntitiesWithMetricToEntities(entitiesWithMetric)

		if len(entities) == 0 {
			log.Warnf("Group result has no entities, skipping")
			continue
		}

		// Extract group key from GroupByColumnValue (group-level value from IDF)
		groupKey := r.buildGroupKeyFromValue(groupResult.GetGroupByColumnValue(), queryParams.Apply)
		if groupKey == nil {
			log.Warnf("Failed to build group key from GroupByColumnValue, skipping group")
			continue
		}

		// Convert entities to ItemStats.
		// Per-group entity limit is enforced by RawLimit in the IDF query.
		// Max-limit validation is handled by the dev-platform.
		var stats []*pb.ItemStats
		for _, entity := range entities {
			stat := r.mapIdfAttributeToItemStats(entity)
			stats = append(stats, stat)
		}

		// Extract aggregate results from IDF GroupSummaries
		aggregates := r.buildItemStatsAggregatesFromIDF(groupResult)
		var aggregatesWrapper *pb.ItemStatsAggregateArrayWrapper
		if len(aggregates) > 0 {
			aggregatesWrapper = &pb.ItemStatsAggregateArrayWrapper{
				Value: aggregates,
			}
		}

		// Create ItemStatsGroup with group key, stats, aggregates, and per-group metadata
		itemGroup := &pb.ItemStatsGroup{
			Data: &pb.ItemStatsGroup_ItemStatsArrayData{
				ItemStatsArrayData: &pb.ItemStatsArrayWrapper{
					Value: stats,
				},
			},
			Aggregates: aggregatesWrapper,
			Metadata: &response.ApiResponseMetadata{
				TotalAvailableResults: proto.Int32(int32(entityCount)),
			},
		}

		// Set the group key based on type
		switch v := groupKey.(type) {
		case *pb.ItemStatsGroup_StringGroup:
			itemGroup.Group = v
		case *pb.ItemStatsGroup_Int32Group:
			itemGroup.Group = v
		case *pb.ItemStatsGroup_Int64Group:
			itemGroup.Group = v
		case *pb.ItemStatsGroup_DoubleGroup:
			itemGroup.Group = v
		case *pb.ItemStatsGroup_BooleanGroup:
			itemGroup.Group = v
		default:
			log.Warnf("Unknown group key type: %T", v)
			continue
		}

		itemGroups = append(itemGroups, itemGroup)
	}

	// Outer totalAvailableResults = total number of groups.
	// Use IDF's TotalGroupCount when available, otherwise fall back to len(itemGroups).
	totalCount := queryResponse.GetTotalGroupCount()
	if totalCount == 0 {
		totalCount = int64(len(itemGroups))
	}

	log.Infof("✅ Retrieved %d ItemStatsGroups from GroupBy query (totalAvailableResults: %d groups)",
		len(itemGroups), totalCount)

	return itemGroups, totalCount, nil
}

// oDataToIdfStatsGroupField maps OData camelCase property names to their IDF attribute name and value type.
var oDataToIdfStatsGroupField = map[string]struct {
	idfAttr   string
	valueType string
}{
	"itemExtId":  {"item_ext_id", "string"},
	"age":        {"age", "int32"},
	"heartRate":  {"heart_rate", "int32"},
	"foodIntake": {"food_intake", "double"},
	"timestamp":  {"timestamp", "int64"},
	"speed":      {"speed", "int64"},
}

// extractGroupKey extracts the group key from an entity based on the $apply parameter.
func (r *ItemStatsRepositoryImpl) extractGroupKey(entity *insights_interface.Entity, applyParam string) interface{} {
	var fieldInfo struct {
		idfAttr   string
		valueType string
	}
	found := false
	for odataName, info := range oDataToIdfStatsGroupField {
		if strings.Contains(applyParam, odataName) {
			fieldInfo = info
			found = true
			break
		}
	}
	if !found {
		log.Warnf("Could not find groupby field mapping for ItemStats applyParam: %s", applyParam)
		return nil
	}

	for _, attr := range entity.GetAttributeDataMap() {
		if attr.GetName() != fieldInfo.idfAttr || attr.GetValue() == nil {
			continue
		}
		switch fieldInfo.valueType {
		case "string":
			return &pb.ItemStatsGroup_StringGroup{
				StringGroup: &pb.StringWrapper{
					Value: proto.String(attr.GetValue().GetStrValue()),
				},
			}
		case "int32":
			return &pb.ItemStatsGroup_Int32Group{
				Int32Group: &pb.Int32Wrapper{
					Value: proto.Int32(int32(attr.GetValue().GetInt64Value())),
				},
			}
		case "int64":
			return &pb.ItemStatsGroup_Int64Group{
				Int64Group: &pb.Int64Wrapper{
					Value: proto.Int64(attr.GetValue().GetInt64Value()),
				},
			}
		case "double":
			return &pb.ItemStatsGroup_DoubleGroup{
				DoubleGroup: &pb.DoubleWrapper{
					Value: proto.Float64(attr.GetValue().GetDoubleValue()),
				},
			}
		case "boolean":
			return &pb.ItemStatsGroup_BooleanGroup{
				BooleanGroup: &pb.BooleanWrapper{
					Value: proto.Bool(attr.GetValue().GetBoolValue()),
				},
			}
		}
	}

	log.Warnf("Could not extract group key for IDF attr %s from ItemStats entity", fieldInfo.idfAttr)
	return nil
}

// buildGroupKeyFromValue builds the group key from IDF's QueryGroupResult.GroupByColumnValue.
func (r *ItemStatsRepositoryImpl) buildGroupKeyFromValue(dataValue *insights_interface.DataValue, applyParam string) interface{} {
	if dataValue == nil {
		log.Warnf("GroupByColumnValue is nil for ItemStats applyParam: %s", applyParam)
		return nil
	}

	var fieldInfo struct {
		idfAttr   string
		valueType string
	}
	found := false
	for odataName, info := range oDataToIdfStatsGroupField {
		if strings.Contains(applyParam, odataName) {
			fieldInfo = info
			found = true
			break
		}
	}
	if !found {
		log.Warnf("Could not find groupby field mapping for ItemStats applyParam: %s", applyParam)
		return nil
	}

	switch fieldInfo.valueType {
	case "string":
		return &pb.ItemStatsGroup_StringGroup{
			StringGroup: &pb.StringWrapper{Value: proto.String(dataValue.GetStrValue())},
		}
	case "int32":
		return &pb.ItemStatsGroup_Int32Group{
			Int32Group: &pb.Int32Wrapper{Value: proto.Int32(int32(dataValue.GetInt64Value()))},
		}
	case "int64":
		return &pb.ItemStatsGroup_Int64Group{
			Int64Group: &pb.Int64Wrapper{Value: proto.Int64(dataValue.GetInt64Value())},
		}
	case "double":
		return &pb.ItemStatsGroup_DoubleGroup{
			DoubleGroup: &pb.DoubleWrapper{Value: proto.Float64(dataValue.GetDoubleValue())},
		}
	case "boolean":
		return &pb.ItemStatsGroup_BooleanGroup{
			BooleanGroup: &pb.BooleanWrapper{Value: proto.Bool(dataValue.GetBoolValue())},
		}
	}

	log.Warnf("Unsupported group value type for ItemStats field %s", fieldInfo.idfAttr)
	return nil
}

// mapIdfAttributeToItemStats maps IDF attributes (snake_case) to protobuf ItemStats (camelCase)
func (r *ItemStatsRepositoryImpl) mapIdfAttributeToItemStats(entity *insights_interface.Entity) *pb.ItemStats {
	stat := &pb.ItemStats{}

	for _, attr := range entity.GetAttributeDataMap() {
		switch attr.GetName() {
		case "item_ext_id":
			if attr.GetValue() != nil {
				val := attr.GetValue().GetStrValue()
				if val != "" {
					stat.ItemExtId = &val
					log.Debugf("  Mapped item_ext_id: %s", val)
				}
			}
		case "age":
			// Note: Time-series metrics are now arrays of time-value pairs
			// This function receives Entity (not EntityWithMetric), so we can't extract all time-series values here
			// For now, leave as nil - the stats module endpoint may need to be updated to use EntityWithMetric
			// TODO: Update stats module to use EntityWithMetric to get all time-series values with timestamps
			if attr.GetValue() != nil {
				if intVal := attr.GetValue().GetInt64Value(); intVal != 0 {
					log.Debugf("  Skipped age: %d (time-series metrics require EntityWithMetric for timestamps)", intVal)
				}
			}
		case "heart_rate":
			// Note: Time-series metrics are now arrays of time-value pairs
			// This function receives Entity (not EntityWithMetric), so we can't extract all time-series values here
			// For now, leave as nil - the stats module endpoint may need to be updated to use EntityWithMetric
			if attr.GetValue() != nil {
				if intVal := attr.GetValue().GetInt64Value(); intVal != 0 {
					log.Debugf("  Skipped heart_rate: %d (time-series metrics require EntityWithMetric for timestamps)", intVal)
				}
			}
		case "food_intake":
			// Note: Time-series metrics are now arrays of time-value pairs
			// This function receives Entity (not EntityWithMetric), so we can't extract all time-series values here
			// For now, leave as nil - the stats module endpoint may need to be updated to use EntityWithMetric
			if attr.GetValue() != nil {
				if doubleVal := attr.GetValue().GetDoubleValue(); doubleVal != 0 {
					log.Debugf("  Skipped food_intake: %f (time-series metrics require EntityWithMetric for timestamps)", doubleVal)
				}
			}
		}
	}

	return stat
}

// buildItemStatsAggregatesFromIDF creates ItemStatsAggregate objects from IDF GroupSummaries.
// Mirrors the pattern in buildItemAggregatesFromIDF for the config module.
func (r *ItemStatsRepositoryImpl) buildItemStatsAggregatesFromIDF(groupResult *insights_interface.QueryGroupResult) []*pb.ItemStatsAggregate {
	aggregates := make([]*pb.ItemStatsAggregate, 0)
	groupSummaries := groupResult.GetGroupSummaries()
	if len(groupSummaries) == 0 {
		return aggregates
	}

	log.Infof("Processing %d GroupSummaries for stats aggregates", len(groupSummaries))
	for _, groupSummary := range groupSummaries {
		summaryData := groupSummary.GetSummaryData()
		if summaryData == nil {
			continue
		}

		aggregate := &pb.ItemStatsAggregate{
			Label: proto.String(summaryData.GetName()),
		}

		if len(summaryData.GetValueList()) > 0 {
			firstValue := summaryData.GetValueList()[0]
			if firstValue.GetValue() != nil {
				val := firstValue.GetValue()
				if intVal := val.GetInt64Value(); intVal != 0 {
					aggregate.Result = &pb.ItemStatsAggregate_Int64Result{
						Int64Result: &pb.Int64Wrapper{
							Value: proto.Int64(intVal),
						},
					}
				} else if doubleVal := val.GetDoubleValue(); doubleVal != 0 {
					aggregate.Result = &pb.ItemStatsAggregate_DoubleResult{
						DoubleResult: &pb.DoubleWrapper{
							Value: proto.Float64(doubleVal),
						},
					}
				} else {
					aggregate.Result = &pb.ItemStatsAggregate_Int64Result{
						Int64Result: &pb.Int64Wrapper{
							Value: proto.Int64(0),
						},
					}
				}
			}
		}

		aggregates = append(aggregates, aggregate)
	}

	log.Infof("Built %d ItemStatsAggregate objects from GroupSummaries", len(aggregates))
	return aggregates
}