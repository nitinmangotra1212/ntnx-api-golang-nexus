/*
 * IDF Repository Implementation for Item Entity
 * Maps between protobuf Item model (camelCase) and IDF attributes (snake_case)
 */

package idf

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/nutanix-core/go-cache/insights/insights_interface"
	idfQr "github.com/nutanix-core/go-cache/insights/insights_interface/query"
	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config"
	statsPb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/stats"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/db"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/external"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/models"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ItemRepositoryImpl struct{}

// IDF Column Names (snake_case) - These match the Python script
const (
	itemEntityTypeName = "item"
	itemListPath       = "/items"

	// IDF attribute names (snake_case) - as registered in setup_nexus_idf.py
	itemIdAttr      = "item_id"
	itemNameAttr    = "item_name"
	itemTypeAttr    = "item_type"
	descriptionAttr = "description"
	extIdAttr       = "ext_id"
	// New attributes for GroupBy/Aggregations
	quantityAttr = "quantity"
	priceAttr    = "price"
	isActiveAttr = "is_active"
	priorityAttr = "priority"
	statusAttr   = "status"
	// List attributes
	int64ListAttr = "int64_list"
)

func NewItemRepository() db.ItemRepository {
	return &ItemRepositoryImpl{}
}

// CreateItem creates a new item in IDF
func (r *ItemRepositoryImpl) CreateItem(itemEntity *models.ItemEntity) error {
	// Get IDF client from singleton (following az-manager pattern)
	idfClient := external.Interfaces().IdfClient()

	// Generate UUID for EntityGuid (extId in IDF)
	// This is the entity's external ID in IDF
	var extIdUuid string
	if itemEntity.Item.ExtId != nil && *itemEntity.Item.ExtId != "" {
		extIdUuid = *itemEntity.Item.ExtId
	} else {
		extIdUuid = uuid.New().String()
	}

	attributeDataArgList := []*insights_interface.AttributeDataArg{}

	// Map protobuf fields (camelCase) to IDF attributes (snake_case)
	// Store itemId as int64 in IDF (1, 2, 3, ...)
	if itemEntity.Item.ItemId != nil {
		AddAttribute(&attributeDataArgList, itemIdAttr, *itemEntity.Item.ItemId)
	}

	// Store extId as string (UUID) - independent from itemId
	// This is stored both as EntityGuid.EntityId and as ext_id attribute
	AddAttribute(&attributeDataArgList, extIdAttr, extIdUuid)

	if itemEntity.Item.ItemName != nil {
		AddAttribute(&attributeDataArgList, itemNameAttr, *itemEntity.Item.ItemName)
	}
	if itemEntity.Item.ItemType != nil {
		AddAttribute(&attributeDataArgList, itemTypeAttr, *itemEntity.Item.ItemType)
	}
	if itemEntity.Item.Description != nil {
		AddAttribute(&attributeDataArgList, descriptionAttr, *itemEntity.Item.Description)
	}
	// New fields for GroupBy/Aggregations
	if itemEntity.Item.Quantity != nil {
		AddAttribute(&attributeDataArgList, quantityAttr, *itemEntity.Item.Quantity)
	}
	if itemEntity.Item.Price != nil {
		AddAttribute(&attributeDataArgList, priceAttr, *itemEntity.Item.Price)
	}
	if itemEntity.Item.IsActive != nil {
		AddAttribute(&attributeDataArgList, isActiveAttr, *itemEntity.Item.IsActive)
	}
	if itemEntity.Item.Priority != nil {
		AddAttribute(&attributeDataArgList, priorityAttr, int64(*itemEntity.Item.Priority))
	}
	if itemEntity.Item.Status != nil {
		AddAttribute(&attributeDataArgList, statusAttr, *itemEntity.Item.Status)
	}
	// List attributes
	if itemEntity.Item.Int64List != nil && len(itemEntity.Item.Int64List.Value) > 0 {
		AddAttribute(&attributeDataArgList, int64ListAttr, itemEntity.Item.Int64List.Value)
	}

	updateArg := &insights_interface.UpdateEntityArg{
		EntityGuid: &insights_interface.EntityGuid{
			EntityTypeName: proto.String(itemEntityTypeName),
			EntityId:       &extIdUuid,
		},
		AttributeDataArgList: attributeDataArgList,
	}

	// Call the IDF client to create the entity
	_, err := idfClient.UpdateEntityRet(updateArg)
	if err != nil {
		log.Errorf("Failed to create item: %v", err)
		return err
	}

	// Note: extId is stored in IDF, not in the Item protobuf model
	// The Item model doesn't have a Base field with ExtId

	log.Infof("Item created successfully with extId: %s", extIdUuid)
	return nil
}

// ListItems retrieves a list of items from IDF with pagination and filtering
// Uses OData parser to handle $filter, $orderby, $select, $expand, $apply
// When $apply is present, handles GroupBy and Aggregations
// When $expand is present, uses GraphQL via statsGW (following categories pattern)
func (r *ItemRepositoryImpl) ListItems(queryParams *models.QueryParams) ([]*pb.Item, int64, error) {
	// Handle GroupBy queries if $apply is present
	// Note: GroupBy queries return ItemGroup objects, not Item objects
	// This is handled separately in the gRPC service
	if queryParams.Apply != "" {
		log.Warnf("⚠️  GroupBy query detected but ListItems() cannot return ItemGroup. Use ListItemsWithGroupBy() instead.")
		// For now, fall through to regular query - the gRPC service should handle this
	}

	var items []*pb.Item
	var totalCount int64

	if queryParams.Expand != "" {
		// GraphQL path (with expand) - following categories pattern
		log.Infof("Using GraphQL path for expansion: %s", queryParams.Expand)

		// Try statsGW first, but fallback to regular IDF if it fails
		// NOTE: statsGW might not support nested expand options like $select and $orderby
		// So we'll always fallback to IDF path when nested options are present
		// Also, GraphQL doesn't support itemStats expand, so use IDF path for itemStats
		expandOptions := ParseExpandOptions(queryParams.Expand)
		hasNestedOptions := expandOptions != nil && (expandOptions.Select != nil || expandOptions.OrderBy != nil || expandOptions.Filter != nil)
		hasItemStatsExpand := strings.Contains(queryParams.Expand, "itemStats")

		// Debug: Log time range parameters if present
		if expandOptions != nil && hasItemStatsExpand {
			if expandOptions.StartTime != nil {
				log.Infof("🔍 [ListItems] Parsed $startTime: %d ms (%s)", *expandOptions.StartTime,
					time.Unix(*expandOptions.StartTime/1000, 0).UTC().Format(time.RFC3339))
			}
			if expandOptions.EndTime != nil {
				log.Infof("🔍 [ListItems] Parsed $endTime: %d ms (%s)", *expandOptions.EndTime,
					time.Unix(*expandOptions.EndTime/1000, 0).UTC().Format(time.RFC3339))
			}
			if expandOptions.StartTime == nil && expandOptions.EndTime == nil {
				log.Infof("🔍 [ListItems] No time range parameters in $expand (will use default)")
			}
		}

		statsGWClient := external.Interfaces().StatsGWClient()
		if statsGWClient != nil && !hasNestedOptions && !hasItemStatsExpand {
			// Generate GraphQL query from OData (only for associations, not itemStats)
			graphqlQuery, graphqlErr := GenerateGraphQLQuery(queryParams, itemListPath)
			if graphqlErr == nil {
				// Execute GraphQL via statsGW
				graphqlRet, err := statsGWClient.ExecuteGraphql(context.Background(), graphqlQuery)
				if err == nil {
					// Parse and map GraphQL response
					graphqlRetDto, err := ParseGraphqlResponse(graphqlRet.GetData())
					if err == nil {
						items, err = MapGraphqlToItems(graphqlRetDto, queryParams.Expand)
						if err == nil {
							totalCount = int64(graphqlRetDto.TotalCount)
							log.Infof("✅ Retrieved %d items from GraphQL (total: %d)", len(items), totalCount)
							return items, totalCount, nil
						}
					}
				}
				if err != nil {
					log.Warnf("statsGW query failed, falling back to regular IDF: %v", err)
				}
			}
		} else if hasNestedOptions {
			log.Infof("Nested expand options detected (select=%v, orderby=%v, filter=%v), using IDF fallback path",
				expandOptions.Select != nil, expandOptions.OrderBy != nil, expandOptions.Filter != nil)
		} else if hasItemStatsExpand {
			log.Infof("itemStats expand detected, using IDF path (GraphQL doesn't support itemStats)")
		}

		// Fallback: Fetch items and associations separately from IDF
		// This works without statsGW, though less efficient
		log.Warnf("statsGW not available or failed, fetching associations directly from IDF")
		queryParamsWithoutExpand := *queryParams
		queryParamsWithoutExpand.Expand = "" // Remove expand to use regular IDF path

		// Use regular IDF path to get items
		queryArg, err := GenerateListQuery(&queryParamsWithoutExpand, itemListPath, itemEntityTypeName, itemIdAttr)
		if err != nil {
			log.Errorf("Failed to generate IDF query from OData params: %v", err)
			return nil, 0, fmt.Errorf("failed to parse OData query: %w", err)
		}

		// Query IDF for items
		idfClient := external.Interfaces().IdfClient()
		queryResponse, err := idfClient.GetEntitiesWithMetricsRet(queryArg)
		if err != nil {
			log.Errorf("Failed to query IDF: %v", err)
			return nil, 0, err
		}

		// Convert IDF entities to Item protobufs
		groupResults := queryResponse.GetGroupResultsList()
		if len(groupResults) == 0 {
			return []*pb.Item{}, 0, nil
		}

		entitiesWithMetric := groupResults[0].GetRawResults()
		entities := ConvertEntitiesWithMetricToEntities(entitiesWithMetric)
		for _, entity := range entities {
			item := r.mapIdfAttributeToItem(entity)
			items = append(items, item)
		}

		// Now fetch associations for each item from IDF if expand includes associations
		// Query item_associations entity where item_id matches item.extId
		var associationsMap map[string][]map[string]interface{}
		if strings.Contains(queryParams.Expand, "associations") {
			log.Infof("🔍 Fetching associations for %d items", len(items))
			var err error
			associationsMap, err = r.fetchAssociationsForItems(items)
			if err != nil {
				log.Warnf("Failed to fetch associations: %v, continuing without associations", err)
				associationsMap = make(map[string][]map[string]interface{})
			} else {
				log.Infof("📊 Fetched associations map with %d item entries", len(associationsMap))
				// Attach associations to items
				totalAssocs := 0
				for _, item := range items {
					if item.ExtId != nil {
						log.Debugf("Checking associations for item extId: %s", *item.ExtId)
						if assocs, found := associationsMap[*item.ExtId]; found {
							log.Debugf("Found %d associations for item %s", len(assocs), *item.ExtId)
							if len(assocs) > 0 {
								// Convert map associations to protobuf ItemAssociation objects
								itemAssociations := make([]*pb.ItemAssociation, 0, len(assocs))
								for i, assocMap := range assocs {
									itemAssoc := &pb.ItemAssociation{}

									if entityType, ok := assocMap["entityType"].(string); ok {
										itemAssoc.EntityType = &entityType
										log.Debugf("  Association[%d]: entityType=%s", i, entityType)
									}
									if entityId, ok := assocMap["entityId"].(string); ok {
										itemAssoc.EntityId = &entityId
										log.Debugf("  Association[%d]: entityId=%s", i, entityId)
									}
									if count, ok := assocMap["count"].(int32); ok {
										itemAssoc.Count = &count
										log.Debugf("  Association[%d]: count=%d", i, count)
									}
									if itemId, ok := assocMap["itemId"].(string); ok {
										itemAssoc.ItemId = &itemId
										log.Debugf("  Association[%d]: itemId=%s", i, itemId)
									}
									// Map stats fields (totalCount and averageScore) from stats module
									// TODO: Uncomment after regenerating protobufs with TotalCount and AverageScore fields
									// if totalCount, ok := assocMap["totalCount"].(int64); ok {
									// 	itemAssoc.TotalCount = &totalCount
									// 	log.Debugf("  Association[%d]: totalCount=%d", i, totalCount)
									// }
									// if averageScore, ok := assocMap["averageScore"].(float64); ok {
									// 	itemAssoc.AverageScore = &averageScore
									// 	log.Debugf("  Association[%d]: averageScore=%f", i, averageScore)
									// }

									itemAssociations = append(itemAssociations, itemAssoc)
								}

								log.Infof("📦 Converted %d associations to protobuf for item %s", len(itemAssociations), *item.ExtId)

								// Apply nested expand options (filter, select, orderby)
								// Examples:
								//   - $expand=associations($filter=entityType eq 'vm')
								//   - $expand=associations($select=entityType,count)
								//   - $expand=associations($orderby=entityType asc)
								expandOptions := ParseExpandOptions(queryParams.Expand)
								if expandOptions != nil {
									log.Infof("🔧 Applying expand options: filter=%v, select=%v, orderby=%v (before: %d associations)",
										expandOptions.Filter != nil, expandOptions.Select != nil, expandOptions.OrderBy != nil, len(itemAssociations))
									itemAssociations = ApplyExpandOptions(itemAssociations, expandOptions)
									log.Infof("✅ Applied expand options, result: %d associations", len(itemAssociations))
								}

								// Only attach if there are associations after applying options
								if len(itemAssociations) > 0 {
									// Wrap in ItemAssociationArrayWrapper
									item.Associations = &pb.ItemAssociationArrayWrapper{
										Value: itemAssociations,
									}
									totalAssocs += len(itemAssociations)
									// Log association details for debugging
									for i, assoc := range itemAssociations {
										log.Infof("  Association[%d]: entityType=%v, entityId=%v, count=%v, itemId=%v",
											i, assoc.EntityType, assoc.EntityId, assoc.Count, assoc.ItemId)
									}
									log.Infof("✅ Attached %d associations to item %s", len(itemAssociations), *item.ExtId)
								} else {
									log.Warnf("⚠️  No associations remaining after applying expand options for item %s", *item.ExtId)
								}
							} else {
								log.Debugf("No associations found for item %s (empty list)", *item.ExtId)
							}
						} else {
							log.Debugf("No associations found in map for item extId: %s", *item.ExtId)
						}
					} else {
						log.Debugf("Item has no extId, skipping association fetch")
					}
				}
				log.Infof("✅ Attached %d total associations to %d items (from %d items processed)", totalAssocs, len(associationsMap), len(items))
			}
		}

		// Now fetch itemStats for each item from IDF if expand includes itemStats
		if strings.Contains(queryParams.Expand, "itemStats") {
			log.Infof("🔍 [itemStats EXPAND] Fetching itemStats for %d items (expand param: %s)", len(items), queryParams.Expand)
			// Parse expand options to extract time-series parameters
			expandOptions := ParseExpandOptions(queryParams.Expand)
			itemStatsMap, err := r.fetchItemStatsForItems(items, expandOptions)
			if err != nil {
				log.Errorf("❌ [itemStats EXPAND] Failed to fetch itemStats: %v", err)
			} else {
				log.Infof("📊 [itemStats EXPAND] Fetched itemStats map with %d item entries (total items: %d)", len(itemStatsMap), len(items))
				// Attach itemStats to items
				totalStats := 0
				for _, item := range items {
					if item.ExtId != nil {
						log.Debugf("[itemStats EXPAND] Checking itemStats for item extId: %s", *item.ExtId)
						if stats, found := itemStatsMap[*item.ExtId]; found {
							log.Debugf("[itemStats EXPAND] Found %d itemStats for item %s (taking first for one-to-one)", len(stats), *item.ExtId)
							if len(stats) > 0 {
								// One-to-one relationship: take only the first itemStats
								firstStat := stats[0]
								// Assign single ItemStats object (protobuf now uses single object, not array wrapper)
								item.ItemStats = firstStat
								totalStats += 1
								log.Infof("✅ [itemStats EXPAND] Attached 1 itemStats (one-to-one) to item %s", *item.ExtId)
							}
						} else {
							log.Debugf("[itemStats EXPAND] No itemStats found for item extId: %s", *item.ExtId)
						}
					} else {
						log.Debugf("[itemStats EXPAND] Item has no extId, skipping itemStats fetch")
					}
				}
				log.Infof("✅ [itemStats EXPAND] Attached %d total itemStats to %d items (map size: %d)", totalStats, len(items), len(itemStatsMap))
			}
		} else {
			log.Debugf("[itemStats EXPAND] Expand does not contain 'itemStats' (expand: %s)", queryParams.Expand)
		}

		totalCount = groupResults[0].GetTotalEntityCount()
		log.Infof("✅ Retrieved %d items from IDF (total: %d) with associations and itemStats fetched separately", len(items), totalCount)
	} else {
		// Regular IDF path (no expand)
		log.Debugf("Using regular IDF path (no expand)")

		// Use OData parser to generate IDF query
		queryArg, err := GenerateListQuery(queryParams, itemListPath, itemEntityTypeName, itemIdAttr)
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

		// Convert IDF entities to Item protobufs
		groupResults := queryResponse.GetGroupResultsList()
		if len(groupResults) == 0 {
			return []*pb.Item{}, 0, nil
		}

		entitiesWithMetric := groupResults[0].GetRawResults()
		// Convert EntityWithMetric to Entity (following az-manager pattern)
		entities := ConvertEntitiesWithMetricToEntities(entitiesWithMetric)
		for _, entity := range entities {
			item := r.mapIdfAttributeToItem(entity)
			items = append(items, item)
		}

		totalCount = groupResults[0].GetTotalEntityCount()
		log.Infof("✅ Retrieved %d items from IDF (total: %d)", len(items), totalCount)
	}

	return items, totalCount, nil
}

