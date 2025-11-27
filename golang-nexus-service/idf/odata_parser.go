/*
 * OData Query Parser for IDF Queries
 * Aligned with az-manager implementation
 */

package idf

import (
	"fmt"

	"github.com/nutanix-core/go-cache/insights/insights_interface"
	idfQr "github.com/nutanix-core/go-cache/insights/insights_interface/query"
	"github.com/nutanix-core/ntnx-api-odata-go/db/idf"
	"github.com/nutanix-core/ntnx-api-odata-go/odata/edm"
	"github.com/nutanix-core/ntnx-api-odata-go/odata/uri/parser"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/models"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

// GenerateListQuery generates an IDF query from OData query parameters
// This function follows the same pattern as az-manager's GenerateListQuery
func GenerateListQuery(queryParams *models.QueryParams, resourcePath string,
	entityName string, defaultSortColumn string) (*insights_interface.GetEntitiesWithMetricsArg, error) {
	// Get entity bindings for nexus module
	// For now, we'll create a minimal EDM provider with Item entity binding
	entityBindingList := GetNexusEntityBindings()
	
	log.Debugf("EDM bindings count: %d", len(entityBindingList))
	for i, binding := range entityBindingList {
		if binding.PropertyMappings != nil {
			log.Debugf("EDM bindings[%d] property mappings: %+v", i, binding.PropertyMappings)
		}
	}

	// Create EDM provider with entity bindings
	edmProvider := edm.NewCustomEdmProvider(entityBindingList)
	
	// Create OData parser
	odataParser := parser.NewParser(edmProvider)
	
	// Create query parameter object
	queryParam := parser.NewQueryParam()
	if queryParams.Filter != "" {
		queryParam.SetFilter(queryParams.Filter)
	}

	if queryParams.Orderby != "" {
		queryParam.SetOrderBy(queryParams.Orderby)
	}

	if queryParams.Select != "" {
		queryParam.SetSelect(queryParams.Select)
	}

	if queryParams.Expand != "" {
		queryParam.SetExpand(queryParams.Expand)
	}

	// Parse OData query parameters
	uriInfo, parseErr := odataParser.ParserWithQueryParam(queryParam, resourcePath)
	if parseErr != nil {
		log.Errorf("Failed to Parse OData expression: %v", parseErr)
		return nil, fmt.Errorf("invalid OData query: %w", parseErr)
	}

	// Use IDF query evaluator to convert parsed OData to IDF query
	// For now, we don't support GraphQL expansion, so use regular IDF evaluator
	log.Debugf("Using regular IDF query evaluator")
	idfQueryEval := idf.IDFQueryEvaluator{}
	idfQuery, evalErr := idfQueryEval.GetQuery(uriInfo, resourcePath)
	if evalErr != nil {
		log.Errorf("Failed to Evaluate OData expression: %v", evalErr)
		return nil, fmt.Errorf("failed to evaluate OData query: %w", evalErr)
	}

	// Construct final IDF query with pagination
	queryArg, err := constructIDFQuery(queryParams, idfQuery, entityName, defaultSortColumn)
	if err != nil {
		log.Errorf("Failed to construct IDF Query: %v", err)
		return nil, fmt.Errorf("failed to construct IDF query: %w", err)
	}

	return queryArg, nil
}

// constructIDFQuery constructs the final IDF query from parsed OData and query params
// This follows az-manager's constructIDFQuery pattern
func constructIDFQuery(queryParams *models.QueryParams, idfQuery *insights_interface.Query,
	entityType string, defaultSortColumn string) (*insights_interface.GetEntitiesWithMetricsArg, error) {
	
	// Build base query
	query, err := idfQr.QUERY(entityType + "ListQuery").
		FROM(entityType).Proto()
	if err != nil {
		log.Errorf("Failed to build IDF query: %v", err)
		return nil, fmt.Errorf("failed to build IDF query: %w", err)
	}

	log.Debugf("Query in constructIDFQuery: %+v", query.String())

	// Handle pagination
	page := queryParams.Page
	limit := queryParams.Limit

	if page < 0 {
		page = 0
	}

	// Default limit if not specified or invalid
	if limit <= 0 {
		limit = 50 // Default page size
	}
	if limit > 1000 {
		limit = 1000 // Max page size
	}

	if query.GroupBy == nil {
		query.GroupBy = &insights_interface.QueryGroupBy{}
	}

	// Use columns from IDF query evaluator (from OData $select)
	if idfQuery.GetGroupBy() != nil && len(idfQuery.GetGroupBy().RawColumns) > 0 {
		query.GroupBy.RawColumns = idfQuery.GetGroupBy().RawColumns
		log.Debugf("Using OData $select columns: %+v", query.GroupBy.RawColumns)
	} else {
		// Default: fetch all item columns
		itemColumns := []string{itemIdAttr, itemNameAttr, itemTypeAttr, descriptionAttr, extIdAttr}
		var rawColumns []*insights_interface.QueryRawColumn
		for _, col := range itemColumns {
			rawColumns = append(rawColumns, &insights_interface.QueryRawColumn{
				Column: proto.String(col),
			})
		}
		query.GroupBy.RawColumns = rawColumns
		log.Debugf("Using default columns: %v", itemColumns)
	}

	// Add sorting from OData $orderby
	if idfQuery.GetGroupBy() != nil && idfQuery.GetGroupBy().GetGroupSortOrder() != nil {
		query.GroupBy.RawSortOrder = idfQuery.GetGroupBy().GetGroupSortOrder()
		log.Debugf("Using OData $orderby: %+v", query.GroupBy.RawSortOrder)
	}

	// Add pagination
	offset := page * limit
	limit64 := int64(limit)
	offset64 := int64(offset)

	if query.GroupBy.RawLimit == nil {
		query.GroupBy.RawLimit = &insights_interface.QueryLimit{}
	}

	query.GroupBy.RawLimit.Limit = &limit64
	query.GroupBy.RawLimit.Offset = &offset64

	// Add filter from OData $filter
	query.WhereClause = idfQuery.GetWhereClause()
	if query.WhereClause != nil {
		log.Debugf("Using OData $filter: %+v", query.WhereClause)
	}

	log.Debugf("Final IDF Query: %+v", query.String())

	dbQueryArg := &insights_interface.GetEntitiesWithMetricsArg{
		Query: query,
	}

	return dbQueryArg, nil
}

// GetNexusEntityBindings returns EDM entity bindings for nexus module
// This creates a minimal EDM binding for the Item entity
// In a full implementation, these would be generated from YAML definitions
func GetNexusEntityBindings() []*edm.EdmEntityBinding {
	var entityBindingList []*edm.EdmEntityBinding
	
	// Create Item entity binding
	itemBinding := createItemEntityBinding()
	entityBindingList = append(entityBindingList, itemBinding)
	
	return entityBindingList
}

// createItemEntityBinding creates an EDM binding for the Item entity
// This maps OData field names (camelCase) to IDF attribute names (snake_case)
// Following the pattern from az-manager and guru generated EDM bindings
func createItemEntityBinding() *edm.EdmEntityBinding {
	binding := new(edm.EdmEntityBinding)

	// Set Property Mappings (OData field name → IDF column name)
	binding.PropertyMappings = make(map[string]string)
	binding.PropertyMappings["itemId"] = itemIdAttr      // "item_id"
	binding.PropertyMappings["itemName"] = itemNameAttr   // "item_name"
	binding.PropertyMappings["itemType"] = itemTypeAttr  // "item_type"
	binding.PropertyMappings["description"] = descriptionAttr // "description"
	binding.PropertyMappings["extId"] = extIdAttr        // "ext_id"

	// Filterable properties (can be used in $filter)
	filterProperties := make(map[string]bool)
	filterProperties["itemId"] = true
	filterProperties["itemName"] = true
	filterProperties["itemType"] = true
	filterProperties["extId"] = true

	// Sortable properties (can be used in $orderby)
	sortableProperties := make(map[string]bool)
	sortableProperties["itemId"] = true
	sortableProperties["itemName"] = true
	sortableProperties["itemType"] = true

	// Create properties for Item entity
	var properties []*edm.EdmProperty

	// itemId property
	itemIdProp := new(edm.EdmProperty)
	itemIdProp.Name = "itemId"
	itemIdProp.IsCollection = false
	itemIdProp.Type = string(edm.EdmInt64) // Use EdmInt64 (IDF stores as int64, protobuf uses int32)
	itemIdProp.MappedName = binding.PropertyMappings["itemId"]
	itemIdProp.IsFilterable = filterProperties["itemId"]
	itemIdProp.IsSortable = sortableProperties["itemId"]
	properties = append(properties, itemIdProp)

	// itemName property
	itemNameProp := new(edm.EdmProperty)
	itemNameProp.Name = "itemName"
	itemNameProp.IsCollection = false
	itemNameProp.Type = string(edm.EdmString)
	itemNameProp.MappedName = binding.PropertyMappings["itemName"]
	itemNameProp.IsFilterable = filterProperties["itemName"]
	itemNameProp.IsSortable = sortableProperties["itemName"]
	properties = append(properties, itemNameProp)

	// itemType property
	itemTypeProp := new(edm.EdmProperty)
	itemTypeProp.Name = "itemType"
	itemTypeProp.IsCollection = false
	itemTypeProp.Type = string(edm.EdmString)
	itemTypeProp.MappedName = binding.PropertyMappings["itemType"]
	itemTypeProp.IsFilterable = filterProperties["itemType"]
	itemTypeProp.IsSortable = sortableProperties["itemType"]
	properties = append(properties, itemTypeProp)

	// description property
	descProp := new(edm.EdmProperty)
	descProp.Name = "description"
	descProp.IsCollection = false
	descProp.Type = string(edm.EdmString)
	descProp.MappedName = binding.PropertyMappings["description"]
	descProp.IsFilterable = false // description is not filterable
	descProp.IsSortable = false   // description is not sortable
	properties = append(properties, descProp)

	// extId property
	extIdProp := new(edm.EdmProperty)
	extIdProp.Name = "extId"
	extIdProp.IsCollection = false
	extIdProp.Type = string(edm.EdmString)
	extIdProp.MappedName = binding.PropertyMappings["extId"]
	extIdProp.IsFilterable = filterProperties["extId"]
	extIdProp.IsSortable = false // extId is not sortable
	properties = append(properties, extIdProp)

	// Set Entity Type
	entityType := new(edm.EdmEntityType)
	entityType.Name = "item"
	entityType.Properties = properties
	binding.EntityType = entityType

	// Set Entity Set
	entitySet := new(edm.EdmEntitySet)
	entitySet.Name = "items"
	entitySet.EntityType = edm.GetFullQualifiedName(edm.NamespaceEntities, "item")
	entitySet.IncludeInServiceDocument = true
	entitySet.TableName = itemEntityTypeName // "item"
	binding.EntitySet = entitySet

	return binding
}

