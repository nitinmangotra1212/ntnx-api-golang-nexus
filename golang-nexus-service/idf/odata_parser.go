/*
 * OData Query Parser for IDF Queries
 * Aligned with az-manager implementation
 */

package idf

import (
	"fmt"
	"strings"

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

	if queryParams.Apply != "" {
		queryParam.SetApply(queryParams.Apply)
		log.Debugf("Set $apply parameter: %s", queryParams.Apply)
	}

	// Parse OData query parameters
	// Note: Module should be "config" or "stats" (not entityName which is "item")
	// The resourcePath is "/items" which becomes "items" after removing leading slash
	// GetResourcePathFromParseParam concatenates: Namespace + Module + Resource
	// Result: "nexus" + "config" + "items" = "nexusconfigitems" (for entity set lookup)
	module := "config" // Default to config module
	if strings.Contains(resourcePath, "stats") {
		module = "stats"
	}
	parseParam := parser.ParseParam{
		Namespace: "nexus",
		Module:    module, // "config" or "stats" (not entityName)
		Resource:  resourcePath,
	}
	uriInfo, parseErr := odataParser.ParserWithQueryParam(queryParam, parseParam)
	if parseErr != nil {
		log.Errorf("Failed to Parse OData expression: %v", parseErr)
		// Provide helpful error message for common syntax mistakes
		if strings.Contains(parseErr.Error(), "invalid groupby format") {
			return nil, fmt.Errorf("invalid OData query: %w. Hint: Use 'groupby((propertyName))' with double parentheses. For filtering, use '$filter=property eq value&$apply=groupby((property))'", parseErr)
		}
		// Return error with context for AppMessage formatting
		return nil, fmt.Errorf("invalid OData query: %w", parseErr)
	}

	// Use GraphQL query evaluator when expansion is requested, otherwise use regular IDF evaluator
	// This follows az-manager pattern
	var idfQuery *insights_interface.Query
	var evalErr error

	if queryParams.Expand != "" {
		log.Infof("Using GraphQL query evaluator for expansion: %s", queryParams.Expand)
		// Note: GraphQL query generation is handled separately in repository
		// For now, we still need IDF query for non-expanded fields
		// The actual GraphQL execution happens in repository
		idfQueryEval := idf.IDFQueryEvaluator{}
		idfQuery, evalErr = idfQueryEval.GetQuery(uriInfo, parseParam)
	} else {
		log.Debugf("Using regular IDF query evaluator")
		idfQueryEval := idf.IDFQueryEvaluator{}
		idfQuery, evalErr = idfQueryEval.GetQuery(uriInfo, parseParam)
	}

	if evalErr != nil {
		log.Errorf("Failed to Evaluate OData expression: %v", evalErr)
		// Return error with context for AppMessage formatting
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

	// CRITICAL: Copy GroupByColumn from idfQuery if present (for $apply=groupby)
	// This tells IDF which column to actually group by (e.g., "item_type")
	// GetGroupByColumn() returns a string (not a pointer), so check if it's not empty
	if idfQuery.GetGroupBy() != nil && idfQuery.GetGroupBy().GetGroupByColumn() != "" {
		groupByCol := idfQuery.GetGroupBy().GetGroupByColumn()
		query.GroupBy.GroupByColumn = &groupByCol
		log.Infof("✅ Setting GroupBy column: %s", groupByCol)
	}

	// CRITICAL: Copy AggregateColumns from idfQuery if present (for aggregations)
	// This tells IDF which aggregations to compute (e.g., sum, count, average)
	if idfQuery.GetGroupBy() != nil && len(idfQuery.GetGroupBy().GetAggregateColumns()) > 0 {
		query.GroupBy.AggregateColumns = idfQuery.GetGroupBy().GetAggregateColumns()
		log.Infof("✅ Setting Aggregate columns: %d aggregations", len(query.GroupBy.AggregateColumns))
	}

	// List columns that should always be included for item entities
	listColumns := []string{
		"int64_list",
	}

	// Time-series metrics that should always be included for item_stats entities
	itemStatsMetrics := []string{
		"age",         // Time-series metric (is_attribute: false)
		"heart_rate",  // Time-series metric (is_attribute: false)
		"food_intake", // Time-series metric (is_attribute: false)
	}

	// Use columns from IDF query evaluator (from OData $select)
	if idfQuery.GetGroupBy() != nil && len(idfQuery.GetGroupBy().RawColumns) > 0 {
		// Start with columns from OData parser
		rawColumns := idfQuery.GetGroupBy().RawColumns

		// Check which columns are already included
		existingColumns := make(map[string]bool)
		for _, col := range rawColumns {
			colName := col.GetColumn()
			if colName != "" {
				existingColumns[colName] = true
			}
		}

		// Add required columns based on entity type
		if entityType == "item_stats" {
			// For item_stats: ensure time-series metrics are included
			for _, metric := range itemStatsMetrics {
				if !existingColumns[metric] {
					rawColumns = append(rawColumns, &insights_interface.QueryRawColumn{
						Column: proto.String(metric),
					})
					log.Infof("📋 [IDF QUERY] Added missing item_stats time-series metric: %s", metric)
				}
			}
		} else {
			// For item: add list columns if they're not already present
			for _, listCol := range listColumns {
				if !existingColumns[listCol] {
					rawColumns = append(rawColumns, &insights_interface.QueryRawColumn{
						Column: proto.String(listCol),
					})
					log.Infof("📋 [IDF QUERY] Added missing list column: %s", listCol)
				}
			}
		}

		query.GroupBy.RawColumns = rawColumns
		log.Infof("📋 [IDF QUERY] Using OData $select columns + required columns: %d total columns", len(rawColumns))
		for i, col := range rawColumns {
			log.Infof("📋 [IDF QUERY] Column %d: %s", i, col.GetColumn())
		}
	} else {
		// Default: fetch all columns based on entity type
		var defaultColumns []string

		if entityType == "item_stats" {
			// For item_stats: include attributes and time-series metrics
			defaultColumns = []string{
				"stats_ext_id", // Attribute (is_attribute: true)
				"item_ext_id",  // Attribute (is_attribute: true)
				"age",          // Time-series metric (is_attribute: false)
				"heart_rate",   // Time-series metric (is_attribute: false)
				"food_intake",  // Time-series metric (is_attribute: false)
			}
			log.Infof("📋 [IDF QUERY] Using default item_stats columns (including time-series metrics)")
		} else {
			// For item: include all item columns (including new GroupBy and list fields)
			defaultColumns = []string{
				itemIdAttr, itemNameAttr, itemTypeAttr, descriptionAttr, extIdAttr,
				quantityAttr, priceAttr, isActiveAttr, priorityAttr, statusAttr,
			}
			// Add list columns
			defaultColumns = append(defaultColumns, listColumns...)
		}

		var rawColumns []*insights_interface.QueryRawColumn
		for _, col := range defaultColumns {
			rawColumns = append(rawColumns, &insights_interface.QueryRawColumn{
				Column: proto.String(col),
			})
		}
		query.GroupBy.RawColumns = rawColumns
		log.Infof("📋 [IDF QUERY] Using default columns: %v", defaultColumns)
		log.Infof("📋 [IDF QUERY] RawColumns count: %d", len(rawColumns))
		for i, col := range rawColumns {
			log.Infof("📋 [IDF QUERY] Column %d: %s", i, col.GetColumn())
		}
	}

	// Add sorting from OData $orderby
	if idfQuery.GetGroupBy() != nil && idfQuery.GetGroupBy().GetGroupSortOrder() != nil {
		query.GroupBy.RawSortOrder = idfQuery.GetGroupBy().GetGroupSortOrder()
		log.Debugf("Using OData $orderby: %+v", query.GroupBy.RawSortOrder)
	}

	// CRITICAL: Copy GroupLimit from idfQuery if present (for $apply group pagination)
	// IDFApplyEvaluator sets GroupLimit for group-level pagination
	if idfQuery.GetGroupBy() != nil && idfQuery.GetGroupBy().GetGroupLimit() != nil {
		query.GroupBy.GroupLimit = idfQuery.GetGroupBy().GetGroupLimit()
		log.Debugf("Using GroupLimit from $apply: limit=%d, offset=%d",
			*idfQuery.GetGroupBy().GetGroupLimit().Limit,
			*idfQuery.GetGroupBy().GetGroupLimit().Offset)
	} else {
		// Add pagination for regular queries (not GroupBy)
		offset := page * limit
		limit64 := int64(limit)
		offset64 := int64(offset)

		if query.GroupBy.RawLimit == nil {
			query.GroupBy.RawLimit = &insights_interface.QueryLimit{}
		}

		query.GroupBy.RawLimit.Limit = &limit64
		query.GroupBy.RawLimit.Offset = &offset64
	}

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
// This creates EDM bindings for Item and ItemAssociation entities
// In a full implementation, these would be generated from YAML definitions
func GetNexusEntityBindings() []*edm.EdmEntityBinding {
	var entityBindingList []*edm.EdmEntityBinding

	// Create Item entity binding
	itemBinding := createItemEntityBinding()
	entityBindingList = append(entityBindingList, itemBinding)

	// Create ItemAssociation entity binding (for $expand)
	itemAssocBinding := createItemAssociationEntityBinding()
	entityBindingList = append(entityBindingList, itemAssocBinding)

	// Create ItemStats entity binding (for stats module)
	itemStatsBinding := createItemStatsEntityBinding()
	entityBindingList = append(entityBindingList, itemStatsBinding)

	return entityBindingList
}

// createItemEntityBinding creates an EDM binding for the Item entity
// This maps OData field names (camelCase) to IDF attribute names (snake_case)
// Following the pattern from az-manager and guru generated EDM bindings
func createItemEntityBinding() *edm.EdmEntityBinding {
	binding := new(edm.EdmEntityBinding)

	// Set Property Mappings (OData field name → IDF column name)
	binding.PropertyMappings = make(map[string]string)
	binding.PropertyMappings["itemId"] = itemIdAttr           // "item_id"
	binding.PropertyMappings["itemName"] = itemNameAttr       // "item_name"
	binding.PropertyMappings["itemType"] = itemTypeAttr       // "item_type"
	binding.PropertyMappings["description"] = descriptionAttr // "description"
	binding.PropertyMappings["extId"] = extIdAttr             // "ext_id"
	binding.PropertyMappings["quantity"] = quantityAttr       // "quantity"
	binding.PropertyMappings["price"] = priceAttr             // "price"
	binding.PropertyMappings["isActive"] = isActiveAttr       // "is_active"
	binding.PropertyMappings["priority"] = priorityAttr       // "priority"
	binding.PropertyMappings["status"] = statusAttr           // "status"
	binding.PropertyMappings["int64List"] = int64ListAttr     // "int64_list"

	// Filterable properties (can be used in $filter)
	filterProperties := make(map[string]bool)
	filterProperties["itemId"] = true
	filterProperties["itemName"] = true
	filterProperties["itemType"] = true
	filterProperties["extId"] = true
	filterProperties["quantity"] = true
	filterProperties["price"] = true
	filterProperties["isActive"] = true
	filterProperties["status"] = true

	// Sortable properties (can be used in $orderby)
	sortableProperties := make(map[string]bool)
	sortableProperties["itemId"] = true
	sortableProperties["itemName"] = true
	sortableProperties["itemType"] = true
	sortableProperties["quantity"] = true
	sortableProperties["price"] = true
	sortableProperties["priority"] = true

	// Groupable properties (can be used in $apply=groupby) - ALL fields are groupable
	groupableProperties := make(map[string]bool)
	groupableProperties["itemId"] = true
	groupableProperties["itemName"] = true
	groupableProperties["itemType"] = true
	groupableProperties["description"] = true
	groupableProperties["extId"] = true
	groupableProperties["quantity"] = true
	groupableProperties["price"] = true
	groupableProperties["isActive"] = true
	groupableProperties["priority"] = true
	groupableProperties["status"] = true
	groupableProperties["int64List"] = true

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
	itemIdProp.IsGroupable = groupableProperties["itemId"]
	properties = append(properties, itemIdProp)

	// itemName property
	itemNameProp := new(edm.EdmProperty)
	itemNameProp.Name = "itemName"
	itemNameProp.IsCollection = false
	itemNameProp.Type = string(edm.EdmString)
	itemNameProp.MappedName = binding.PropertyMappings["itemName"]
	itemNameProp.IsFilterable = filterProperties["itemName"]
	itemNameProp.IsSortable = sortableProperties["itemName"]
	itemNameProp.IsGroupable = groupableProperties["itemName"]
	properties = append(properties, itemNameProp)

	// itemType property
	itemTypeProp := new(edm.EdmProperty)
	itemTypeProp.Name = "itemType"
	itemTypeProp.IsCollection = false
	itemTypeProp.Type = string(edm.EdmString)
	itemTypeProp.MappedName = binding.PropertyMappings["itemType"]
	itemTypeProp.IsFilterable = filterProperties["itemType"]
	itemTypeProp.IsSortable = sortableProperties["itemType"]
	itemTypeProp.IsGroupable = groupableProperties["itemType"]
	properties = append(properties, itemTypeProp)

	// description property
	descProp := new(edm.EdmProperty)
	descProp.Name = "description"
	descProp.IsCollection = false
	descProp.Type = string(edm.EdmString)
	descProp.MappedName = binding.PropertyMappings["description"]
	descProp.IsFilterable = false // description is not filterable
	descProp.IsSortable = false   // description is not sortable
	descProp.IsGroupable = groupableProperties["description"]
	properties = append(properties, descProp)

	// extId property
	extIdProp := new(edm.EdmProperty)
	extIdProp.Name = "extId"
	extIdProp.IsCollection = false
	extIdProp.Type = string(edm.EdmString)
	extIdProp.MappedName = binding.PropertyMappings["extId"]
	extIdProp.IsFilterable = filterProperties["extId"]
	extIdProp.IsSortable = false // extId is not sortable
	extIdProp.IsGroupable = groupableProperties["extId"]
	properties = append(properties, extIdProp)

	// quantity property
	quantityProp := new(edm.EdmProperty)
	quantityProp.Name = "quantity"
	quantityProp.IsCollection = false
	quantityProp.Type = string(edm.EdmInt64)
	quantityProp.MappedName = binding.PropertyMappings["quantity"]
	quantityProp.IsFilterable = filterProperties["quantity"]
	quantityProp.IsSortable = sortableProperties["quantity"]
	quantityProp.IsGroupable = groupableProperties["quantity"]
	properties = append(properties, quantityProp)

	// price property
	priceProp := new(edm.EdmProperty)
	priceProp.Name = "price"
	priceProp.IsCollection = false
	priceProp.Type = string(edm.EdmDouble)
	priceProp.MappedName = binding.PropertyMappings["price"]
	priceProp.IsFilterable = filterProperties["price"]
	priceProp.IsSortable = sortableProperties["price"]
	priceProp.IsGroupable = groupableProperties["price"]
	properties = append(properties, priceProp)

	// isActive property
	isActiveProp := new(edm.EdmProperty)
	isActiveProp.Name = "isActive"
	isActiveProp.IsCollection = false
	isActiveProp.Type = string(edm.EdmBoolean)
	isActiveProp.MappedName = binding.PropertyMappings["isActive"]
	isActiveProp.IsFilterable = filterProperties["isActive"]
	isActiveProp.IsSortable = false // boolean is typically not sortable
	isActiveProp.IsGroupable = groupableProperties["isActive"]
	properties = append(properties, isActiveProp)

	// priority property
	priorityProp := new(edm.EdmProperty)
	priorityProp.Name = "priority"
	priorityProp.IsCollection = false
	priorityProp.Type = string(edm.EdmInt32) // Changed from EdmByte to EdmInt32
	priorityProp.MappedName = binding.PropertyMappings["priority"]
	priorityProp.IsFilterable = false // priority is not filterable
	priorityProp.IsSortable = sortableProperties["priority"]
	priorityProp.IsGroupable = groupableProperties["priority"]
	properties = append(properties, priorityProp)

	// status property
	statusProp := new(edm.EdmProperty)
	statusProp.Name = "status"
	statusProp.IsCollection = false
	statusProp.Type = string(edm.EdmString)
	statusProp.MappedName = binding.PropertyMappings["status"]
	statusProp.IsFilterable = filterProperties["status"]
	statusProp.IsSortable = false // status is not sortable
	statusProp.IsGroupable = groupableProperties["status"]
	properties = append(properties, statusProp)

	// int64List property
	int64ListProp := new(edm.EdmProperty)
	int64ListProp.Name = "int64List"
	int64ListProp.IsCollection = true
	int64ListProp.Type = string(edm.EdmInt64)
	int64ListProp.MappedName = binding.PropertyMappings["int64List"]
	int64ListProp.IsFilterable = filterProperties["int64List"]
	int64ListProp.IsSortable = false // list is not sortable
	int64ListProp.IsGroupable = groupableProperties["int64List"]
	properties = append(properties, int64ListProp)

	// Set Entity Type
	entityType := new(edm.EdmEntityType)
	entityType.Name = "item"
	entityType.Properties = properties

	// Add navigation properties for $expand
	// associations is a navigation property that references ItemAssociation entity
	var navigationProperties []*edm.EdmNavigationProperty
	associationsNavProp := new(edm.EdmNavigationProperty)
	associationsNavProp.Name = "associations"
	associationsNavProp.IsCollection = true // It's an array/collection
	associationsNavProp.Type = edm.GetFullQualifiedName(edm.NamespaceEntities, "itemassociation")
	// Join keys use OData property names (camelCase), not IDF column names
	// Item.extId (UUID) = ItemAssociation.itemId (UUID)
	associationsNavProp.LeftEntityKey = "extId"   // OData property name in Item entity
	associationsNavProp.RightEntityKey = "itemId" // OData property name in ItemAssociation entity
	navigationProperties = append(navigationProperties, associationsNavProp)

	// Navigation property: itemStats (ItemStats - stats module)
	// Item.extId = ItemStats.itemExtId (one-to-many relationship)
	itemStatsNavProp := new(edm.EdmNavigationProperty)
	itemStatsNavProp.Name = "itemStats"
	itemStatsNavProp.IsCollection = true // It's an array/collection (one item can have multiple stats records)
	itemStatsNavProp.Type = edm.GetFullQualifiedName(edm.NamespaceEntities, "itemstats")
	itemStatsNavProp.LeftEntityKey = "extId"      // OData property name in Item entity
	itemStatsNavProp.RightEntityKey = "itemExtId" // OData property name in ItemStats entity
	navigationProperties = append(navigationProperties, itemStatsNavProp)

	entityType.NavigationProperties = navigationProperties

	binding.EntityType = entityType

	// Set Entity Set
	// Entity set name must match: Namespace + Module + Resource
	// For ParseParam{Namespace: "nexus", Module: "config", Resource: "items"}
	// The lookup name is: "nexus" + "config" + "items" = "nexusconfigitems"
	entitySet := new(edm.EdmEntitySet)
	entitySet.Name = "nexusconfigitems"
	entitySet.EntityType = edm.GetFullQualifiedName(edm.NamespaceEntities, "item")
	entitySet.IncludeInServiceDocument = true
	entitySet.TableName = itemEntityTypeName // "item"

	// Add navigation property bindings for $expand
	var navigationPropertyBindings []*edm.EdmNavigationPropertyBinding

	// Navigation property: associations (ItemAssociation - config module)
	associationsNavBinding := new(edm.EdmNavigationPropertyBinding)
	associationsNavBinding.Path = "associations"
	associationsNavBinding.Target = "nexusconfigitemassociations" // Target entity set name (must match entity set name)
	navigationPropertyBindings = append(navigationPropertyBindings, associationsNavBinding)

	// Navigation property: itemStats (ItemStats - stats module)
	itemStatsNavBinding := new(edm.EdmNavigationPropertyBinding)
	itemStatsNavBinding.Path = "itemStats"
	itemStatsNavBinding.Target = "nexusstatsitemstats" // Target entity set name (stats module)
	navigationPropertyBindings = append(navigationPropertyBindings, itemStatsNavBinding)

	entitySet.NavigationPropertyBindings = navigationPropertyBindings

	binding.EntitySet = entitySet

	return binding
}

// createItemAssociationEntityBinding creates an EDM binding for the ItemAssociation entity
// This is used for $expand=associations queries
func createItemAssociationEntityBinding() *edm.EdmEntityBinding {
	binding := new(edm.EdmEntityBinding)

	// Set Property Mappings (OData field name → IDF column name)
	binding.PropertyMappings = make(map[string]string)
	binding.PropertyMappings["itemId"] = "item_id"
	binding.PropertyMappings["entityType"] = "entity_type"
	binding.PropertyMappings["entityId"] = "entity_id"
	binding.PropertyMappings["count"] = "count"

	// Filterable properties
	filterProperties := make(map[string]bool)
	filterProperties["entityType"] = true
	filterProperties["count"] = true

	// Create properties for ItemAssociation entity
	var properties []*edm.EdmProperty

	// itemId property
	itemIdProp := new(edm.EdmProperty)
	itemIdProp.Name = "itemId"
	itemIdProp.IsCollection = false
	itemIdProp.Type = string(edm.EdmString)
	itemIdProp.MappedName = binding.PropertyMappings["itemId"]
	itemIdProp.IsFilterable = false
	itemIdProp.IsSortable = false
	properties = append(properties, itemIdProp)

	// entityType property
	entityTypeProp := new(edm.EdmProperty)
	entityTypeProp.Name = "entityType"
	entityTypeProp.IsCollection = false
	entityTypeProp.Type = string(edm.EdmString)
	entityTypeProp.MappedName = binding.PropertyMappings["entityType"]
	entityTypeProp.IsFilterable = filterProperties["entityType"]
	entityTypeProp.IsSortable = false
	properties = append(properties, entityTypeProp)

	// entityId property
	entityIdProp := new(edm.EdmProperty)
	entityIdProp.Name = "entityId"
	entityIdProp.IsCollection = false
	entityIdProp.Type = string(edm.EdmString)
	entityIdProp.MappedName = binding.PropertyMappings["entityId"]
	entityIdProp.IsFilterable = false
	entityIdProp.IsSortable = false
	properties = append(properties, entityIdProp)

	// count property
	countProp := new(edm.EdmProperty)
	countProp.Name = "count"
	countProp.IsCollection = false
	countProp.Type = string(edm.EdmInt64)
	countProp.MappedName = binding.PropertyMappings["count"]
	countProp.IsFilterable = filterProperties["count"]
	countProp.IsSortable = false
	properties = append(properties, countProp)

	// Set Entity Type
	entityType := new(edm.EdmEntityType)
	entityType.Name = "itemassociation" // Lowercase to match generated code
	entityType.Properties = properties
	binding.EntityType = entityType

	// Set Entity Set
	// Entity set name must match: Namespace + Module + Resource
	// For ParseParam{Namespace: "nexus", Module: "config", Resource: "item-associations"}
	// The lookup name is: "nexus" + "config" + "itemassociations" = "nexusconfigitemassociations"
	// Note: Resource path "/item-associations" becomes "itemassociations" (hyphens removed)
	entitySet := new(edm.EdmEntitySet)
	entitySet.Name = "nexusconfigitemassociations"
	entitySet.EntityType = edm.GetFullQualifiedName(edm.NamespaceEntities, "itemassociation")
	entitySet.IncludeInServiceDocument = true // Can be accessed via expand
	entitySet.TableName = "item_associations"
	binding.EntitySet = entitySet

	return binding
}

// createItemStatsEntityBinding creates an EDM binding for the ItemStats entity (stats module)
// This is used for /api/nexus/v4.1/stats/item-stats endpoint and $expand=itemStats from config module
func createItemStatsEntityBinding() *edm.EdmEntityBinding {
	binding := new(edm.EdmEntityBinding)

	// Set Property Mappings (OData field name → IDF column name)
	binding.PropertyMappings = make(map[string]string)
	binding.PropertyMappings["statsExtId"] = "stats_ext_id"
	binding.PropertyMappings["itemExtId"] = "item_ext_id"
	binding.PropertyMappings["age"] = "age"
	binding.PropertyMappings["heartRate"] = "heart_rate"
	binding.PropertyMappings["foodIntake"] = "food_intake"

	// Filterable properties
	filterProperties := make(map[string]bool)
	filterProperties["itemExtId"] = true
	filterProperties["age"] = true
	filterProperties["heartRate"] = true
	filterProperties["foodIntake"] = true

	// Sortable properties
	sortableProperties := make(map[string]bool)
	sortableProperties["itemExtId"] = true
	sortableProperties["age"] = true
	sortableProperties["heartRate"] = true
	sortableProperties["foodIntake"] = true

	// Groupable properties - ALL fields are groupable
	groupableProperties := make(map[string]bool)
	groupableProperties["statsExtId"] = true
	groupableProperties["itemExtId"] = true
	groupableProperties["age"] = true
	groupableProperties["heartRate"] = true
	groupableProperties["foodIntake"] = true

	// Create properties for ItemStats entity
	var properties []*edm.EdmProperty

	// statsExtId property (primary key)
	statsExtIdProp := new(edm.EdmProperty)
	statsExtIdProp.Name = "statsExtId"
	statsExtIdProp.IsCollection = false
	statsExtIdProp.Type = string(edm.EdmString)
	statsExtIdProp.MappedName = binding.PropertyMappings["statsExtId"]
	statsExtIdProp.IsFilterable = false
	statsExtIdProp.IsSortable = false
	statsExtIdProp.IsGroupable = groupableProperties["statsExtId"]
	properties = append(properties, statsExtIdProp)

	// itemExtId property (foreign key to Item.extId)
	itemExtIdProp := new(edm.EdmProperty)
	itemExtIdProp.Name = "itemExtId"
	itemExtIdProp.IsCollection = false
	itemExtIdProp.Type = string(edm.EdmString)
	itemExtIdProp.MappedName = binding.PropertyMappings["itemExtId"]
	itemExtIdProp.IsFilterable = filterProperties["itemExtId"]
	itemExtIdProp.IsSortable = sortableProperties["itemExtId"]
	itemExtIdProp.IsGroupable = groupableProperties["itemExtId"]
	properties = append(properties, itemExtIdProp)

	// age property
	ageProp := new(edm.EdmProperty)
	ageProp.Name = "age"
	ageProp.IsCollection = false
	ageProp.Type = string(edm.EdmInt32)
	ageProp.MappedName = binding.PropertyMappings["age"]
	ageProp.IsFilterable = filterProperties["age"]
	ageProp.IsSortable = sortableProperties["age"]
	ageProp.IsGroupable = groupableProperties["age"]
	properties = append(properties, ageProp)

	// heartRate property
	heartRateProp := new(edm.EdmProperty)
	heartRateProp.Name = "heartRate"
	heartRateProp.IsCollection = false
	heartRateProp.Type = string(edm.EdmInt32)
	heartRateProp.MappedName = binding.PropertyMappings["heartRate"]
	heartRateProp.IsFilterable = filterProperties["heartRate"]
	heartRateProp.IsSortable = sortableProperties["heartRate"]
	heartRateProp.IsGroupable = groupableProperties["heartRate"]
	properties = append(properties, heartRateProp)

	// foodIntake property
	foodIntakeProp := new(edm.EdmProperty)
	foodIntakeProp.Name = "foodIntake"
	foodIntakeProp.IsCollection = false
	foodIntakeProp.Type = string(edm.EdmDouble)
	foodIntakeProp.MappedName = binding.PropertyMappings["foodIntake"]
	foodIntakeProp.IsFilterable = filterProperties["foodIntake"]
	foodIntakeProp.IsSortable = sortableProperties["foodIntake"]
	foodIntakeProp.IsGroupable = groupableProperties["foodIntake"]
	properties = append(properties, foodIntakeProp)

	// Set Entity Type
	entityType := new(edm.EdmEntityType)
	entityType.Name = "itemstats" // Lowercase to match generated code
	entityType.Properties = properties
	binding.EntityType = entityType

	// Set Entity Set
	// Entity set name must match: Namespace + Module + Resource
	// For ParseParam{Namespace: "nexus", Module: "stats", Resource: "item-stats"}
	// The lookup name is: "nexus" + "stats" + "itemstats" = "nexusstatsitemstats"
	entitySet := new(edm.EdmEntitySet)
	entitySet.Name = "nexusstatsitemstats"
	entitySet.EntityType = edm.GetFullQualifiedName(edm.NamespaceEntities, "itemstats")
	entitySet.IncludeInServiceDocument = true
	entitySet.TableName = "item_stats" // IDF table name
	binding.EntitySet = entitySet

	return binding
}