// ListItemsWithGroupBy handles GroupBy and Aggregations queries
// When $apply=groupby(...) is used, the response structure is different (ItemGroup)
// Returns ItemGroup objects with group keys and aggregated data
func (r *ItemRepositoryImpl) ListItemsWithGroupBy(queryParams *models.QueryParams) ([]*pb.ItemGroup, int64, error) {
	log.Infof("Executing GroupBy query with $apply: %s", queryParams.Apply)

	// Use OData parser to generate IDF query (handles $apply via IDFApplyEvaluator)
	queryArg, err := GenerateListQuery(queryParams, itemListPath, itemEntityTypeName, itemIdAttr)
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

	// Convert grouped results to ItemGroup objects
	groupResults := queryResponse.GetGroupResultsList()
	if len(groupResults) == 0 {
		log.Infof("No grouped results returned from IDF")
		return []*pb.ItemGroup{}, 0, nil
	}

	var itemGroups []*pb.ItemGroup
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
		// For groupby(itemType), all entities in the group have the same itemType value
		firstEntity := entities[0]
		groupKey := r.extractGroupKey(firstEntity, queryParams.Apply)
		if groupKey == nil {
			log.Warnf("Failed to extract group key from first entity, skipping group")
			continue
		}

		// Convert entities to items
		var items []*pb.Item
		for _, entity := range entities {
			item := r.mapIdfAttributeToItem(entity)
			items = append(items, item)
		}

		// Handle $expand if present - fetch associations for items in this group
		if queryParams.Expand != "" {
			log.Infof("🔗 Processing $expand for GroupBy: %s", queryParams.Expand)
			// Fetch associations for all items in this group
			associationsMap, err := r.fetchAssociationsForItems(items)
			if err != nil {
				log.Warnf("Failed to fetch associations for group: %v, continuing without associations", err)
			} else {
				// Attach associations to items in this group
				totalAssocs := 0
				for _, item := range items {
					if item.ExtId != nil {
						if assocs, found := associationsMap[*item.ExtId]; found {
							if len(assocs) > 0 {
								// Convert map associations to protobuf ItemAssociation objects
								itemAssociations := make([]*pb.ItemAssociation, 0, len(assocs))
								for i, assocMap := range assocs {
									itemAssoc := &pb.ItemAssociation{}

									if entityType, ok := assocMap["entityType"].(string); ok {
										itemAssoc.EntityType = &entityType
										log.Debugf("  Association[%d]: entityType=%s", i, entityType)
									}
									if entityId, ok := assocMap["entityId"].(string); ok {
										itemAssoc.EntityId = &entityId
										log.Debugf("  Association[%d]: entityId=%s", i, entityId)
									}
									if count, ok := assocMap["count"].(int32); ok {
										itemAssoc.Count = &count
										log.Debugf("  Association[%d]: count=%d", i, count)
									}
									if itemId, ok := assocMap["itemId"].(string); ok {
										itemAssoc.ItemId = &itemId
										log.Debugf("  Association[%d]: itemId=%s", i, itemId)
									}
									// Map stats fields (totalCount and averageScore) from stats module
									// TODO: Uncomment after regenerating protobufs with TotalCount and AverageScore fields
									// if totalCount, ok := assocMap["totalCount"].(int64); ok {
									// 	itemAssoc.TotalCount = &totalCount
									// 	log.Debugf("  Association[%d]: totalCount=%d", i, totalCount)
									// }
									// if averageScore, ok := assocMap["averageScore"].(float64); ok {
									// 	itemAssoc.AverageScore = &averageScore
									// 	log.Debugf("  Association[%d]: averageScore=%f", i, averageScore)
									// }

									itemAssociations = append(itemAssociations, itemAssoc)
								}

								// Apply nested expand options (filter, select, orderby)
								// Examples:
								//   - $expand=associations($filter=entityType eq 'vm')
								//   - $expand=associations($select=entityType,count)
								//   - $expand=associations($orderby=entityType asc)
								expandOptions := ParseExpandOptions(queryParams.Expand)
								if expandOptions != nil {
									log.Infof("🔧 Applying expand options to GroupBy: filter=%v, select=%v, orderby=%v (before: %d associations)",
										expandOptions.Filter != nil, expandOptions.Select != nil, expandOptions.OrderBy != nil, len(itemAssociations))
									itemAssociations = ApplyExpandOptions(itemAssociations, expandOptions)
									log.Infof("✅ Applied expand options, result: %d associations", len(itemAssociations))
								}

								// Only attach if there are associations after applying options
								if len(itemAssociations) > 0 {
									// Wrap in ItemAssociationArrayWrapper
									item.Associations = &pb.ItemAssociationArrayWrapper{
										Value: itemAssociations,
									}
									totalAssocs += len(itemAssociations)
									log.Debugf("✅ Attached %d associations to item %s in group", len(itemAssociations), *item.ExtId)
								}
							}
						}
					}
				}
				log.Infof("✅ Attached %d total associations to items in this group", totalAssocs)
			}

			// Fetch itemStats if expand includes itemStats
			if strings.Contains(queryParams.Expand, "itemStats") {
				log.Infof("🔍 Fetching itemStats for items in this group")
				expandOptions := ParseExpandOptions(queryParams.Expand)
				itemStatsMap, err := r.fetchItemStatsForItems(items, expandOptions)
				if err != nil {
					log.Warnf("Failed to fetch itemStats for group: %v, continuing without itemStats", err)
				} else {
					totalStats := 0
					for _, item := range items {
						if item.ExtId != nil {
							if stats, found := itemStatsMap[*item.ExtId]; found {
								if len(stats) > 0 {
									// One-to-one relationship: take only the first itemStats
									firstStat := stats[0]
									// Assign single ItemStats object (protobuf now uses single object, not array wrapper)
									item.ItemStats = firstStat
									totalStats += 1
								}
							}
						}
					}
					log.Infof("✅ Attached %d total itemStats to items in this group", totalStats)
				}
			}
		}

		// Extract aggregate results from IDF if present
		// TODO: IDF aggregate results need to be extracted from EntityWithMetric.MetricDataList
		// For now, we'll extract from the $apply parameter and construct aggregates
		// The actual aggregate values should come from IDF's MetricDataList in EntityWithMetric
		var aggregates []*pb.ItemAggregate

		// Check if $apply contains aggregate expressions
		if strings.Contains(queryParams.Apply, "aggregate(") {
			log.Infof("📊 Parsing aggregate expressions from $apply: %s", queryParams.Apply)

			// Parse $apply to extract aggregation aliases (e.g., "totalCount" from "itemId with count as totalCount")
			aggregationAliases := r.parseAggregationAliases(queryParams.Apply)
			log.Debugf("  Parsed %d aggregation aliases: %v", len(aggregationAliases), aggregationAliases)

			// Extract aggregate values from EntityWithMetric.MetricDataList
			// For group-by queries with aggregations, IDF returns aggregate values as metrics
			entitiesWithMetric := groupResult.GetRawResults()
			if len(entitiesWithMetric) > 0 {
				// The first EntityWithMetric might contain group-level aggregate metrics
				firstEntityWithMetric := entitiesWithMetric[0]
				if firstEntityWithMetric != nil && firstEntityWithMetric.GetMetricDataList() != nil {
					metricDataList := firstEntityWithMetric.GetMetricDataList()
					log.Debugf("  Found %d metrics in EntityWithMetric", len(metricDataList))

					// Log all metric names for debugging
					for i, metricData := range metricDataList {
						log.Debugf("  Metric[%d]: name=%s", i, metricData.GetName())
					}

					// Map metric data to ItemAggregate objects
					// Only include metrics that match the requested aggregations
					for _, metricData := range metricDataList {
						metricName := metricData.GetName()
						if metricData.GetValueList() != nil && len(metricData.GetValueList()) > 0 {
							metricValue := metricData.GetValueList()[0].GetValue()

							// Try to match metric name to aggregation alias
							// Metric names in IDF might be in format "column_operator" (e.g., "item_id_kCount", "price_kAvg")
							alias := r.findAggregationAliasFromMetricName(metricName, aggregationAliases)

							// Only create aggregate if we found a matching alias (i.e., this metric corresponds to a requested aggregation)
							if alias == "" {
								log.Debugf("  Skipping metric %s (no matching aggregation alias)", metricName)
								continue
							}

							itemAggregate := &pb.ItemAggregate{
								Label: &alias,
							}

							// Map the metric value to appropriate result type
							if metricValue != nil {
								if intVal := metricValue.GetInt64Value(); intVal != 0 {
									itemAggregate.Result = &pb.ItemAggregate_Int64Result{
										Int64Result: &pb.Int64Wrapper{
											Value: proto.Int64(intVal),
										},
									}
									log.Debugf("  ✅ Mapped metric %s -> aggregate %s (int64: %d)", metricName, alias, intVal)
								} else if doubleVal := metricValue.GetDoubleValue(); doubleVal != 0 {
									itemAggregate.Result = &pb.ItemAggregate_DoubleResult{
										DoubleResult: &pb.DoubleWrapper{
											Value: proto.Float64(doubleVal),
										},
									}
									log.Debugf("  ✅ Mapped metric %s -> aggregate %s (double: %f)", metricName, alias, doubleVal)
								} else {
									log.Debugf("  ⚠️  Metric %s has value but type is not int64 or double", metricName)
								}
							} else {
								log.Debugf("  ⚠️  Metric %s has nil value", metricName)
							}

							if itemAggregate.Result != nil {
								aggregates = append(aggregates, itemAggregate)
							}
						}
					}
				}
			}

			log.Infof("✅ Mapped %d aggregates to ItemAggregate objects (expected: %d)", len(aggregates), len(aggregationAliases))
		} else {
			log.Debugf("No aggregate expressions found in $apply parameter")
		}

		// Create ItemGroup with group key, items, and aggregates
		var aggregatesWrapper *pb.ItemAggregateArrayWrapper
		if len(aggregates) > 0 {
			aggregatesWrapper = &pb.ItemAggregateArrayWrapper{
				Value: aggregates,
			}
		}

		itemGroup := &pb.ItemGroup{
			Data: &pb.ItemGroup_ItemArrayData{
				ItemArrayData: &pb.ItemArrayWrapper{
					Value: items,
				},
			},
			Aggregates: aggregatesWrapper,
		}

		// Set the group key based on type
		switch v := groupKey.(type) {
		case *pb.ItemGroup_StringGroup:
			itemGroup.Group = v
		case *pb.ItemGroup_Int32Group:
			itemGroup.Group = v
		case *pb.ItemGroup_Int64Group:
			itemGroup.Group = v
		case *pb.ItemGroup_DoubleGroup:
			itemGroup.Group = v
		case *pb.ItemGroup_BooleanGroup:
			itemGroup.Group = v
		default:
			log.Warnf("Unknown group key type: %T", v)
			continue
		}

		itemGroups = append(itemGroups, itemGroup)
		totalCount += groupResult.GetTotalEntityCount()
	}

	log.Infof("✅ Retrieved %d ItemGroups from GroupBy query (total: %d groups, %d total items)",
		len(itemGroups), len(groupResults), totalCount)

	return itemGroups, totalCount, nil
}

