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
	var totalCount int64

	// Process each group result
	for _, groupResult := range groupResults {
		// Get entities in this group
		entitiesWithMetric := groupResult.GetRawResults()
		entities := ConvertEntitiesWithMetricToEntities(entitiesWithMetric)

		if len(entities) == 0 {
			log.Warnf("Group result has no entities, skipping")
			continue
		}

		// Extract group key from first entity
		firstEntity := entities[0]
		groupKey := r.extractGroupKey(firstEntity, queryParams.Apply)
		if groupKey == nil {
			log.Warnf("Failed to extract group key from first entity, skipping group")
			continue
		}

		// Convert entities to ItemStats
		var stats []*pb.ItemStats
		for _, entity := range entities {
			stat := r.mapIdfAttributeToItemStats(entity)
			stats = append(stats, stat)
		}

		// Create ItemStatsGroup with group key and stats
		itemGroup := &pb.ItemStatsGroup{
			Data: &pb.ItemStatsGroup_ItemStatsArrayData{
				ItemStatsArrayData: &pb.ItemStatsArrayWrapper{
					Value: stats,
				},
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
		totalCount += groupResult.GetTotalEntityCount()
	}

	log.Infof("✅ Retrieved %d ItemStatsGroups from GroupBy query (total: %d groups, %d total items)",
		len(itemGroups), len(groupResults), totalCount)

	return itemGroups, totalCount, nil
}

// extractGroupKey extracts the group key from an entity based on the $apply parameter
func (r *ItemStatsRepositoryImpl) extractGroupKey(entity *insights_interface.Entity, applyParam string) interface{} {
	if strings.Contains(applyParam, "itemExtId") {
		for _, attr := range entity.GetAttributeDataMap() {
			if attr.GetName() == "item_ext_id" {
				if attr.GetValue() != nil {
					val := attr.GetValue().GetStrValue()
					return &pb.ItemStatsGroup_StringGroup{
						StringGroup: &pb.StringWrapper{
							Value: proto.String(val),
						},
					}
				}
			}
		}
	} else if strings.Contains(applyParam, "age") {
		for _, attr := range entity.GetAttributeDataMap() {
			if attr.GetName() == "age" {
				if attr.GetValue() != nil {
					val := int32(attr.GetValue().GetInt64Value())
					return &pb.ItemStatsGroup_Int32Group{
						Int32Group: &pb.Int32Wrapper{
							Value: proto.Int32(val),
						},
					}
				}
			}
		}
	} else if strings.Contains(applyParam, "heartRate") {
		for _, attr := range entity.GetAttributeDataMap() {
			if attr.GetName() == "heart_rate" {
				if attr.GetValue() != nil {
					val := int32(attr.GetValue().GetInt64Value())
					return &pb.ItemStatsGroup_Int32Group{
						Int32Group: &pb.Int32Wrapper{
							Value: proto.Int32(val),
						},
					}
				}
			}
		}
	} else if strings.Contains(applyParam, "foodIntake") {
		for _, attr := range entity.GetAttributeDataMap() {
			if attr.GetName() == "food_intake" {
				if attr.GetValue() != nil {
					val := attr.GetValue().GetDoubleValue()
					return &pb.ItemStatsGroup_DoubleGroup{
						DoubleGroup: &pb.DoubleWrapper{
							Value: proto.Float64(val),
						},
					}
				}
			}
		}
	}

	log.Warnf("Could not extract group key from applyParam for ItemStats: %s", applyParam)
	return nil
}

// mapIdfAttributeToItemStats maps IDF attributes (snake_case) to protobuf ItemStats (camelCase)
func (r *ItemStatsRepositoryImpl) mapIdfAttributeToItemStats(entity *insights_interface.Entity) *pb.ItemStats {
	stat := &pb.ItemStats{}

	// Get statsExtId from EntityGuid if available
	if entity.GetEntityGuid() != nil && entity.GetEntityGuid().GetEntityId() != "" {
		extId := entity.GetEntityGuid().GetEntityId()
		stat.StatsExtId = &extId
		log.Debugf("  Set statsExtId from EntityGuid: %s", extId)
	}

	for _, attr := range entity.GetAttributeDataMap() {
		switch attr.GetName() {
		case "stats_ext_id":
			if attr.GetValue() != nil {
				val := attr.GetValue().GetStrValue()
				if val != "" {
					stat.StatsExtId = &val
					log.Debugf("  Mapped stats_ext_id: %s", val)
				}
			}
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


