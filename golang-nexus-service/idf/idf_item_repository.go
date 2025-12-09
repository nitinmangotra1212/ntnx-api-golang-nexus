/*
 * IDF Repository Implementation for Item Entity
 * Maps between protobuf Item model (camelCase) and IDF attributes (snake_case)
 */

package idf

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nutanix-core/go-cache/insights/insights_interface"
	idfQr "github.com/nutanix-core/go-cache/insights/insights_interface/query"
	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config"
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
// Uses OData parser to handle $filter, $orderby, $select, $expand
// When $expand is present, uses GraphQL via statsGW (following categories pattern)
func (r *ItemRepositoryImpl) ListItems(queryParams *models.QueryParams) ([]*pb.Item, int64, error) {
	var items []*pb.Item
	var totalCount int64

	if queryParams.Expand != "" {
		// GraphQL path (with expand) - following categories pattern
		log.Infof("Using GraphQL path for expansion: %s", queryParams.Expand)

		// Try statsGW first, but fallback to regular IDF if it fails
		// NOTE: statsGW might not support nested expand options like $select and $orderby
		// So we'll always fallback to IDF path when nested options are present
		expandOptions := ParseExpandOptions(queryParams.Expand)
		hasNestedOptions := expandOptions != nil && (expandOptions.Select != nil || expandOptions.OrderBy != nil || expandOptions.Filter != nil)

		statsGWClient := external.Interfaces().StatsGWClient()
		if statsGWClient != nil && !hasNestedOptions {
			// Generate GraphQL query from OData
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

		// Now fetch associations for each item from IDF
		// Query item_associations entity where item_id matches item.extId
		log.Infof("🔍 Fetching associations for %d items", len(items))
		associationsMap, err := r.fetchAssociationsForItems(items)
		if err != nil {
			log.Warnf("Failed to fetch associations: %v, continuing without associations", err)
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

		totalCount = groupResults[0].GetTotalEntityCount()
		log.Infof("✅ Retrieved %d items from IDF (total: %d) with associations fetched separately", len(items), totalCount)
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
	log.Debugf("Mapping IDF entity with %d attributes", len(entity.GetAttributeDataMap()))
	for _, attr := range entity.GetAttributeDataMap() {
		log.Debugf("  IDF attribute: %s = %+v", attr.GetName(), attr.GetValue())
	}

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
	query.GroupBy = &insights_interface.QueryGroupBy{
		RawColumns: []*insights_interface.QueryRawColumn{
			{Column: proto.String("item_id")},
			{Column: proto.String("entity_type")},
			{Column: proto.String("entity_id")},
			{Column: proto.String("count")},
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