// extractGroupKey extracts the group key from an entity based on the $apply parameter
// For groupby(itemType), extracts itemType value and returns ItemGroup_StringGroup
func (r *ItemRepositoryImpl) extractGroupKey(entity *insights_interface.Entity, applyParam string) interface{} {
	// Parse $apply to determine which field we're grouping by
	// For now, handle simple cases: groupby(itemType), groupby(itemId), etc.
	// TODO: Support multiple groupby fields and complex expressions

	// Simple heuristic: if applyParam contains "itemType", group by itemType
	if strings.Contains(applyParam, "itemType") {
		// Extract itemType value from entity
		for _, attr := range entity.GetAttributeDataMap() {
			if attr.GetName() == itemTypeAttr {
				if attr.GetValue() != nil {
					val := attr.GetValue().GetStrValue()
					return &pb.ItemGroup_StringGroup{
						StringGroup: &pb.StringWrapper{
							Value: proto.String(val),
						},
					}
				}
			}
		}
	} else if strings.Contains(applyParam, "itemId") {
		// Group by itemId (int64)
		for _, attr := range entity.GetAttributeDataMap() {
			if attr.GetName() == itemIdAttr {
				if attr.GetValue() != nil {
					val := int32(attr.GetValue().GetInt64Value())
					return &pb.ItemGroup_Int32Group{
						Int32Group: &pb.Int32Wrapper{
							Value: proto.Int32(val),
						},
					}
				}
			}
		}
	} else if strings.Contains(applyParam, "isActive") {
		// Group by isActive (boolean)
		for _, attr := range entity.GetAttributeDataMap() {
			if attr.GetName() == isActiveAttr {
				if attr.GetValue() != nil {
					val := attr.GetValue().GetBoolValue()
					return &pb.ItemGroup_BooleanGroup{
						BooleanGroup: &pb.BooleanWrapper{
							Value: proto.Bool(val),
						},
					}
				}
			}
		}
	} else if strings.Contains(applyParam, "status") {
		// Group by status (string)
		for _, attr := range entity.GetAttributeDataMap() {
			if attr.GetName() == statusAttr {
				if attr.GetValue() != nil {
					val := attr.GetValue().GetStrValue()
					return &pb.ItemGroup_StringGroup{
						StringGroup: &pb.StringWrapper{
							Value: proto.String(val),
						},
					}
				}
			}
		}
	}

	log.Warnf("Could not extract group key from applyParam: %s", applyParam)
	return nil
}

// parseAggregationAliases parses the $apply parameter to extract aggregation aliases
// Example: "groupby((itemType),aggregate(itemId with count as totalCount,quantity with sum as totalQuantity))"
// Returns a map of "column_operator" -> "alias" (e.g., "item_id_count" -> "totalCount")
func (r *ItemRepositoryImpl) parseAggregationAliases(applyParam string) map[string]string {
	aliases := make(map[string]string)

	if applyParam == "" {
		return aliases
	}

	// Look for aggregate(...) expressions in the $apply parameter
	// Pattern: aggregate(columnName with operator as alias, ...)
	aggregatePattern := regexp.MustCompile(`aggregate\(([^)]+)\)`)
	matches := aggregatePattern.FindStringSubmatch(applyParam)

	if len(matches) < 2 {
		log.Debugf("No aggregate expressions found in $apply: %s", applyParam)
		return aliases
	}

	// Split by comma to get individual aggregation expressions
	// Example: "itemId with count as totalCount,quantity with sum as totalQuantity"
	aggregateExprs := strings.Split(matches[1], ",")

	for _, expr := range aggregateExprs {
		expr = strings.TrimSpace(expr)
		// Pattern: "columnName with operator as alias"
		// Example: "itemId with count as totalCount"
		parts := strings.Split(expr, " with ")
		if len(parts) != 2 {
			log.Debugf("Invalid aggregation expression format: %s", expr)
			continue
		}

		columnName := strings.TrimSpace(parts[0])
		operatorAndAlias := strings.TrimSpace(parts[1])

		// Split "operator as alias"
		operatorParts := strings.Split(operatorAndAlias, " as ")
		if len(operatorParts) != 2 {
			log.Debugf("Invalid operator/alias format: %s", operatorAndAlias)
			continue
		}

		operator := strings.TrimSpace(operatorParts[0])
		alias := strings.TrimSpace(operatorParts[1])

		// Convert column name to IDF format (camelCase -> snake_case)
		// For now, use simple mapping (can be enhanced)
		idfColumn := r.camelToSnake(columnName)

		// Create key: "column_operator" (e.g., "item_id_count")
		key := fmt.Sprintf("%s_%s", idfColumn, strings.ToLower(operator))
		aliases[key] = alias

		log.Debugf("Parsed aggregation: column=%s, operator=%s, alias=%s, key=%s", columnName, operator, alias, key)
	}

	return aliases
}

// findAggregationAlias finds the alias for a given column and operator
func (r *ItemRepositoryImpl) findAggregationAlias(column, operator string, aliases map[string]string) string {
	key := fmt.Sprintf("%s_%s", strings.ToLower(column), strings.ToLower(operator))
	if alias, found := aliases[key]; found {
		return alias
	}
	return ""
}

// findAggregationAliasFromMetricName finds the alias for a given metric name
// IDF metric names are in format "column_kOperator" (e.g., "price_kAvg", "item_id_kCount")
// The aggregation aliases map has keys like "price_average" (from "price with average as avgPrice")
func (r *ItemRepositoryImpl) findAggregationAliasFromMetricName(metricName string, aliases map[string]string) string {
	// Parse IDF metric name: "column_kOperator" -> extract column and operator
	// Examples: "price_kAvg" -> column="price", operator="average"
	//           "item_id_kCount" -> column="item_id", operator="count"

	metricLower := strings.ToLower(metricName)

	// IDF operators: kCount, kSum, kAvg, kMin, kMax
	operatorMap := map[string]string{
		"kcount": "count",
		"ksum":   "sum",
		"kavg":   "average",
		"kmin":   "min",
		"kmax":   "max",
	}

	// Try to find operator in metric name
	var foundOperator string
	var foundColumn string
	for idfOp, odataOp := range operatorMap {
		if strings.HasSuffix(metricLower, "_"+idfOp) || strings.HasSuffix(metricLower, idfOp) {
			foundOperator = odataOp
			// Extract column name (everything before the operator)
			parts := strings.Split(metricLower, "_"+idfOp)
			if len(parts) > 0 {
				foundColumn = parts[0]
			} else {
				parts = strings.Split(metricLower, idfOp)
				if len(parts) > 0 {
					foundColumn = parts[0]
				}
			}
			break
		}
	}

	if foundColumn != "" && foundOperator != "" {
		// Build key in format "column_operator" (e.g., "price_average")
		key := fmt.Sprintf("%s_%s", foundColumn, foundOperator)
		if alias, found := aliases[key]; found {
			log.Debugf("  Matched metric %s (column=%s, operator=%s) -> alias %s", metricName, foundColumn, foundOperator, alias)
			return alias
		}
		log.Debugf("  No alias found for metric %s (key: %s)", metricName, key)
	} else {
		log.Debugf("  Could not parse metric name: %s", metricName)
	}

	return ""
}

// camelToSnake converts camelCase to snake_case (simple implementation)
func (r *ItemRepositoryImpl) camelToSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			result.WriteRune('_')
		}
		result.WriteRune(unicode.ToLower(r))
	}
	return result.String()
}

