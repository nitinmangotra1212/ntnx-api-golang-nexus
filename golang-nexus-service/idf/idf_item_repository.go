/*
 * IDF Repository Implementation for Item Entity
 * Maps between protobuf Item model (camelCase) and IDF attributes (snake_case)
 */

package idf

import (
	"context"
	"fmt"
	"regexp"
	"strings"
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
	stringListAttr = "string_list"
	int64ListAttr  = "int64_list"
	floatListAttr  = "float_list"
	boolListAttr   = "bool_list"
	byteListAttr   = "byte_list"
	enumListAttr   = "enum_list"
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
	if itemEntity.Item.StringList != nil && len(itemEntity.Item.StringList.Value) > 0 {
		AddAttribute(&attributeDataArgList, stringListAttr, itemEntity.Item.StringList.Value)
	}
	if itemEntity.Item.Int64List != nil && len(itemEntity.Item.Int64List.Value) > 0 {
		AddAttribute(&attributeDataArgList, int64ListAttr, itemEntity.Item.Int64List.Value)
	}
	if itemEntity.Item.FloatList != nil && len(itemEntity.Item.FloatList.Value) > 0 {
		AddAttribute(&attributeDataArgList, floatListAttr, itemEntity.Item.FloatList.Value)
	}
	if itemEntity.Item.BoolList != nil && len(itemEntity.Item.BoolList.Value) > 0 {
		AddAttribute(&attributeDataArgList, boolListAttr, itemEntity.Item.BoolList.Value)
	}
	if itemEntity.Item.ByteList != nil && len(itemEntity.Item.ByteList.Value) > 0 {
		AddAttribute(&attributeDataArgList, byteListAttr, itemEntity.Item.ByteList.Value)
	}
	if itemEntity.Item.EnumList != nil && len(itemEntity.Item.EnumList.Value) > 0 {
		AddAttribute(&attributeDataArgList, enumListAttr, itemEntity.Item.EnumList.Value)
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
			itemStatsMap, err := r.fetchItemStatsForItems(items)
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
				itemStatsMap, err := r.fetchItemStatsForItems(items)
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

		case stringListAttr: // "string_list" (IDF) → StringList (protobuf) - []string
			log.Debugf("  🔍 Found string_list attribute")
			if attr.GetValue() == nil {
				log.Debugf("  ⚠️  string_list value is nil")
			} else if attr.GetValue().GetStrList() == nil {
				log.Debugf("  ⚠️  string_list GetStrList() is nil, value type: %T", attr.GetValue().ValueType)
			} else {
				val := attr.GetValue().GetStrList().GetValueList()
				log.Debugf("  ✅ string_list has %d values: %v", len(val), val)
				if len(val) > 0 {
					item.StringList = &pb.StringArrayWrapper{
						Value: val,
					}
					log.Infof("  ✅ Mapped string_list: %v", val)
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

		case floatListAttr: // "float_list" (IDF) → FloatList (protobuf) - []double
			log.Debugf("  🔍 Found float_list attribute")
			if attr.GetValue() == nil {
				log.Debugf("  ⚠️  float_list value is nil")
			} else if attr.GetValue().GetDoubleList() == nil {
				log.Debugf("  ⚠️  float_list GetDoubleList() is nil, value type: %T", attr.GetValue().ValueType)
			} else {
				val := attr.GetValue().GetDoubleList().GetValueList()
				log.Debugf("  ✅ float_list has %d values: %v", len(val), val)
				if len(val) > 0 {
					item.FloatList = &pb.DoubleArrayWrapper{
						Value: val,
					}
					log.Infof("  ✅ Mapped float_list: %v", val)
				}
			}

		case boolListAttr: // "bool_list" (IDF) → BoolList (protobuf) - []bool
			log.Debugf("  🔍 Found bool_list attribute")
			if attr.GetValue() == nil {
				log.Debugf("  ⚠️  bool_list value is nil")
			} else if attr.GetValue().GetBoolList() == nil {
				log.Debugf("  ⚠️  bool_list GetBoolList() is nil, value type: %T", attr.GetValue().ValueType)
			} else {
				val := attr.GetValue().GetBoolList().GetValueList()
				log.Debugf("  ✅ bool_list has %d values: %v", len(val), val)
				if len(val) > 0 {
					item.BoolList = &pb.BooleanArrayWrapper{
						Value: val,
					}
					log.Infof("  ✅ Mapped bool_list: %v", val)
				}
			}

		case byteListAttr: // "byte_list" (IDF) → ByteList (protobuf) - []int32 (byte)
			log.Debugf("  🔍 Found byte_list attribute")
			if attr.GetValue() == nil {
				log.Debugf("  ⚠️  byte_list value is nil")
			} else if attr.GetValue().GetInt64List() == nil {
				log.Debugf("  ⚠️  byte_list GetInt64List() is nil, value type: %T", attr.GetValue().ValueType)
			} else {
				int64List := attr.GetValue().GetInt64List().GetValueList()
				log.Debugf("  ✅ byte_list has %d values: %v", len(int64List), int64List)
				if len(int64List) > 0 {
					// Convert []int64 to []int32 (byte)
					byteList := make([]int32, len(int64List))
					for i, v := range int64List {
						byteList[i] = int32(v)
					}
					item.ByteList = &pb.IntegerArrayWrapper{
						Value: byteList,
					}
					log.Infof("  ✅ Mapped byte_list: %v", byteList)
				}
			}

		case enumListAttr: // "enum_list" (IDF) → EnumList (protobuf) - []string
			log.Debugf("  🔍 Found enum_list attribute")
			if attr.GetValue() == nil {
				log.Debugf("  ⚠️  enum_list value is nil")
			} else if attr.GetValue().GetStrList() == nil {
				log.Debugf("  ⚠️  enum_list GetStrList() is nil, value type: %T", attr.GetValue().ValueType)
			} else {
				val := attr.GetValue().GetStrList().GetValueList()
				log.Debugf("  ✅ enum_list has %d values: %v", len(val), val)
				if len(val) > 0 {
					item.EnumList = &pb.StringArrayWrapper{
						Value: val,
					}
					log.Infof("  ✅ Mapped enum_list: %v", val)
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
	if itemEntity.Item.StringList != nil && len(itemEntity.Item.StringList.Value) > 0 {
		AddAttribute(&attributeDataArgList, stringListAttr, itemEntity.Item.StringList.Value)
	}
	if itemEntity.Item.Int64List != nil && len(itemEntity.Item.Int64List.Value) > 0 {
		AddAttribute(&attributeDataArgList, int64ListAttr, itemEntity.Item.Int64List.Value)
	}
	if itemEntity.Item.FloatList != nil && len(itemEntity.Item.FloatList.Value) > 0 {
		AddAttribute(&attributeDataArgList, floatListAttr, itemEntity.Item.FloatList.Value)
	}
	if itemEntity.Item.BoolList != nil && len(itemEntity.Item.BoolList.Value) > 0 {
		AddAttribute(&attributeDataArgList, boolListAttr, itemEntity.Item.BoolList.Value)
	}
	if itemEntity.Item.ByteList != nil && len(itemEntity.Item.ByteList.Value) > 0 {
		AddAttribute(&attributeDataArgList, byteListAttr, itemEntity.Item.ByteList.Value)
	}
	if itemEntity.Item.EnumList != nil && len(itemEntity.Item.EnumList.Value) > 0 {
		AddAttribute(&attributeDataArgList, enumListAttr, itemEntity.Item.EnumList.Value)
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
// Returns a map of item extId -> list of ItemStats protobuf objects
func (r *ItemRepositoryImpl) fetchItemStatsForItems(items []*pb.Item) (map[string][]*statsPb.ItemStats, error) {
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

	// Query item_stats entity from IDF
	// Filter by item_ext_id IN (extIds)
	idfClient := external.Interfaces().IdfClient()

	// Build query to get all itemStats for these items
	query, err := idfQr.QUERY("itemStatsListQuery").
		FROM("item_stats").
		Proto()
	if err != nil {
		return nil, fmt.Errorf("failed to build IDF query for itemStats: %w", err)
	}

	// Include all item_stats columns
	query.GroupBy = &insights_interface.QueryGroupBy{
		RawColumns: []*insights_interface.QueryRawColumn{
			{Column: proto.String("stats_ext_id")},
			{Column: proto.String("item_ext_id")},
			{Column: proto.String("age")},
			{Column: proto.String("heart_rate")},
			{Column: proto.String("food_intake")},
		},
	}

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
			case "age":
				if attr.GetValue() != nil {
					val := int32(attr.GetValue().GetInt64Value())
					stat.Age = &val
				}
			case "heart_rate":
				if attr.GetValue() != nil {
					val := int32(attr.GetValue().GetInt64Value())
					stat.HeartRate = &val
				}
			case "food_intake":
				if attr.GetValue() != nil {
					val := attr.GetValue().GetDoubleValue()
					stat.FoodIntake = &val
				}
			}
		}

		// Only include itemStats for items we're interested in
		if itemExtId != "" && extIdSet[itemExtId] {
			itemStatsMap[itemExtId] = append(itemStatsMap[itemExtId], stat)
			totalStatsCount++
			log.Debugf("  Added itemStats for item %s: statsExtId=%v, age=%v, heartRate=%v, foodIntake=%v",
				itemExtId, stat.StatsExtId, stat.Age, stat.HeartRate, stat.FoodIntake)
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
