/*
 * IDF Repository Implementation for Item Entity
 * Maps between protobuf Item model (camelCase) and IDF attributes (snake_case)
 */

package idf

import (
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
func (r *ItemRepositoryImpl) ListItems(queryParams *models.QueryParams) ([]*pb.Item, int64, error) {
	// Build IDF query
	query, err := idfQr.QUERY(itemEntityTypeName + "ListQuery").
		FROM(itemEntityTypeName).Proto()
	if err != nil {
		log.Errorf("Failed to build IDF query: %v", err)
		return nil, 0, err
	}

	// Add pagination
	page := queryParams.Page
	limit := queryParams.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := page * limit

	if query.GroupBy == nil {
		query.GroupBy = &insights_interface.QueryGroupBy{}
	}

	// CRITICAL: Specify which columns to fetch from IDF
	// Without this, IDF returns entities but without attribute data populated
	itemColumns := []string{itemIdAttr, itemNameAttr, itemTypeAttr, descriptionAttr, extIdAttr}
	var rawColumns []*insights_interface.QueryRawColumn
	for _, col := range itemColumns {
		rawColumns = append(rawColumns, &insights_interface.QueryRawColumn{
			Column: proto.String(col),
		})
	}
	query.GroupBy.RawColumns = rawColumns
	log.Debugf("IDF query columns: %v", itemColumns)

	query.GroupBy.RawLimit = &insights_interface.QueryLimit{
		Limit:  proto.Int64(int64(limit)),
		Offset: proto.Int64(int64(offset)),
	}

	queryArg := &insights_interface.GetEntitiesWithMetricsArg{
		Query: query,
	}

	// Query IDF
	idfClient := external.Interfaces().IdfClient()
	queryResponse, err := idfClient.GetEntitiesWithMetricsRet(queryArg)
	if err != nil {
		log.Errorf("Failed to query IDF: %v", err)
		return nil, 0, err
	}

	// Convert IDF entities to Item protobufs
	var items []*pb.Item
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

	totalCount := groupResults[0].GetTotalEntityCount()
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