// mapIdfAttributeToItem maps IDF attributes (snake_case) to protobuf Item (camelCase)
// This is the key mapping function that converts between IDF column names and protobuf field names
// itemId is integer (1, 2, 3...), extId is UUID string from EntityGuid
func (r *ItemRepositoryImpl) mapIdfAttributeToItem(entity *insights_interface.Entity) *pb.Item {
	item := &pb.Item{}

	// Get extId from EntityGuid.EntityId (this is the UUID stored in IDF)
	// This is the entity's external ID in IDF
	if entity.GetEntityGuid() != nil && entity.GetEntityGuid().GetEntityId() != "" {
		extId := entity.GetEntityGuid().GetEntityId()
		item.ExtId = &extId
		log.Debugf("  Set extId from EntityGuid: %s", extId)
	}

	// Debug: Log all attributes received from IDF
	attrMap := entity.GetAttributeDataMap()
	log.Infof("🔍 [IDF ENTITY] Mapping IDF entity with %d attributes", len(attrMap))

	// Log ALL attribute names first to see what IDF is returning
	allAttrNames := make([]string, 0, len(attrMap))
	for _, attr := range attrMap {
		allAttrNames = append(allAttrNames, attr.GetName())
	}
	log.Infof("📋 [IDF ENTITY] All attribute names from IDF: %v", allAttrNames)

	listAttrCount := 0
	for _, attr := range attrMap {
		attrName := attr.GetName()
		if strings.Contains(attrName, "_list") {
			listAttrCount++
			log.Infof("  🔍 [LIST DEBUG] Found list attribute: %s", attrName)
			if attr.GetValue() == nil {
				log.Warnf("  ⚠️  [LIST DEBUG] %s value is nil", attrName)
			} else {
				// Try to determine the actual value type
				if attr.GetValue().GetStrList() != nil {
					vals := attr.GetValue().GetStrList().GetValueList()
					log.Infof("  ✅ [LIST DEBUG] %s is StrList with %d values: %v", attrName, len(vals), vals)
				} else if attr.GetValue().GetInt64List() != nil {
					vals := attr.GetValue().GetInt64List().GetValueList()
					log.Infof("  ✅ [LIST DEBUG] %s is Int64List with %d values: %v", attrName, len(vals), vals)
				} else if attr.GetValue().GetDoubleList() != nil {
					vals := attr.GetValue().GetDoubleList().GetValueList()
					log.Infof("  ✅ [LIST DEBUG] %s is DoubleList with %d values: %v", attrName, len(vals), vals)
				} else if attr.GetValue().GetBoolList() != nil {
					vals := attr.GetValue().GetBoolList().GetValueList()
					log.Infof("  ✅ [LIST DEBUG] %s is BoolList with %d values: %v", attrName, len(vals), vals)
				} else {
					log.Warnf("  ⚠️  [LIST DEBUG] %s value exists but is not a recognized list type (value: %+v)", attrName, attr.GetValue())
				}
			}
		} else {
			log.Debugf("  IDF attribute: %s = %+v", attrName, attr.GetValue())
		}
	}
	log.Infof("  📊 Found %d list attributes in IDF entity", listAttrCount)

	for _, attr := range entity.GetAttributeDataMap() {
		switch attr.GetName() {
		case itemIdAttr: // "item_id" (IDF) → ItemId (protobuf) - stored as int64
			if attr.GetValue() != nil && attr.GetValue().GetInt64Value() != 0 {
				val := int32(attr.GetValue().GetInt64Value())
				item.ItemId = &val
				log.Debugf("  Mapped item_id: %d", val)
			}

		case itemNameAttr: // "item_name" (IDF) → ItemName (protobuf)
			if attr.GetValue() != nil {
				val := attr.GetValue().GetStrValue()
				item.ItemName = &val
				log.Debugf("  Mapped item_name: %s", val)
			}

		case itemTypeAttr: // "item_type" (IDF) → ItemType (protobuf)
			if attr.GetValue() != nil {
				val := attr.GetValue().GetStrValue()
				item.ItemType = &val
				log.Debugf("  Mapped item_type: %s", val)
			}

		case descriptionAttr: // "description" (IDF) → Description (protobuf)
			if attr.GetValue() != nil {
				val := attr.GetValue().GetStrValue()
				item.Description = &val
				log.Debugf("  Mapped description: %s", val)
			}

		case extIdAttr: // "ext_id" (IDF) → ExtId (protobuf) - UUID string
			// extId can also come from attribute (if stored separately from EntityGuid)
			if attr.GetValue() != nil {
				val := attr.GetValue().GetStrValue()
				if val != "" {
					item.ExtId = &val
					log.Debugf("  Mapped ext_id from attribute: %s", val)
				}
			}

		case quantityAttr: // "quantity" (IDF) → Quantity (protobuf) - int64
			if attr.GetValue() != nil {
				val := attr.GetValue().GetInt64Value()
				if val != 0 {
					item.Quantity = &val
					log.Debugf("  Mapped quantity: %d", val)
				}
			}

		case priceAttr: // "price" (IDF) → Price (protobuf) - double
			if attr.GetValue() != nil {
				val := attr.GetValue().GetDoubleValue()
				item.Price = &val
				log.Debugf("  Mapped price: %f", val)
			}

		case isActiveAttr: // "is_active" (IDF) → IsActive (protobuf) - bool
			if attr.GetValue() != nil {
				val := attr.GetValue().GetBoolValue()
				item.IsActive = &val
				log.Debugf("  Mapped is_active: %v", val)
			}

		case priorityAttr: // "priority" (IDF) → Priority (protobuf) - int32
			if attr.GetValue() != nil {
				val := int32(attr.GetValue().GetInt64Value()) // byte stored as int64 in IDF
				item.Priority = &val
				log.Debugf("  Mapped priority: %d", val)
			}

		case statusAttr: // "status" (IDF) → Status (protobuf) - string
			if attr.GetValue() != nil {
				val := attr.GetValue().GetStrValue()
				if val != "" {
					item.Status = &val
					log.Debugf("  Mapped status: %s", val)
				}
			}

		case int64ListAttr: // "int64_list" (IDF) → Int64List (protobuf) - []int64
			log.Debugf("  🔍 Found int64_list attribute")
			if attr.GetValue() == nil {
				log.Debugf("  ⚠️  int64_list value is nil")
			} else if attr.GetValue().GetInt64List() == nil {
				log.Debugf("  ⚠️  int64_list GetInt64List() is nil, value type: %T", attr.GetValue().ValueType)
			} else {
				val := attr.GetValue().GetInt64List().GetValueList()
				log.Debugf("  ✅ int64_list has %d values: %v", len(val), val)
				if len(val) > 0 {
					item.Int64List = &pb.LongArrayWrapper{
						Value: val,
					}
					log.Infof("  ✅ Mapped int64_list: %v", val)
				}
			}

		default:
			log.Debugf("Unknown attribute %s in IDF entity for item", attr.GetName())
		}
	}

	log.Debugf("Mapped Item: ItemId=%v, ItemName=%v, ItemType=%v, Description=%v, ExtId=%v",
		item.ItemId, item.ItemName, item.ItemType, item.Description, item.ExtId)

	return item
}

// GetItemById retrieves an item by its external ID
func (r *ItemRepositoryImpl) GetItemById(extId string) (*models.ItemEntity, error) {
	getArg := &insights_interface.GetEntitiesArg{
		EntityGuidList: []*insights_interface.EntityGuid{
			{
				EntityTypeName: proto.String(itemEntityTypeName),
				EntityId:       &extId,
			},
		},
	}

	idfClient := external.Interfaces().IdfClient()
	getResponse, err := idfClient.GetEntityRet(getArg)
	if err != nil {
		log.Errorf("Failed to get item by ID %s: %v", extId, err)
		return nil, err
	}

	// GetEntitiesRet returns Entity field (via GetEntity() method)
	if len(getResponse.GetEntity()) == 0 {
		return nil, fmt.Errorf("item not found: %s", extId)
	}

	entity := getResponse.GetEntity()[0]
	// Entity from GetEntityRet is already Entity type, not EntityWithMetric
	item := r.mapIdfAttributeToItem(entity)

	return &models.ItemEntity{
		Item: item,
	}, nil
}

// UpdateItem updates an existing item in IDF
func (r *ItemRepositoryImpl) UpdateItem(extId string, itemEntity *models.ItemEntity) error {
	attributeDataArgList := []*insights_interface.AttributeDataArg{}

	// Map protobuf fields to IDF attributes
	// Store itemId as int64 in IDF (1, 2, 3, ...)
	if itemEntity.Item.ItemId != nil {
		AddAttribute(&attributeDataArgList, itemIdAttr, *itemEntity.Item.ItemId)
	}

	// Store extId as string (UUID) - independent from itemId
	// Use the extId parameter (from URL path) or from the request body
	updateExtId := extId
	if itemEntity.Item.ExtId != nil && *itemEntity.Item.ExtId != "" {
		updateExtId = *itemEntity.Item.ExtId
	}
	if updateExtId != "" {
		AddAttribute(&attributeDataArgList, extIdAttr, updateExtId)
	}

	if itemEntity.Item.ItemName != nil {
		AddAttribute(&attributeDataArgList, itemNameAttr, *itemEntity.Item.ItemName)
	}
	if itemEntity.Item.ItemType != nil {
		AddAttribute(&attributeDataArgList, itemTypeAttr, *itemEntity.Item.ItemType)
	}
	if itemEntity.Item.Description != nil {
		AddAttribute(&attributeDataArgList, descriptionAttr, *itemEntity.Item.Description)
	}
	// New fields for GroupBy/Aggregations
	if itemEntity.Item.Quantity != nil {
		AddAttribute(&attributeDataArgList, quantityAttr, *itemEntity.Item.Quantity)
	}
	if itemEntity.Item.Price != nil {
		AddAttribute(&attributeDataArgList, priceAttr, *itemEntity.Item.Price)
	}
	if itemEntity.Item.IsActive != nil {
		AddAttribute(&attributeDataArgList, isActiveAttr, *itemEntity.Item.IsActive)
	}
	if itemEntity.Item.Priority != nil {
		AddAttribute(&attributeDataArgList, priorityAttr, int64(*itemEntity.Item.Priority))
	}
	if itemEntity.Item.Status != nil {
		AddAttribute(&attributeDataArgList, statusAttr, *itemEntity.Item.Status)
	}
	// List attributes
	if itemEntity.Item.Int64List != nil && len(itemEntity.Item.Int64List.Value) > 0 {
		AddAttribute(&attributeDataArgList, int64ListAttr, itemEntity.Item.Int64List.Value)
	}

	updateArg := &insights_interface.UpdateEntityArg{
		EntityGuid: &insights_interface.EntityGuid{
			EntityTypeName: proto.String(itemEntityTypeName),
			EntityId:       &extId,
		},
		AttributeDataArgList: attributeDataArgList,
	}

	idfClient := external.Interfaces().IdfClient()
	_, err := idfClient.UpdateEntityRet(updateArg)
	if err != nil {
		log.Errorf("Failed to update item %s: %v", extId, err)
		return err
	}

	log.Infof("Item updated successfully: %s", extId)
	return nil
}

// DeleteItem deletes an item from IDF
func (r *ItemRepositoryImpl) DeleteItem(extId string) error {
	// IDF deletion is typically handled by setting a flag or using a delete operation
	// For now, we'll log a warning as IDF deletion patterns vary
	log.Warnf("DeleteItem not yet implemented for IDF. ExtId: %s", extId)
	return fmt.Errorf("delete operation not yet implemented")
}

