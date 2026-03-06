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

// extractGroupByColumn extracts the IDF column name from $apply=groupby((propName))
// Maps API property name to IDF column name using the item property mapping.
func extractGroupByColumn(applyParam string) string {
	// Parse "groupby((propName))" or "groupby((propName),aggregate(...))"
	// The column list is always (col1,col2,...) inside the outer groupby().
	// Find first ")" after "((" to get the end of the column list.
	start := strings.Index(applyParam, "((")
	if start < 0 {
		return ""
	}
	end := strings.Index(applyParam[start+2:], ")")
	if end < 0 {
		return ""
	}
	apiPropName := applyParam[start+2 : start+2+end]

	propToCol := map[string]string{
		"itemId": itemIdAttr, "itemName": itemNameAttr, "itemType": itemTypeAttr,
		"description": descriptionAttr, "extId": extIdAttr, "quantity": quantityAttr,
		"price": priceAttr, "isActive": isActiveAttr, "priority": priorityAttr,
		"status": statusAttr, "int64List": int64ListAttr,
	}
	if col, ok := propToCol[apiPropName]; ok {
		return col
	}
	return apiPropName
}

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

	// Limit defaults and validation are handled by the dev-platform layer.
	// Pass through whatever value is received.

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

	// API property → IDF column mappings (used for $select)
	itemPropToCol := map[string]string{
		"itemId": itemIdAttr, "itemName": itemNameAttr, "itemType": itemTypeAttr,
		"description": descriptionAttr, "extId": extIdAttr, "quantity": quantityAttr,
		"price": priceAttr, "isActive": isActiveAttr, "priority": priorityAttr,
		"status": statusAttr, "int64List": int64ListAttr,
	}
	itemStatsPropToCol := map[string]string{
		"age": "age", "heartRate": "heart_rate", "foodIntake": "food_intake",
	}

	var selectedColumns []string

	if queryParams.Select != "" {
		// Parse $select and map API property names to IDF columns.
		// The OData library's RawColumns mapping is incomplete, so we handle it ourselves
		// (matching the categories data-sync-service pattern).
		propMap := itemPropToCol
		if entityType == "item_stats" {
			propMap = itemStatsPropToCol
		}
		for _, prop := range strings.Split(queryParams.Select, ",") {
			prop = strings.TrimSpace(prop)
			if idfCol, ok := propMap[prop]; ok {
				selectedColumns = append(selectedColumns, idfCol)
			} else {
				log.Warnf("📋 [IDF QUERY] $select property %q not found in mapping, skipping", prop)
			}
		}

		log.Infof("📋 [IDF QUERY] Using $select columns: %v", selectedColumns)
	} else {
		// No $select: fetch all columns
		if entityType == "item_stats" {
			selectedColumns = []string{
				"item_ext_id", "age", "heart_rate", "food_intake", "timestamp", "speed",
			}
		} else {
			selectedColumns = []string{
				itemIdAttr, itemNameAttr, itemTypeAttr, descriptionAttr, extIdAttr,
				quantityAttr, priceAttr, isActiveAttr, priorityAttr, statusAttr,
				int64ListAttr,
			}
		}
		log.Infof("📋 [IDF QUERY] Using all columns (no $select): %v", selectedColumns)
	}

	var rawColumns []*insights_interface.QueryRawColumn
	for _, col := range selectedColumns {
		rawColumns = append(rawColumns, &insights_interface.QueryRawColumn{
			Column: proto.String(col),
		})
	}
	query.GroupBy.RawColumns = rawColumns

	// Add sorting from OData $orderby (matches categories: listQuery.GroupBy.RawSortOrderList = orderBy)
	if idfQuery.GetGroupBy() != nil && len(idfQuery.GetGroupBy().GetRawSortOrderList()) > 0 {
		query.GroupBy.RawSortOrderList = idfQuery.GetGroupBy().GetRawSortOrderList()
		log.Infof("📋 [IDF QUERY] Using OData $orderby: %d sort orders", len(query.GroupBy.RawSortOrderList))
		for i, so := range query.GroupBy.RawSortOrderList {
			log.Infof("📋 [IDF QUERY] Sort[%d]: column=%s, order=%s", i, so.GetSortColumn(), so.GetSortOrder().String())
		}
	}

	// Copy GroupSortOrder from idfQuery if present (for /orderby within $apply).
	// GroupSortOrder controls the sort order of groups themselves (distinct from
	// RawSortOrderList which sorts entities within groups from top-level $orderby).
	if idfQuery.GetGroupBy() != nil && idfQuery.GetGroupBy().GetGroupSortOrder() != nil {
		query.GroupBy.GroupSortOrder = idfQuery.GetGroupBy().GetGroupSortOrder()
		log.Infof("📋 [IDF QUERY] Using $apply orderby (GroupSortOrder): column=%s, order=%s",
			query.GroupBy.GetGroupSortOrder().GetSortColumn(),
			query.GroupBy.GetGroupSortOrder().GetSortOrder().String())
	}

	// Copy GroupLimit from idfQuery if present (for $apply group pagination)
	// GroupLimit controls how many groups IDF returns (group-level pagination).
	if idfQuery.GetGroupBy() != nil && idfQuery.GetGroupBy().GetGroupLimit() != nil {
		query.GroupBy.GroupLimit = idfQuery.GetGroupBy().GetGroupLimit()
		log.Debugf("Using GroupLimit from $apply: limit=%d, offset=%d",
			*idfQuery.GetGroupBy().GetGroupLimit().Limit,
			*idfQuery.GetGroupBy().GetGroupLimit().Offset)
	}

	// ALWAYS set RawLimit — it controls per-group entity count.
	// Even for $apply=groupby queries, $page/$limit apply to entities within each group,
	// matching the categories service pattern (addLimitAndPageParam is called unconditionally).
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

	// itemType property (enum stored as int64 in IDF)
	itemTypeProp := new(edm.EdmProperty)
	itemTypeProp.Name = "itemType"
	itemTypeProp.IsCollection = false
	itemTypeProp.Type = string(edm.EdmInt64)
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
	associationsNavProp.IsCollection = false
	associationsNavProp.MappingType = edm.EdmEnumMember{Value: "ONE_TO_MANY"}
	associationsNavProp.Type = edm.GetFullQualifiedName(edm.NamespaceEntities, "itemassociation")
	associationsNavProp.LeftEntityKey = "ext_id"
	associationsNavProp.RightEntityKey = "item_id"
	navigationProperties = append(navigationProperties, associationsNavProp)

	itemStatsNavProp := new(edm.EdmNavigationProperty)
	itemStatsNavProp.Name = "itemStats"
	itemStatsNavProp.IsCollection = false
	itemStatsNavProp.Type = edm.GetFullQualifiedName(edm.NamespaceEntities, "itemstats")
	itemStatsNavProp.LeftEntityKey = "ext_id"
	itemStatsNavProp.RightEntityKey = "item_ext_id"
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

	// Set Property Mappings — only TSDB metrics (following volumes pattern)
	// itemExtId is NOT included here; it's populated from the parent item's extId via JOIN context
	binding.PropertyMappings = make(map[string]string)
	binding.PropertyMappings["age"] = "age"
	binding.PropertyMappings["heartRate"] = "heart_rate"
	binding.PropertyMappings["foodIntake"] = "food_intake"

	// Filterable properties (only TSDB metrics)
	filterProperties := make(map[string]bool)
	filterProperties["age"] = true
	filterProperties["heartRate"] = true
	filterProperties["foodIntake"] = true

	// Sortable properties (only TSDB metrics)
	sortableProperties := make(map[string]bool)
	sortableProperties["age"] = true
	sortableProperties["heartRate"] = true
	sortableProperties["foodIntake"] = true

	// Groupable properties (only TSDB metrics)
	groupableProperties := make(map[string]bool)
	groupableProperties["age"] = true
	groupableProperties["heartRate"] = true
	groupableProperties["foodIntake"] = true

	// Create properties for ItemStats entity — only TSDB metrics
	// Following volumes pattern: stats entity only contains metrics, not identifiers
	var properties []*edm.EdmProperty

	ageProp := new(edm.EdmProperty)
	ageProp.Name = "age"
	ageProp.IsCollection = true
	ageProp.Type = string(edm.EdmInt32)
	ageProp.MappedName = binding.PropertyMappings["age"]
	ageProp.IsFilterable = filterProperties["age"]
	ageProp.IsSortable = sortableProperties["age"]
	ageProp.IsGroupable = groupableProperties["age"]
	properties = append(properties, ageProp)

	heartRateProp := new(edm.EdmProperty)
	heartRateProp.Name = "heartRate"
	heartRateProp.IsCollection = true
	heartRateProp.Type = string(edm.EdmInt32)
	heartRateProp.MappedName = binding.PropertyMappings["heartRate"]
	heartRateProp.IsFilterable = filterProperties["heartRate"]
	heartRateProp.IsSortable = sortableProperties["heartRate"]
	heartRateProp.IsGroupable = groupableProperties["heartRate"]
	properties = append(properties, heartRateProp)

	foodIntakeProp := new(edm.EdmProperty)
	foodIntakeProp.Name = "foodIntake"
	foodIntakeProp.IsCollection = true
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