// fetchAssociationsForItems fetches associations from IDF for a list of items
// Returns a map of item extId -> list of associations
func (r *ItemRepositoryImpl) fetchAssociationsForItems(items []*pb.Item) (map[string][]map[string]interface{}, error) {
	if len(items) == 0 {
		return make(map[string][]map[string]interface{}), nil
	}

	// Collect all item extIds
	extIds := make([]string, 0, len(items))
	for _, item := range items {
		if item.ExtId != nil && *item.ExtId != "" {
			extIds = append(extIds, *item.ExtId)
		}
	}

	if len(extIds) == 0 {
		return make(map[string][]map[string]interface{}), nil
	}

	// Query item_associations entity from IDF
	// Filter by item_id IN (extIds)
	idfClient := external.Interfaces().IdfClient()

	// Build query to get all associations for these items
	// We'll query item_associations entity and filter by item_id
	query, err := idfQr.QUERY("itemAssociationsListQuery").
		FROM("item_associations").
		Proto()
	if err != nil {
		return nil, fmt.Errorf("failed to build IDF query for associations: %w", err)
	}

	// Add filter: item_id IN (extIds)
	// Note: IDF query builder syntax may vary, this is a simplified approach
	// In practice, you might need to query each item separately or use a different filter syntax

	// For now, let's query all associations and filter in memory (not ideal but works)
	// Include total_count and average_score columns for stats data
	query.GroupBy = &insights_interface.QueryGroupBy{
		RawColumns: []*insights_interface.QueryRawColumn{
			{Column: proto.String("item_id")},
			{Column: proto.String("entity_type")},
			{Column: proto.String("entity_id")},
			{Column: proto.String("count")},
			{Column: proto.String("total_count")},
			{Column: proto.String("average_score")},
		},
	}

	queryArg := &insights_interface.GetEntitiesWithMetricsArg{
		Query: query,
	}

	queryResponse, err := idfClient.GetEntitiesWithMetricsRet(queryArg)
	if err != nil {
		return nil, fmt.Errorf("failed to query IDF for associations: %w", err)
	}

	// Build map of extId -> associations
	associationsMap := make(map[string][]map[string]interface{})

	groupResults := queryResponse.GetGroupResultsList()
	if len(groupResults) == 0 {
		log.Warnf("⚠️  No group results from IDF for associations query")
		return associationsMap, nil
	}

	log.Debugf("📊 IDF returned %d group results for associations", len(groupResults))

	entitiesWithMetric := groupResults[0].GetRawResults()
	entities := ConvertEntitiesWithMetricToEntities(entitiesWithMetric)

	// Create a set of extIds for fast lookup
	extIdSet := make(map[string]bool)
	for _, extId := range extIds {
		extIdSet[extId] = true
	}

	// Process associations and group by item_id
	log.Debugf("Processing %d association entities from IDF", len(entities))
	totalAssocCount := 0
	for _, entity := range entities {
		var itemId string
		assoc := make(map[string]interface{})

		for _, attr := range entity.GetAttributeDataMap() {
			switch attr.GetName() {
			case "item_id":
				if attr.GetValue() != nil {
					itemId = attr.GetValue().GetStrValue()
				}
			case "entity_type":
				if attr.GetValue() != nil {
					assoc["entityType"] = attr.GetValue().GetStrValue()
				}
			case "entity_id":
				if attr.GetValue() != nil {
					assoc["entityId"] = attr.GetValue().GetStrValue()
				}
			case "count":
				if attr.GetValue() != nil {
					assoc["count"] = int32(attr.GetValue().GetInt64Value())
				}
			case "total_count":
				if attr.GetValue() != nil {
					assoc["totalCount"] = attr.GetValue().GetInt64Value()
				}
			case "average_score":
				if attr.GetValue() != nil {
					assoc["averageScore"] = attr.GetValue().GetDoubleValue()
				}
			}
		}

		// Only include associations for items we're interested in
		if itemId != "" && extIdSet[itemId] {
			associationsMap[itemId] = append(associationsMap[itemId], assoc)
			totalAssocCount++
			log.Debugf("  Added association for item %s: entityType=%v, entityId=%v, count=%v", itemId, assoc["entityType"], assoc["entityId"], assoc["count"])
		} else {
			log.Debugf("  Skipped association (itemId=%s, inSet=%v)", itemId, extIdSet[itemId])
		}
	}

	log.Infof("✅ Fetched associations for %d items from IDF (total associations: %d)", len(associationsMap), totalAssocCount)
	for extId, assocs := range associationsMap {
		log.Debugf("  Item %s: %d associations", extId, len(assocs))
	}
	return associationsMap, nil
}

// fetchItemStatsForItems fetches itemStats from IDF for a list of items
// expandOptions may contain time-series parameters ($startTime, $endTime, $statType, $samplingInterval)
// Returns a map of item extId -> list of ItemStats protobuf objects
func (r *ItemRepositoryImpl) fetchItemStatsForItems(items []*pb.Item, expandOptions *ExpandOptions) (map[string][]*statsPb.ItemStats, error) {
	if len(items) == 0 {
		return make(map[string][]*statsPb.ItemStats), nil
	}

	// Collect all item extIds
	extIds := make([]string, 0, len(items))
	for _, item := range items {
		if item.ExtId != nil && *item.ExtId != "" {
			extIds = append(extIds, *item.ExtId)
		}
	}

	if len(extIds) == 0 {
		return make(map[string][]*statsPb.ItemStats), nil
	}

	// CRITICAL: GraphQL doesn't support 'item_stats' entity type (returns null for time-series metrics)
	// We must use IDF protobuf query, which returns the latest value per metric
	// IDF protobuf works but only returns latest value (not multiple values in time range)

	// Set default time range if not provided
	// IDF's default behavior is VERY restrictive - only returns data from last few minutes
	// We need to ensure we query with a reasonable time range
	if expandOptions == nil {
		expandOptions = &ExpandOptions{}
	}

	// CRITICAL: IDF without time range only returns data from last few minutes
	// Always set a default time range (last 7 days) to ensure data is returned
	if expandOptions.StartTime == nil && expandOptions.EndTime == nil {
		now := time.Now().Unix() * 1000                 // Current time in milliseconds
		sevenDaysAgo := now - (7 * 24 * 60 * 60 * 1000) // 7 days ago
		expandOptions.StartTime = &sevenDaysAgo
		expandOptions.EndTime = &now
		log.Infof("🔍 [fetchItemStatsForItems] Using default time range: last 7 days (IDF default is too restrictive)")
		log.Infof("🔍 [fetchItemStatsForItems] Time range: %s to %s",
			time.Unix(*expandOptions.StartTime/1000, 0).UTC().Format(time.RFC3339),
			time.Unix(*expandOptions.EndTime/1000, 0).UTC().Format(time.RFC3339))
		log.Warnf("⚠️  [fetchItemStatsForItems] NOTE: Time range is set but not passed to IDF protobuf query")
		log.Warnf("⚠️  IDF protobuf Query doesn't support TimeRange field - will return only latest value")
		log.Warnf("⚠️  For reliable results, use GraphQL with time range (requires item_stats registration)")
	} else {
		// Log explicit time range
		log.Infof("🔍 [fetchItemStatsForItems] Using explicit time range")
		if expandOptions.StartTime != nil {
			log.Infof("🔍 [fetchItemStatsForItems] StartTime: %s (%d ms)",
				time.Unix(*expandOptions.StartTime/1000, 0).UTC().Format(time.RFC3339),
				*expandOptions.StartTime)
		}
		if expandOptions.EndTime != nil {
			log.Infof("🔍 [fetchItemStatsForItems] EndTime: %s (%d ms)",
				time.Unix(*expandOptions.EndTime/1000, 0).UTC().Format(time.RFC3339),
				*expandOptions.EndTime)
		}
		log.Warnf("⚠️  [fetchItemStatsForItems] Time range parameters are logged but not passed to IDF protobuf query")
		log.Warnf("⚠️  IDF protobuf Query doesn't support TimeRange field - will return only latest value")
	}

	// Try GraphQL first (in case item_stats gets registered in future)
	// But expect it to fail/return null and fallback to IDF
	log.Infof("🔍 [fetchItemStatsForItems] Attempting GraphQL query (will fallback to IDF if item_stats not registered)")
	itemStatsMap, err := r.fetchItemStatsForItemsWithGraphQL(items, extIds, expandOptions)
	if err == nil {
		// Check if GraphQL returned any time-series data
		hasData := false
		for _, statsList := range itemStatsMap {
			for _, stat := range statsList {
				if (stat.GetAge() != nil && len(stat.GetAge().GetValue()) > 0) ||
					(stat.GetHeartRate() != nil && len(stat.GetHeartRate().GetValue()) > 0) ||
					(stat.GetFoodIntake() != nil && len(stat.GetFoodIntake().GetValue()) > 0) {
					hasData = true
					break
				}
			}
			if hasData {
				break
			}
		}
		if hasData {
			log.Infof("✅ [fetchItemStatsForItems] GraphQL returned time-series data")
			return itemStatsMap, nil
		}
		log.Warnf("⚠️  [fetchItemStatsForItems] GraphQL returned no time-series data, falling back to IDF protobuf")
	}

	// Fallback to IDF protobuf query (returns only latest value per metric)
	log.Infof("🔍 [fetchItemStatsForItems] Using IDF protobuf query (returns only latest value per metric)")
	return r.fetchItemStatsForItemsWithIDF(items, extIds, expandOptions)
}

// fetchItemStatsForItemsWithIDF is the original IDF protobuf query implementation
// Extracted to allow fallback from GraphQL
func (r *ItemRepositoryImpl) fetchItemStatsForItemsWithIDF(items []*pb.Item, extIds []string, expandOptions *ExpandOptions) (map[string][]*statsPb.ItemStats, error) {
	log.Infof("🔍 [fetchItemStatsForItemsWithIDF] Using IDF protobuf query (returns only latest values)")

	// Query item_stats entity from IDF
	idfClient := external.Interfaces().IdfClient()

	// Build query to get all itemStats for these items
	query, err := idfQr.QUERY("itemStatsListQuery").
		FROM("item_stats").
		Proto()
	if err != nil {
		return nil, fmt.Errorf("failed to build IDF query for itemStats: %w", err)
	}

	// CRITICAL: Set EntityList with EntityTypeName explicitly
	// The query builder's FROM() might not set EntityList correctly
	// This is required for IDF to return time-series metrics
	if len(query.EntityList) == 0 {
		query.EntityList = []*insights_interface.EntityGuid{
			{
				EntityTypeName: proto.String("item_stats"),
			},
		}
		log.Infof("🔍 [fetchItemStatsForItemsWithIDF] Set EntityList with entity_type_name: item_stats")
	} else {
		// Ensure EntityTypeName is set correctly
		for _, eGuid := range query.EntityList {
			if eGuid.GetEntityTypeName() == "" {
				eGuid.EntityTypeName = proto.String("item_stats")
				log.Infof("🔍 [fetchItemStatsForItemsWithIDF] Set EntityTypeName on existing EntityGuid")
			}
		}
	}

	// Include all item_stats columns (attributes and time-series metrics)
	// Note: age, heart_rate, food_intake are time-series metrics (is_attribute: false)
	// They will be returned in MetricDataList and converted to AttributeDataMap
	rawColumns := []*insights_interface.QueryRawColumn{
		{Column: proto.String("stats_ext_id")}, // Attribute
		{Column: proto.String("item_ext_id")},  // Attribute
	}

	// Handle time-series parameters for metrics
	// Apply time range and aggregation to IDF query if provided
	var statType string
	var startTimeMs, endTimeMs *int64

	if expandOptions != nil {
		if expandOptions.StartTime != nil {
			startTimeMs = expandOptions.StartTime
			log.Infof("🔍 [fetchItemStatsForItems] Using $startTime: %d ms (%s)", *startTimeMs,
				time.Unix(*startTimeMs/1000, 0).UTC().Format(time.RFC3339))
		}
		if expandOptions.EndTime != nil {
			endTimeMs = expandOptions.EndTime
			log.Infof("🔍 [fetchItemStatsForItems] Using $endTime: %d ms (%s)", *endTimeMs,
				time.Unix(*endTimeMs/1000, 0).UTC().Format(time.RFC3339))
		}
		if expandOptions.StatType != nil {
			statType = *expandOptions.StatType
			log.Infof("🔍 [fetchItemStatsForItems] Using $statType: %s", statType)
			log.Warnf("⚠️  [fetchItemStatsForItems] Aggregation type is logged but not yet passed to IDF query")
			log.Warnf("⚠️  [fetchItemStatsForItems] Full aggregation support requires GraphQL or MetricType_Operator (see apply_visitor_utils.go)")
		}
		if expandOptions.SamplingInterval != nil {
			log.Infof("🔍 [fetchItemStatsForItems] Using $samplingInterval: %d seconds", *expandOptions.SamplingInterval)
		}
	}

	// If no time range provided, set default to last 1 hour to get multiple values
	// IDF returns only latest value without time range, so we need a default range
	if startTimeMs == nil || endTimeMs == nil {
		now := time.Now().Unix() * 1000 // Current time in milliseconds
		if endTimeMs == nil {
			endTimeMs = &now
		}
		if startTimeMs == nil {
			// Default: last 1 hour
			oneHourAgo := now - (60 * 60 * 1000)
			startTimeMs = &oneHourAgo
		}
		log.Infof("🔍 [fetchItemStatsForItems] Using default time range: %s to %s (last 1 hour)",
			time.Unix(*startTimeMs/1000, 0).UTC().Format(time.RFC3339),
			time.Unix(*endTimeMs/1000, 0).UTC().Format(time.RFC3339))
		log.Infof("✅ [fetchItemStatsForItems] Time range will be set in IDF Query protobuf (start_time_usecs, end_time_usecs)")
	}

	// Add time-series metrics with aggregation type if specified
	// For time-series metrics, we can specify aggregation in the column name or use StatType
	if statType != "" {
		// Add metrics with aggregation type
		// Format: "age:AVG" or just use StatType in query structure
		rawColumns = append(rawColumns,
			&insights_interface.QueryRawColumn{
				Column: proto.String("age"),
				// Note: IDF might support aggregation via StatType field or column name format
				// If QueryRawColumn has StatType field, set it here
			},
			&insights_interface.QueryRawColumn{
				Column: proto.String("heart_rate"),
			},
			&insights_interface.QueryRawColumn{
				Column: proto.String("food_intake"),
			},
		)
		// TODO: Map statType to IDF aggregation type
		// IDF uses MetricType_Operator enum (kAvg, kMin, kMax, kSum, kCount) - see apply_visitor_utils.go
		// For now, aggregation is logged but not applied to query
		log.Infof("🔍 [fetchItemStatsForItems] Aggregation type %s will be applied to time-series metrics (when GraphQL is implemented)", statType)
	} else {
		// No aggregation - just add metrics normally
		rawColumns = append(rawColumns,
			&insights_interface.QueryRawColumn{Column: proto.String("age")},
			&insights_interface.QueryRawColumn{Column: proto.String("heart_rate")},
			&insights_interface.QueryRawColumn{Column: proto.String("food_intake")},
		)
	}

	query.GroupBy = &insights_interface.QueryGroupBy{
		RawColumns: rawColumns,
	}

	// CRITICAL: Set time range in Query protobuf (start_time_usecs and end_time_usecs)
	// IDF requires time range for is_attribute=false metrics to appear
	if startTimeMs != nil {
		startTimeUsecs := uint64(*startTimeMs * 1000) // Convert ms to usecs, then to uint64
		query.StartTimeUsecs = &startTimeUsecs
		log.Infof("✅ [fetchItemStatsForItemsWithIDF] Set query.StartTimeUsecs: %d usecs (%s)",
			startTimeUsecs,
			time.Unix(int64(startTimeUsecs/1000000), 0).UTC().Format(time.RFC3339))
	}
	if endTimeMs != nil {
		endTimeUsecs := uint64(*endTimeMs * 1000) // Convert ms to usecs, then to uint64
		query.EndTimeUsecs = &endTimeUsecs
		log.Infof("✅ [fetchItemStatsForItemsWithIDF] Set query.EndTimeUsecs: %d usecs (%s)",
			endTimeUsecs,
			time.Unix(int64(endTimeUsecs/1000000), 0).UTC().Format(time.RFC3339))
	}

	// Debug: Log the actual query structure
	log.Infof("🔍 [fetchItemStatsForItemsWithIDF] IDF Query structure:")
	log.Infof("   EntityList count: %d", len(query.EntityList))
	for i, eGuid := range query.EntityList {
		log.Infof("   EntityList[%d]: EntityTypeName=%s", i, eGuid.GetEntityTypeName())
	}
	log.Infof("   GroupBy.RawColumns count: %d", len(query.GroupBy.RawColumns))
	for i, col := range query.GroupBy.RawColumns {
		log.Infof("   RawColumns[%d]: column=%s", i, col.GetColumn())
	}
	if query.StartTimeUsecs != nil {
		log.Infof("   StartTimeUsecs: %d usecs", *query.StartTimeUsecs)
	}
	if query.EndTimeUsecs != nil {
		log.Infof("   EndTimeUsecs: %d usecs", *query.EndTimeUsecs)
	}
	log.Infof("   Query string: %s", query.String())

	log.Infof("🔍 [fetchItemStatsForItems] Querying IDF for itemStats with columns: stats_ext_id, item_ext_id, age, heart_rate, food_intake")
	log.Infof("🔍 [fetchItemStatsForItems] Requesting itemStats for %d items: %v", len(extIds), extIds)

	queryArg := &insights_interface.GetEntitiesWithMetricsArg{
		Query: query,
	}

	queryResponse, err := idfClient.GetEntitiesWithMetricsRet(queryArg)
	if err != nil {
		return nil, fmt.Errorf("failed to query IDF for itemStats: %w", err)
	}

	// Build map of extId -> itemStats
	itemStatsMap := make(map[string][]*statsPb.ItemStats)

	groupResults := queryResponse.GetGroupResultsList()
	if len(groupResults) == 0 {
		log.Warnf("⚠️  No group results from IDF for itemStats query")
		return itemStatsMap, nil
	}

	log.Debugf("📊 IDF returned %d group results for itemStats", len(groupResults))

	entitiesWithMetric := groupResults[0].GetRawResults()

	// Debug: Log what IDF returned before conversion
	log.Infof("🔍 [fetchItemStatsForItems] IDF returned %d EntityWithMetric objects", len(entitiesWithMetric))
	if len(entitiesWithMetric) == 0 {
		log.Warnf("⚠️  [fetchItemStatsForItems] No EntityWithMetric objects returned from IDF!")
		log.Warnf("⚠️  This could mean: 1) No itemStats exist, 2) Query didn't match any entities, 3) IDF query issue")
	}

	for i, ewm := range entitiesWithMetric {
		entityId := ewm.GetEntityGuid().GetEntityId()
		metricList := ewm.GetMetricDataList()
		log.Infof("  EntityWithMetric[%d]: entityId=%s, metricCount=%d", i, entityId, len(metricList))

		// Note: We'll filter by item_ext_id later after converting to Entity
		// For now, just log the metrics

		if len(metricList) == 0 {
			log.Warnf("    ⚠️  EntityWithMetric[%d] has NO metrics in MetricDataList!", i)
			log.Warnf("    ⚠️  This means IDF didn't return any metrics for this entity")
		} else {
			for j, metric := range metricList {
				metricName := metric.GetName()
				valueList := metric.GetValueList()
				log.Infof("    Metric[%d]: name=%s, valueCount=%d", j, metricName, len(valueList))
				if len(valueList) == 0 {
					log.Warnf("      ⚠️  Metric '%s' has NO values in ValueList!", metricName)
				} else {
					val := valueList[0].GetValue()
					if val != nil {
						if intVal := val.GetInt64Value(); intVal != 0 {
							log.Infof("      ✅ First value: int64=%d", intVal)
						} else if doubleVal := val.GetDoubleValue(); doubleVal != 0 {
							log.Infof("      ✅ First value: double=%f", doubleVal)
						} else if strVal := val.GetStrValue(); strVal != "" {
							log.Infof("      ✅ First value: string=%s", strVal)
						} else {
							log.Warnf("      ⚠️  Metric '%s' value is nil or zero", metricName)
						}
					} else {
						log.Warnf("      ⚠️  Metric '%s' Value is nil", metricName)
					}
				}
			}
		}
	}

	entities := ConvertEntitiesWithMetricToEntities(entitiesWithMetric)

	// Create a set of extIds for fast lookup
	extIdSet := make(map[string]bool)
	for _, extId := range extIds {
		extIdSet[extId] = true
	}

	// Process itemStats and group by item_ext_id
	log.Debugf("Processing %d itemStats entities from IDF", len(entities))
	totalStatsCount := 0
	for _, entity := range entities {
		var itemExtId string
		stat := &statsPb.ItemStats{}

		// Debug: Log all attributes after conversion
		attrMap := entity.GetAttributeDataMap()
		log.Infof("  🔍 Entity has %d attributes after conversion", len(attrMap))
		for _, attr := range attrMap {
			attrName := attr.GetName()
			hasValue := attr.GetValue() != nil
			if hasValue {
				val := attr.GetValue()
				if intVal := val.GetInt64Value(); intVal != 0 {
					log.Infof("    ✅ Attribute: %s = %d (int64)", attrName, intVal)
				} else if doubleVal := val.GetDoubleValue(); doubleVal != 0 {
					log.Infof("    ✅ Attribute: %s = %f (double)", attrName, doubleVal)
				} else if strVal := val.GetStrValue(); strVal != "" {
					log.Infof("    ✅ Attribute: %s = %s (string)", attrName, strVal)
				} else {
					log.Warnf("    ⚠️  Attribute: %s has value but it's zero/nil", attrName)
				}
			} else {
				log.Warnf("    ⚠️  Attribute: %s has NO value", attrName)
			}
		}

		// Extract timestamps from original EntityWithMetric before conversion
		// We need to map entity back to EntityWithMetric to get all time-series values
		var entityWithMetric *insights_interface.EntityWithMetric
		for _, ewm := range entitiesWithMetric {
			if ewm.GetEntityGuid().GetEntityId() == entity.GetEntityGuid().GetEntityId() {
				entityWithMetric = ewm
				break
			}
		}

		// Extract attributes (stats_ext_id, item_ext_id) from converted entity
		for _, attr := range entity.GetAttributeDataMap() {
			switch attr.GetName() {
			case "item_ext_id":
				if attr.GetValue() != nil {
					itemExtId = attr.GetValue().GetStrValue()
				}
			case "stats_ext_id":
				if attr.GetValue() != nil {
					val := attr.GetValue().GetStrValue()
					stat.StatsExtId = &val
				}
			}
		}

		// Extract time-series metrics from EntityWithMetric (not from converted entity)
		// These are stored as arrays of time-value pairs
		if entityWithMetric != nil {
			log.Infof("    📊 Found EntityWithMetric with %d metrics", len(entityWithMetric.GetMetricDataList()))

			for _, metric := range entityWithMetric.GetMetricDataList() {
				metricName := metric.GetName()
				valueList := metric.GetValueList()
				log.Infof("    📊 Processing metric '%s' with %d values", metricName, len(valueList))

				if len(valueList) == 0 {
					log.Warnf("    ⚠️  Metric '%s' has no values in ValueList", metricName)
					continue
				}

				switch metricName {
				case "age":
					// Create array of IntegerTimeValuePair
					agePairs := make([]*statsPb.IntegerTimeValuePair, 0, len(valueList))
					if expandOptions != nil && (expandOptions.StartTime != nil || expandOptions.EndTime != nil) {
						log.Infof("    🔍 [IDF] Filtering age: startTime=%v, endTime=%v, total values=%d",
							expandOptions.StartTime, expandOptions.EndTime, len(valueList))
					}
					for _, val := range valueList {
						timestampUsecs := val.GetTimestampUsecs()
						seconds := int64(timestampUsecs / 1000000)
						nanos := int64((timestampUsecs % 1000000) * 1000)
						timestamp := timestamppb.New(time.Unix(seconds, nanos))

						// Filter by time range if specified
						// timestampUsecs is in microseconds, convert to milliseconds for comparison
						if expandOptions != nil {
							timestampMs := int64(timestampUsecs / 1000)
							if expandOptions.StartTime != nil && timestampMs < *expandOptions.StartTime {
								continue // Skip if before start time
							}
							if expandOptions.EndTime != nil && timestampMs > *expandOptions.EndTime {
								continue // Skip if after end time
							}
						}

						valInt32 := int32(val.GetValue().GetInt64Value())
						pair := &statsPb.IntegerTimeValuePair{
							Timestamp: timestamp,
							Value:     &valInt32,
						}
						agePairs = append(agePairs, pair)
					}
					if len(agePairs) > 0 {
						stat.Age = &statsPb.IntegerTimeValuePairArrayWrapper{
							Value: agePairs,
						}
						log.Infof("    ✅ Set age: %d time-value pairs (filtered by time range)", len(agePairs))
					}

				case "heart_rate":
					// Create array of IntegerTimeValuePair
					heartRatePairs := make([]*statsPb.IntegerTimeValuePair, 0, len(valueList))
					for _, val := range valueList {
						timestampUsecs := val.GetTimestampUsecs()
						seconds := int64(timestampUsecs / 1000000)
						nanos := int64((timestampUsecs % 1000000) * 1000)
						timestamp := timestamppb.New(time.Unix(seconds, nanos))

						// Filter by time range if specified
						// timestampUsecs is in microseconds, convert to milliseconds for comparison
						if expandOptions != nil {
							timestampMs := int64(timestampUsecs / 1000)
							if expandOptions.StartTime != nil && timestampMs < *expandOptions.StartTime {
								continue // Skip if before start time
							}
							if expandOptions.EndTime != nil && timestampMs > *expandOptions.EndTime {
								continue // Skip if after end time
							}
						}

						valInt32 := int32(val.GetValue().GetInt64Value())
						pair := &statsPb.IntegerTimeValuePair{
							Timestamp: timestamp,
							Value:     &valInt32,
						}
						heartRatePairs = append(heartRatePairs, pair)
					}
					if len(heartRatePairs) > 0 {
						stat.HeartRate = &statsPb.IntegerTimeValuePairArrayWrapper{
							Value: heartRatePairs,
						}
						log.Infof("    ✅ Set heartRate: %d time-value pairs (filtered by time range)", len(heartRatePairs))
					}

				case "food_intake":
					// Create array of DoubleTimeValuePair
					foodIntakePairs := make([]*statsPb.DoubleTimeValuePair, 0, len(valueList))
					for _, val := range valueList {
						timestampUsecs := val.GetTimestampUsecs()
						seconds := int64(timestampUsecs / 1000000)
						nanos := int64((timestampUsecs % 1000000) * 1000)
						timestamp := timestamppb.New(time.Unix(seconds, nanos))

						// Filter by time range if specified
						// timestampUsecs is in microseconds, convert to milliseconds for comparison
						if expandOptions != nil {
							timestampMs := int64(timestampUsecs / 1000)
							if expandOptions.StartTime != nil && timestampMs < *expandOptions.StartTime {
								continue // Skip if before start time
							}
							if expandOptions.EndTime != nil && timestampMs > *expandOptions.EndTime {
								continue // Skip if after end time
							}
						}

						valDouble := val.GetValue().GetDoubleValue()
						pair := &statsPb.DoubleTimeValuePair{
							Timestamp: timestamp,
							Value:     &valDouble,
						}
						foodIntakePairs = append(foodIntakePairs, pair)
					}
					if len(foodIntakePairs) > 0 {
						stat.FoodIntake = &statsPb.DoubleTimeValuePairArrayWrapper{
							Value: foodIntakePairs,
						}
						log.Infof("    ✅ Set foodIntake: %d time-value pairs (filtered by time range)", len(foodIntakePairs))
					}
				}
			}
		} else {
			log.Warnf("    ⚠️  EntityWithMetric is nil - cannot extract time-series metrics")
		}

		// Only include itemStats for items we're interested in
		if itemExtId != "" && extIdSet[itemExtId] {
			// Debug: Log time-value pair arrays before adding to map
			if stat.GetAge() != nil && len(stat.GetAge().GetValue()) > 0 {
				log.Infof("  ✅ itemStats for %s has age: %d time-value pairs", itemExtId, len(stat.GetAge().GetValue()))
			} else {
				log.Warnf("  ⚠️  itemStats for %s has NO age time-value pairs", itemExtId)
			}
			if stat.GetHeartRate() != nil && len(stat.GetHeartRate().GetValue()) > 0 {
				log.Infof("  ✅ itemStats for %s has heartRate: %d time-value pairs", itemExtId, len(stat.GetHeartRate().GetValue()))
			} else {
				log.Warnf("  ⚠️  itemStats for %s has NO heartRate time-value pairs", itemExtId)
			}
			if stat.GetFoodIntake() != nil && len(stat.GetFoodIntake().GetValue()) > 0 {
				log.Infof("  ✅ itemStats for %s has foodIntake: %d time-value pairs", itemExtId, len(stat.GetFoodIntake().GetValue()))
			} else {
				log.Warnf("  ⚠️  itemStats for %s has NO foodIntake time-value pairs", itemExtId)
			}
			itemStatsMap[itemExtId] = append(itemStatsMap[itemExtId], stat)
			totalStatsCount++
			ageCount := 0
			heartRateCount := 0
			foodIntakeCount := 0
			if stat.GetAge() != nil {
				ageCount = len(stat.GetAge().GetValue())
			}
			if stat.GetHeartRate() != nil {
				heartRateCount = len(stat.GetHeartRate().GetValue())
			}
			if stat.GetFoodIntake() != nil {
				foodIntakeCount = len(stat.GetFoodIntake().GetValue())
			}
			log.Debugf("  Added itemStats for item %s: statsExtId=%v, age=%d pairs, heartRate=%d pairs, foodIntake=%d pairs",
				itemExtId, stat.StatsExtId, ageCount, heartRateCount, foodIntakeCount)
		} else {
			log.Debugf("  Skipped itemStats (itemExtId=%s, inSet=%v)", itemExtId, extIdSet[itemExtId])
		}
	}

	log.Infof("✅ [fetchItemStatsForItems] Fetched itemStats for %d items from IDF (total itemStats records: %d)", len(itemStatsMap), totalStatsCount)
	if totalStatsCount == 0 {
		log.Warnf("⚠️  [fetchItemStatsForItems] No itemStats records found in IDF for any of the %d requested items", len(extIds))
		log.Warnf("⚠️  [fetchItemStatsForItems] Requested extIds: %v", extIds)
	}
	for extId, stats := range itemStatsMap {
		log.Debugf("  [fetchItemStatsForItems] Item %s: %d itemStats", extId, len(stats))
	}
	return itemStatsMap, nil
}

// buildItemStatsGraphQLQuery builds a GraphQL query for item_stats with time range support
// Format: query { item_stats(args: {interval_start_ms: X, interval_end_ms: Y, ...}) { age(timeseries: true), heart_rate(timeseries: true), food_intake(timeseries: true), item_ext_id, stats_ext_id, _entity_id_ } }
func buildItemStatsGraphQLQuery(extIds []string, expandOptions *ExpandOptions) string {
	var query strings.Builder
	query.Grow(1000)

	// Start GraphQL query
	query.WriteString("query { item_stats")

	// Build args section
	query.WriteString("(args:{")

	// Add query name
	queryName := fmt.Sprintf("itemStatsQuery-%d", time.Now().UnixNano())
	query.WriteString(fmt.Sprintf("query_name:\"%s\"", queryName))

	// Add time range if provided
	if expandOptions != nil {
		if expandOptions.StartTime != nil {
			query.WriteString(fmt.Sprintf(",interval_start_ms:%d", *expandOptions.StartTime))
		}
		if expandOptions.EndTime != nil {
			query.WriteString(fmt.Sprintf(",interval_end_ms:%d", *expandOptions.EndTime))
		}
		if expandOptions.SamplingInterval != nil {
			query.WriteString(fmt.Sprintf(",downsampling_interval_secs:%d", *expandOptions.SamplingInterval))
		}
	}

	// Add filter for item_ext_id IN (extIds)
	// Limit to first 50 extIds to avoid query size issues
	maxExtIds := 50
	if len(extIds) > 0 {
		extIdsToUse := extIds
		if len(extIds) > maxExtIds {
			log.Warnf("⚠️  [buildItemStatsGraphQLQuery] Limiting filter to first %d extIds (requested %d) to avoid query size issues", maxExtIds, len(extIds))
			extIdsToUse = extIds[:maxExtIds]
		}

		// Build OData filter: item_ext_id eq 'extId1' or item_ext_id eq 'extId2' ...
		filterParts := make([]string, 0, len(extIdsToUse))
		for _, extId := range extIdsToUse {
			filterParts = append(filterParts, fmt.Sprintf("item_ext_id eq '%s'", extId))
		}
		filterCriteria := strings.Join(filterParts, " or ")
		query.WriteString(fmt.Sprintf(",odata_filter_criteria:\"%s\"", filterCriteria))
		log.Infof("🔍 [buildItemStatsGraphQLQuery] Added filter for %d item extIds", len(extIdsToUse))
	}

	query.WriteString("})")

	// Build select fields
	query.WriteString("{")

	// Add time-series metrics with timeseries:true
	// Note: GraphQL may require sampling parameter for timeseries queries
	// Try with sampling:LAST first (returns latest values)
	query.WriteString("age(sampling:LAST,timeseries:true)")
	query.WriteString(",heart_rate(sampling:LAST,timeseries:true)")
	query.WriteString(",food_intake(sampling:LAST,timeseries:true)")

	// Add attributes
	query.WriteString(",item_ext_id")
	query.WriteString(",stats_ext_id")
	query.WriteString(",_entity_id_")

	query.WriteString("}}")

	graphqlQuery := query.String()
	log.Infof("🔍 [buildItemStatsGraphQLQuery] Generated GraphQL query: %s", graphqlQuery)
	return graphqlQuery
}

// fetchItemStatsForItemsWithGraphQL fetches itemStats using GraphQL query with time range support
// This enables multiple values per metric (time-series arrays)
func (r *ItemRepositoryImpl) fetchItemStatsForItemsWithGraphQL(items []*pb.Item, extIds []string, expandOptions *ExpandOptions) (map[string][]*statsPb.ItemStats, error) {
	log.Infof("🔍 [fetchItemStatsForItemsWithGraphQL] Using GraphQL to fetch itemStats with time range")

	// Build GraphQL query
	graphqlQuery := buildItemStatsGraphQLQuery(extIds, expandOptions)

	// Execute GraphQL query via statsGW
	statsGWClient := external.Interfaces().StatsGWClient()
	if statsGWClient == nil {
		return nil, fmt.Errorf("statsGW client not available, cannot execute GraphQL query")
	}

	graphqlRet, err := statsGWClient.ExecuteGraphql(context.Background(), graphqlQuery)
	if err != nil {
		log.Errorf("❌ [fetchItemStatsForItemsWithGraphQL] GraphQL query failed: %v", err)
		log.Warnf("⚠️  [fetchItemStatsForItemsWithGraphQL] Falling back to IDF protobuf query (will return only latest values)")
		// Fallback to regular IDF query
		return r.fetchItemStatsForItemsWithIDF(items, extIds, expandOptions)
	}

	// Parse GraphQL response
	graphqlData := graphqlRet.GetData()
	if graphqlData == "" {
		log.Warnf("⚠️  [fetchItemStatsForItemsWithGraphQL] GraphQL returned empty data")
		return make(map[string][]*statsPb.ItemStats), nil
	}

	log.Infof("🔍 [fetchItemStatsForItemsWithGraphQL] GraphQL response data length: %d", len(graphqlData))
	log.Infof("🔍 [fetchItemStatsForItemsWithGraphQL] GraphQL response data (first 500 chars): %s",
		func() string {
			if len(graphqlData) > 500 {
				return graphqlData[:500] + "..."
			}
			return graphqlData
		}())

	// Parse GraphQL JSON response
	itemStatsMap, err := r.parseItemStatsGraphQLResponse(graphqlData, extIds, expandOptions)
	if err != nil {
		log.Errorf("❌ [fetchItemStatsForItemsWithGraphQL] Failed to parse GraphQL response: %v", err)
		log.Warnf("⚠️  [fetchItemStatsForItemsWithGraphQL] Falling back to IDF protobuf query")
		// Fallback to IDF protobuf query
		return r.fetchItemStatsForItemsWithIDF(items, extIds, expandOptions)
	}

	// Check if GraphQL returned any time-series data
	hasTimeSeriesData := false
	for _, statsList := range itemStatsMap {
		for _, stat := range statsList {
			if stat.GetAge() != nil && len(stat.GetAge().GetValue()) > 0 {
				hasTimeSeriesData = true
				break
			}
			if stat.GetHeartRate() != nil && len(stat.GetHeartRate().GetValue()) > 0 {
				hasTimeSeriesData = true
				break
			}
			if stat.GetFoodIntake() != nil && len(stat.GetFoodIntake().GetValue()) > 0 {
				hasTimeSeriesData = true
				break
			}
		}
		if hasTimeSeriesData {
			break
		}
	}

	if !hasTimeSeriesData && len(itemStatsMap) > 0 {
		log.Warnf("⚠️  [fetchItemStatsForItemsWithGraphQL] GraphQL returned entities but no time-series data")
		log.Warnf("⚠️  This likely means 'item_stats' entity type is not registered in GraphQL schema")
		log.Warnf("⚠️  Falling back to IDF protobuf query (will return only latest values)")
		// Fallback to IDF protobuf query
		return r.fetchItemStatsForItemsWithIDF(items, extIds, expandOptions)
	}

	log.Infof("✅ [fetchItemStatsForItemsWithGraphQL] Successfully fetched itemStats via GraphQL for %d items", len(itemStatsMap))
	return itemStatsMap, nil
}

// ItemStatsGraphQLDto represents the GraphQL response structure for item_stats
type ItemStatsGraphQLDto struct {
	ItemStats []ItemStatsGraphQLItemDto `json:"item_stats"`
}

// ItemStatsGraphQLItemDto represents a single item_stats entity in GraphQL response
type ItemStatsGraphQLItemDto struct {
	Age        []ItemStatsTimeValuePair `json:"age"`
	HeartRate  []ItemStatsTimeValuePair `json:"heart_rate"`
	FoodIntake []ItemStatsTimeValuePair `json:"food_intake"`
	ItemExtId  []string                 `json:"item_ext_id"`
	StatsExtId []string                 `json:"stats_ext_id"`
	EntityId   []string                 `json:"_entity_id_"`
}

// ItemStatsTimeValuePair represents a time-value pair in GraphQL response
type ItemStatsTimeValuePair struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

// parseItemStatsGraphQLResponse parses GraphQL JSON response to ItemStats protobuf objects
func (r *ItemRepositoryImpl) parseItemStatsGraphQLResponse(graphqlData string, extIds []string, expandOptions *ExpandOptions) (map[string][]*statsPb.ItemStats, error) {
	itemStatsMap := make(map[string][]*statsPb.ItemStats)

	// Debug: Log raw JSON before parsing
	log.Infof("🔍 [parseItemStatsGraphQLResponse] Raw GraphQL JSON response: %s", graphqlData)

	// Try to parse as generic map first to see the structure
	var rawResponse map[string]interface{}
	if err := json.Unmarshal([]byte(graphqlData), &rawResponse); err == nil {
		log.Infof("🔍 [parseItemStatsGraphQLResponse] GraphQL response structure (keys): %v", func() []string {
			keys := make([]string, 0, len(rawResponse))
			for k := range rawResponse {
				keys = append(keys, k)
			}
			return keys
		}())
		if itemStatsRaw, ok := rawResponse["item_stats"]; ok {
			log.Infof("🔍 [parseItemStatsGraphQLResponse] Found 'item_stats' key in response")
			if itemStatsArray, ok := itemStatsRaw.([]interface{}); ok {
				log.Infof("🔍 [parseItemStatsGraphQLResponse] item_stats is array with %d elements", len(itemStatsArray))
				if len(itemStatsArray) > 0 {
					if firstItem, ok := itemStatsArray[0].(map[string]interface{}); ok {
						log.Infof("🔍 [parseItemStatsGraphQLResponse] First item keys: %v", func() []string {
							keys := make([]string, 0, len(firstItem))
							for k := range firstItem {
								keys = append(keys, k)
							}
							return keys
						}())
						if ageRaw, ok := firstItem["age"]; ok {
							log.Infof("🔍 [parseItemStatsGraphQLResponse] 'age' field type: %T, value: %v", ageRaw, ageRaw)
						}
					}
				}
			}
		}
	}

	// Parse JSON response
	var graphqlResp ItemStatsGraphQLDto
	if err := json.Unmarshal([]byte(graphqlData), &graphqlResp); err != nil {
		log.Errorf("❌ [parseItemStatsGraphQLResponse] JSON unmarshal error: %v", err)
		log.Errorf("❌ [parseItemStatsGraphQLResponse] Failed JSON: %s", graphqlData)
		return nil, fmt.Errorf("failed to unmarshal GraphQL JSON response: %w", err)
	}

	log.Infof("🔍 [parseItemStatsGraphQLResponse] Parsed %d item_stats entities from GraphQL", len(graphqlResp.ItemStats))

	// Debug: Log structure of first entity if available
	if len(graphqlResp.ItemStats) > 0 {
		first := graphqlResp.ItemStats[0]
		log.Infof("🔍 [parseItemStatsGraphQLResponse] First entity: item_ext_id=%v, stats_ext_id=%v, age=%d pairs, heart_rate=%d pairs, food_intake=%d pairs",
			first.ItemExtId, first.StatsExtId, len(first.Age), len(first.HeartRate), len(first.FoodIntake))
	}

	// Create a set of extIds for fast lookup
	extIdSet := make(map[string]bool)
	for _, extId := range extIds {
		extIdSet[extId] = true
	}

	// Check if GraphQL returned null for all time-series metrics
	// This indicates GraphQL doesn't support item_stats entity type or metrics aren't registered
	allMetricsNull := true
	for _, itemStatsDto := range graphqlResp.ItemStats {
		if len(itemStatsDto.Age) > 0 {
			allMetricsNull = false
			break
		}
		if len(itemStatsDto.HeartRate) > 0 {
			allMetricsNull = false
			break
		}
		if len(itemStatsDto.FoodIntake) > 0 {
			allMetricsNull = false
			break
		}
	}

	if allMetricsNull && len(graphqlResp.ItemStats) > 0 {
		log.Warnf("⚠️  [parseItemStatsGraphQLResponse] GraphQL returned null for all time-series metrics")
		log.Warnf("⚠️  This likely means 'item_stats' entity type is not registered in GraphQL schema")
		log.Warnf("⚠️  GraphQL can find entities but cannot query time-series metrics")
		log.Warnf("⚠️  Falling back to IDF protobuf query (will return only latest values)")
		// Return empty map to trigger fallback
		return make(map[string][]*statsPb.ItemStats), fmt.Errorf("GraphQL returned null for time-series metrics - entity type may not be registered in GraphQL schema")
	}

	// Convert GraphQL DTOs to protobuf ItemStats
	for _, itemStatsDto := range graphqlResp.ItemStats {
		// Extract item_ext_id (should be single value)
		var itemExtId string
		if len(itemStatsDto.ItemExtId) > 0 {
			itemExtId = itemStatsDto.ItemExtId[0]
		}

		// Only process itemStats for requested items
		if itemExtId == "" || !extIdSet[itemExtId] {
			continue
		}

		stat := &statsPb.ItemStats{}

		// Extract stats_ext_id
		if len(itemStatsDto.StatsExtId) > 0 {
			statsExtId := itemStatsDto.StatsExtId[0]
			stat.StatsExtId = &statsExtId
		}

		// Convert age time-series array
		if len(itemStatsDto.Age) > 0 {
			agePairs := make([]*statsPb.IntegerTimeValuePair, 0, len(itemStatsDto.Age))
			for i, tvp := range itemStatsDto.Age {
				// Debug: Log raw values before conversion
				if i == 0 {
					log.Infof("    🔍 [parseItemStatsGraphQLResponse] First age pair: timestamp=%d, value=%f", tvp.Timestamp, tvp.Value)
				}

				// Check if timestamp is valid (not 0)
				if tvp.Timestamp == 0 {
					log.Warnf("    ⚠️  [parseItemStatsGraphQLResponse] Age pair[%d] has timestamp=0, skipping", i)
					continue
				}

				// Filter by time range if specified
				// GraphQL returns timestamp in milliseconds
				if expandOptions != nil {
					if expandOptions.StartTime != nil && tvp.Timestamp < *expandOptions.StartTime {
						continue // Skip if before start time
					}
					if expandOptions.EndTime != nil && tvp.Timestamp > *expandOptions.EndTime {
						continue // Skip if after end time
					}
				}

				// Convert timestamp (Unix milliseconds) to timestamppb.Timestamp
				seconds := tvp.Timestamp / 1000
				nanos := int64((tvp.Timestamp % 1000) * 1000000)
				timestamp := timestamppb.New(time.Unix(seconds, nanos))

				valInt32 := int32(tvp.Value)
				if valInt32 == 0 && tvp.Value != 0 {
					log.Warnf("    ⚠️  [parseItemStatsGraphQLResponse] Age value %f truncated to 0 (int32)", tvp.Value)
				}

				pair := &statsPb.IntegerTimeValuePair{
					Timestamp: timestamp,
					Value:     &valInt32,
				}
				agePairs = append(agePairs, pair)
			}
			if len(agePairs) > 0 {
				stat.Age = &statsPb.IntegerTimeValuePairArrayWrapper{
					Value: agePairs,
				}
				if expandOptions != nil && (expandOptions.StartTime != nil || expandOptions.EndTime != nil) {
					log.Infof("    ✅ Set age: %d time-value pairs (filtered by time range from %d total)", len(agePairs), len(itemStatsDto.Age))
				} else {
					log.Infof("    ✅ Set age: %d time-value pairs", len(agePairs))
				}
			} else {
				log.Warnf("    ⚠️  [parseItemStatsGraphQLResponse] All age pairs had invalid timestamps (0), skipping")
			}
		}

		// Convert heart_rate time-series array
		if len(itemStatsDto.HeartRate) > 0 {
			heartRatePairs := make([]*statsPb.IntegerTimeValuePair, 0, len(itemStatsDto.HeartRate))
			for i, tvp := range itemStatsDto.HeartRate {
				// Check if timestamp is valid (not 0)
				if tvp.Timestamp == 0 {
					log.Warnf("    ⚠️  [parseItemStatsGraphQLResponse] HeartRate pair[%d] has timestamp=0, skipping", i)
					continue
				}

				// Filter by time range if specified
				// GraphQL returns timestamp in milliseconds
				if expandOptions != nil {
					if expandOptions.StartTime != nil && tvp.Timestamp < *expandOptions.StartTime {
						continue // Skip if before start time
					}
					if expandOptions.EndTime != nil && tvp.Timestamp > *expandOptions.EndTime {
						continue // Skip if after end time
					}
				}

				// Convert timestamp (Unix milliseconds) to timestamppb.Timestamp
				seconds := tvp.Timestamp / 1000
				nanos := int64((tvp.Timestamp % 1000) * 1000000)
				timestamp := timestamppb.New(time.Unix(seconds, nanos))

				valInt32 := int32(tvp.Value)
				pair := &statsPb.IntegerTimeValuePair{
					Timestamp: timestamp,
					Value:     &valInt32,
				}
				heartRatePairs = append(heartRatePairs, pair)
			}
			if len(heartRatePairs) > 0 {
				stat.HeartRate = &statsPb.IntegerTimeValuePairArrayWrapper{
					Value: heartRatePairs,
				}
				if expandOptions != nil && (expandOptions.StartTime != nil || expandOptions.EndTime != nil) {
					log.Infof("    ✅ Set heartRate: %d time-value pairs (filtered by time range from %d total)", len(heartRatePairs), len(itemStatsDto.HeartRate))
				} else {
					log.Infof("    ✅ Set heartRate: %d time-value pairs", len(heartRatePairs))
				}
			} else {
				log.Warnf("    ⚠️  [parseItemStatsGraphQLResponse] All heartRate pairs had invalid timestamps (0), skipping")
			}
		}

		// Convert food_intake time-series array
		if len(itemStatsDto.FoodIntake) > 0 {
			foodIntakePairs := make([]*statsPb.DoubleTimeValuePair, 0, len(itemStatsDto.FoodIntake))
			for i, tvp := range itemStatsDto.FoodIntake {
				// Check if timestamp is valid (not 0)
				if tvp.Timestamp == 0 {
					log.Warnf("    ⚠️  [parseItemStatsGraphQLResponse] FoodIntake pair[%d] has timestamp=0, skipping", i)
					continue
				}

				// Filter by time range if specified
				// GraphQL returns timestamp in milliseconds
				if expandOptions != nil {
					if expandOptions.StartTime != nil && tvp.Timestamp < *expandOptions.StartTime {
						continue // Skip if before start time
					}
					if expandOptions.EndTime != nil && tvp.Timestamp > *expandOptions.EndTime {
						continue // Skip if after end time
					}
				}

				// Convert timestamp (Unix milliseconds) to timestamppb.Timestamp
				seconds := tvp.Timestamp / 1000
				nanos := int64((tvp.Timestamp % 1000) * 1000000)
				timestamp := timestamppb.New(time.Unix(seconds, nanos))

				valDouble := tvp.Value
				pair := &statsPb.DoubleTimeValuePair{
					Timestamp: timestamp,
					Value:     &valDouble,
				}
				foodIntakePairs = append(foodIntakePairs, pair)
			}
			if len(foodIntakePairs) > 0 {
				stat.FoodIntake = &statsPb.DoubleTimeValuePairArrayWrapper{
					Value: foodIntakePairs,
				}
				if expandOptions != nil && (expandOptions.StartTime != nil || expandOptions.EndTime != nil) {
					log.Infof("    ✅ Set foodIntake: %d time-value pairs (filtered by time range from %d total)", len(foodIntakePairs), len(itemStatsDto.FoodIntake))
				} else {
					log.Infof("    ✅ Set foodIntake: %d time-value pairs", len(foodIntakePairs))
				}
			} else {
				log.Warnf("    ⚠️  [parseItemStatsGraphQLResponse] All foodIntake pairs had invalid timestamps (0), skipping")
			}
		}

		itemStatsMap[itemExtId] = append(itemStatsMap[itemExtId], stat)
	}

	return itemStatsMap, nil
}
