/*
 * GraphQL Response Mapper
 * Maps GraphQL response (JSON) to protobuf Item objects
 * Based on categories implementation pattern
 */

package idf

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nutanix-core/ntnx-api-odata-go/db/idfgraphql"
	"github.com/nutanix-core/ntnx-api-odata-go/odata/edm"
	"github.com/nutanix-core/ntnx-api-odata-go/odata/uri/parser"
	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config"
	statsPb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/stats"
	"github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/response"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/models"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ItemGraphqlRetDto represents the GraphQL response structure
type ItemGraphqlRetDto struct {
	Items         []ItemGraphqlDto `json:"item"`
	TotalCount    int              `json:"total_entity_count"`
	FilteredCount int              `json:"filtered_entity_count"`
}

// ItemGraphqlDto represents a single item in GraphQL response.
// All fields are string arrays because StatsGW returns all values as strings in JSON.
type ItemGraphqlDto struct {
	ItemId      []string `json:"item_id"`
	ItemName    []string `json:"item_name"`
	ItemType    []string `json:"item_type"`
	Description []string `json:"description"`
	ExtId       []string `json:"ext_id"`
	Quantity    []string `json:"quantity"`
	Price       []string `json:"price"`
	IsActive    []string `json:"is_active"`
	Priority    []string `json:"priority"`
	Status      []string `json:"status"`
	// Expanded entities
	Associations *AssociationGraphQLDto     `json:"item_associations,omitempty"`
	ItemStats    *ItemStatsExpandGraphQLDto `json:"item_stats,omitempty"`
}

// AssociationGraphQLDto represents expanded associations in GraphQL response
type AssociationGraphQLDto struct {
	ItemId     []string `json:"item_id"`
	EntityType []string `json:"entity_type"`
	EntityId   []string `json:"entity_id"`
	Count      []string `json:"count"`
}

// StatsGWTimeValuePair represents a time-value pair from StatsGW.
// StatsGW returns {"time": <microseconds>, "values": [<val>]} for ALL fields
// (including identity fields) when timeseries:true is used.
type StatsGWTimeValuePair struct {
	Time   int64         `json:"time"`
	Values []interface{} `json:"values"`
}

// ItemStatsExpandGraphQLDto represents expanded itemStats in a JOINed GraphQL response.
// Uses json.RawMessage because StatsGW returns "values" as a bare number (not array)
// for timeseries data in both JOIN and primary-entity contexts.
type ItemStatsExpandGraphQLDto struct {
	Age        json.RawMessage `json:"age"`
	HeartRate  json.RawMessage `json:"heart_rate"`
	FoodIntake json.RawMessage `json:"food_intake"`
}

// ItemGroupGraphqlReturnDto represents grouped GraphQL response (like CategoryGroupGraphqlReturnDto)
type ItemGroupGraphqlReturnDto struct {
	ItemGroups      []ItemGroupGraphqlDto `json:"groupBy"`
	TotalGroupCount int64                 `json:"total_group_count"`
}

// ItemGroupGraphqlDto represents a single group in grouped GraphQL response
type ItemGroupGraphqlDto struct {
	Items                    []ItemGraphqlDto              `json:"item"`
	GroupByColumnValue       interface{}                   `json:"group_by_column_value"`
	GroupEntityCount         int                           `json:"group_entity_count"`
	AggregateColumnSummaries []AggregateDataGraphqlDto     `json:"aggregate_column_summaries"`
}

// AggregateDataGraphqlDto represents aggregate data for a group
type AggregateDataGraphqlDto struct {
	Data     *AggregateSummaryGraphqlDto `json:"data"`
	Operator string                      `json:"operator"`
}

// AggregateSummaryGraphqlDto represents aggregate summary information
type AggregateSummaryGraphqlDto struct {
	Name   string              `json:"name"`
	Values []AggregateValueDTO `json:"values"`
}

// AggregateValueDTO represents a single aggregate value
type AggregateValueDTO struct {
	Time   int64 `json:"time"`
	Values int64 `json:"values"`
}

// FlippedStatsGraphqlRetDto represents the response when the query was flipped
// (stats entity is primary, config item is nested expand).
type FlippedStatsGraphqlRetDto struct {
	ItemStats     []FlippedStatsGraphqlDto `json:"item_stats"`
	TotalCount    int                      `json:"total_entity_count"`
	FilteredCount int                      `json:"filtered_entity_count"`
}

// FlippedStatsGraphqlDto represents a single stats row in a flipped response.
// Uses json.RawMessage for metric fields because StatsGW returns "values" as a
// bare number (not array) when the stats entity is primary (flipped query).
type FlippedStatsGraphqlDto struct {
	Age        json.RawMessage `json:"age"`
	HeartRate  json.RawMessage `json:"heart_rate"`
	FoodIntake json.RawMessage `json:"food_intake"`
	EntityId   []string        `json:"_entity_id_"`
	Item       *ItemGraphqlDto `json:"item,omitempty"`
}

// parseTimeValuePairs parses a metric field from a StatsGW response.
// Handles both {"time":T,"values":V} (bare number) and {"time":T,"values":[V]} (array).
func parseTimeValuePairs(raw json.RawMessage) []StatsGWTimeValuePair {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	type rawTV struct {
		Time   int64           `json:"time"`
		Values json.RawMessage `json:"values"`
	}
	var rawPairs []rawTV
	if err := json.Unmarshal(raw, &rawPairs); err != nil {
		log.Warnf("Failed to parse time-value pairs: %v", err)
		return nil
	}

	result := make([]StatsGWTimeValuePair, 0, len(rawPairs))
	for _, rp := range rawPairs {
		tv := StatsGWTimeValuePair{Time: rp.Time}
		if len(rp.Values) > 0 && rp.Values[0] == '[' {
			_ = json.Unmarshal(rp.Values, &tv.Values)
		} else if len(rp.Values) > 0 && string(rp.Values) != "null" {
			var single interface{}
			if err := json.Unmarshal(rp.Values, &single); err == nil {
				tv.Values = []interface{}{single}
			}
		}
		result = append(result, tv)
	}
	return result
}

// ParseFlippedGraphqlResponse parses a flipped GraphQL response (item_stats as root).
func ParseFlippedGraphqlResponse(graphqlData string) (*FlippedStatsGraphqlRetDto, error) {
	if graphqlData == "" {
		return nil, fmt.Errorf("empty GraphQL response data")
	}
	var ret FlippedStatsGraphqlRetDto
	if err := json.Unmarshal([]byte(graphqlData), &ret); err != nil {
		log.Errorf("Failed to parse flipped GraphQL JSON response: %v", err)
		return nil, fmt.Errorf("failed to parse flipped GraphQL response: %w", err)
	}
	log.Debugf("Parsed flipped GraphQL response: %d stats rows, total: %d", len(ret.ItemStats), ret.TotalCount)
	return &ret, nil
}

// MapFlippedGraphqlToItems converts a flipped response (stats-primary) back to item-based structure.
func MapFlippedGraphqlToItems(flippedRet *FlippedStatsGraphqlRetDto, expansionKey string) ([]*pb.Item, error) {
	if flippedRet == nil {
		return nil, fmt.Errorf("nil flipped GraphQL response")
	}

	items := make([]*pb.Item, 0, len(flippedRet.ItemStats))
	for _, statsDto := range flippedRet.ItemStats {
		var item *pb.Item
		if statsDto.Item != nil {
			item = mapGraphqlItemFields(statsDto.Item, expansionKey)
		} else {
			item = &pb.Item{}
		}

		statsExpandDto := &ItemStatsExpandGraphQLDto{
			Age:        statsDto.Age,
			HeartRate:  statsDto.HeartRate,
			FoodIntake: statsDto.FoodIntake,
		}
		parentExtId := ""
		if item.ExtId != nil {
			parentExtId = *item.ExtId
		}
		item.ItemStats = mapGraphqlItemStats(statsExpandDto, parentExtId)
		items = append(items, item)
	}

	log.Infof("Mapped %d items from flipped GraphQL response", len(items))
	return items, nil
}

// ParseGroupedGraphqlResponse parses grouped GraphQL JSON response
func ParseGroupedGraphqlResponse(graphqlData string) (*ItemGroupGraphqlReturnDto, error) {
	if graphqlData == "" {
		return nil, fmt.Errorf("empty GraphQL response data")
	}

	var ret ItemGroupGraphqlReturnDto
	err := json.Unmarshal([]byte(graphqlData), &ret)
	if err != nil {
		log.Errorf("Failed to parse grouped GraphQL JSON response: %v", err)
		return nil, fmt.Errorf("failed to parse grouped GraphQL response: %w", err)
	}

	log.Infof("Parsed grouped GraphQL response: %d groups, totalGroupCount: %d",
		len(ret.ItemGroups), ret.TotalGroupCount)
	return &ret, nil
}

// ParseGraphqlResponse parses GraphQL JSON response to DTO
func ParseGraphqlResponse(graphqlData string) (*ItemGraphqlRetDto, error) {
	if graphqlData == "" {
		return nil, fmt.Errorf("empty GraphQL response data")
	}

	var ret ItemGraphqlRetDto
	err := json.Unmarshal([]byte(graphqlData), &ret)
	if err != nil {
		log.Errorf("Failed to parse GraphQL JSON response: %v", err)
		return nil, fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	log.Debugf("Parsed GraphQL response: %d items, total: %d", len(ret.Items), ret.TotalCount)
	return &ret, nil
}

// mapGraphqlItemFields maps common GraphQL DTO fields to a protobuf Item
func mapGraphqlItemFields(itemDto *ItemGraphqlDto, expansionKey string) *pb.Item {
	item := &pb.Item{}

	if len(itemDto.ItemId) > 0 {
		if id, err := parseInt32(itemDto.ItemId[0]); err == nil {
			item.ItemId = &id
		}
	}
	if len(itemDto.ItemName) > 0 {
		item.ItemName = &itemDto.ItemName[0]
	}
	if len(itemDto.ItemType) > 0 {
		protoVal := resolveItemTypeProtoFromStr(itemDto.ItemType[0])
		item.ItemType = &protoVal
	}
	if len(itemDto.Description) > 0 {
		item.Description = &itemDto.Description[0]
	}
	if len(itemDto.ExtId) > 0 {
		item.ExtId = &itemDto.ExtId[0]
	}
	if len(itemDto.Quantity) > 0 {
		if q, err := parseInt64(itemDto.Quantity[0]); err == nil {
			item.Quantity = &q
		}
	}
	if len(itemDto.Price) > 0 {
		if p, err := parseFloat64(itemDto.Price[0]); err == nil {
			item.Price = &p
		}
	}
	if len(itemDto.IsActive) > 0 {
		b := itemDto.IsActive[0] == "true" || itemDto.IsActive[0] == "1"
		item.IsActive = &b
	}
	if len(itemDto.Priority) > 0 {
		if p, err := parseInt32(itemDto.Priority[0]); err == nil {
			item.Priority = &p
		}
	}
	if len(itemDto.Status) > 0 {
		item.Status = &itemDto.Status[0]
	}

	// Map expanded associations (nested options handled server-side by StatsGW)
	if expansionKey != "" && itemDto.Associations != nil {
		associations := mapGraphqlAssociations(itemDto.Associations)
		item.Associations = &pb.ItemAssociationArrayWrapper{Value: associations}
	}

	// Map expanded itemStats (nested options handled server-side by StatsGW)
	// itemExtId is populated from the parent item's extId (following volumes pattern)
	if expansionKey != "" && itemDto.ItemStats != nil {
		parentExtId := ""
		if item.ExtId != nil {
			parentExtId = *item.ExtId
		}
		itemStats := mapGraphqlItemStats(itemDto.ItemStats, parentExtId)
		if itemStats != nil {
			item.ItemStats = itemStats
		}
	}

	return item
}

// MapGraphqlToItems maps GraphQL DTOs to protobuf Item objects
func MapGraphqlToItems(graphqlRet *ItemGraphqlRetDto, expansionKey string) ([]*pb.Item, error) {
	if graphqlRet == nil {
		return nil, fmt.Errorf("nil GraphQL response")
	}

	items := make([]*pb.Item, 0, len(graphqlRet.Items))

	for _, itemDto := range graphqlRet.Items {
		item := mapGraphqlItemFields(&itemDto, expansionKey)

		items = append(items, item)
	}

	log.Infof("Mapped %d items from GraphQL response", len(items))
	return items, nil
}

// mapGraphqlAssociations maps GraphQL association DTOs to protobuf ItemAssociation objects
// GraphQL returns arrays for each field, we need to combine them into objects
// Following categories pattern: arrays are parallel (same index = same entity)
func mapGraphqlAssociations(ascGraphql *AssociationGraphQLDto) []*pb.ItemAssociation {
	if ascGraphql == nil {
		return []*pb.ItemAssociation{}
	}

	// GraphQL returns parallel arrays - same index = same entity
	// Find the maximum length to iterate
	maxLen := len(ascGraphql.ItemId)
	if len(ascGraphql.EntityType) > maxLen {
		maxLen = len(ascGraphql.EntityType)
	}
	if len(ascGraphql.EntityId) > maxLen {
		maxLen = len(ascGraphql.EntityId)
	}
	if len(ascGraphql.Count) > maxLen {
		maxLen = len(ascGraphql.Count)
	}

	associations := make([]*pb.ItemAssociation, 0, maxLen)
	for i := 0; i < maxLen; i++ {
		assoc := &pb.ItemAssociation{}

		if i < len(ascGraphql.ItemId) && ascGraphql.ItemId[i] != "" {
			assoc.ItemId = &ascGraphql.ItemId[i]
		}
		if i < len(ascGraphql.EntityType) && ascGraphql.EntityType[i] != "" {
			assoc.EntityType = &ascGraphql.EntityType[i]
		}
		if i < len(ascGraphql.EntityId) && ascGraphql.EntityId[i] != "" {
			assoc.EntityId = &ascGraphql.EntityId[i]
		}
		if i < len(ascGraphql.Count) && ascGraphql.Count[i] != "" {
			if c, err := parseInt32(ascGraphql.Count[i]); err == nil {
				assoc.Count = &c
			}
		}

		associations = append(associations, assoc)
	}

	return associations
}

// GenerateGraphQLQuery generates a GraphQL query from OData query parameters.
// This uses the IdfGraphqlQueryEvaluator from ntnx-api-odata-go, following the
// categories pattern (getFinalGraphQLQuery in list_category.go).
// The evaluator generates a single GraphQL query with JOINs for expanded entities,
// including nested $select, $filter, $orderby handled server-side by StatsGW.
// Returns the query string and whether the query was flipped (stats entity became primary).
func GenerateGraphQLQuery(queryParams *models.QueryParams, resourcePath string) (string, bool, error) {
	entityBindingList := GetNexusEntityBindings()
	edmProvider := edm.NewCustomEdmProvider(entityBindingList)
	odataParser := parser.NewParser(edmProvider)

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
	}

	parseParam := parser.ParseParam{
		Namespace: "nexus",
		Module:    "config",
		Resource:  resourcePath,
	}
	uriInfo, parseErr := odataParser.ParserWithQueryParam(queryParam, parseParam)
	if parseErr != nil {
		log.Errorf("❌ Failed to Parse OData expression for GraphQL: %v", parseErr)
		return "", false, fmt.Errorf("invalid OData query: %w", parseErr)
	}

	// When $orderby is inside an expand with stats params ($startTime, etc.),
	// the OData parser flips the query: stats entity becomes primary, config becomes expanded.
	// The caller must update parseParam to match the flipped primary entity.
	isFlipped := uriInfo.GetMetadata().IsQueryFlipped()
	if isFlipped {
		log.Infof("🔄 [QueryFlip] Query was flipped: stats entity is now primary, config entity is expanded")
		parseParam.Resource = "itemstats"
		parseParam.Module = "stats"
	}

	idfGraphqlQueryEval := idfgraphql.IdfGraphqlQueryEvaluator{}
	graphqlQuery, err := idfGraphqlQueryEval.GetQuery(uriInfo, parseParam)
	if err != nil {
		log.Errorf("Failed to Evaluate GraphQL expression: %v", err)
		return "", false, fmt.Errorf("failed to evaluate GraphQL query: %w", err)
	}

	rootEntity := "item"
	if isFlipped {
		rootEntity = "item_stats"
	}

	if queryParams.Limit > 0 {
		limit := int(queryParams.Limit)
		page := int(queryParams.Page)
		if page < 0 {
			page = 0
		}
		graphqlQuery = injectPaginationIntoRootArgs(graphqlQuery, rootEntity, limit, page)
		log.Infof("📋 [GraphQL] Query with pagination (page_size=%d, page_offset=%d): %s", limit, page*limit, graphqlQuery)
	} else {
		log.Infof("📋 [GraphQL] Generated query: %s", graphqlQuery)
	}

	// For expand queries (no $apply), append total count fields so StatsGW returns them.
	if queryParams.Apply == "" && strings.HasSuffix(graphqlQuery, "}}") {
		graphqlQuery = graphqlQuery[:len(graphqlQuery)-1] + "filtered_entity_count,total_entity_count}"
	}

	return graphqlQuery, isFlipped, nil
}

// injectPaginationIntoRootArgs adds page_size and page_offset to the root entity's args
// in the GraphQL query. The ntnx-api-odata-go query evaluator does not add these for the
// root entity; StatsGW requires them to apply pagination.
func injectPaginationIntoRootArgs(query, rootEntity string, limit, page int) string {
	if limit <= 0 {
		return query // no default; only called when limit > 0
	}
	if page < 0 {
		page = 0
	}
	pageSize := limit
	pageOffset := page * limit
	paginationArgs := fmt.Sprintf("page_size:%d, page_offset:%d", pageSize, pageOffset)

	// Root entity with no args: "item{" -> "item(args: {page_size:N, page_offset:M}){"
	noArgsPattern := rootEntity + "{"
	if strings.Contains(query, noArgsPattern) {
		return strings.Replace(query, noArgsPattern, rootEntity+"(args: {"+paginationArgs+"}){", 1)
	}

	// Root entity with args: "item(args: {" -> "item(args: {page_size:N, page_offset:M, "
	withArgsPattern := rootEntity + "(args: {"
	if strings.Contains(query, withArgsPattern) {
		return strings.Replace(query, withArgsPattern, rootEntity+"(args: {"+paginationArgs+", ", 1)
	}

	return query
}

// tvFirstValue extracts the first value from a StatsGWTimeValuePair's Values array.
// Returns nil if Values is empty.
func tvFirstValue(tv StatsGWTimeValuePair) interface{} {
	if len(tv.Values) == 0 {
		return nil
	}
	return tv.Values[0]
}



// tvFloat64 extracts a float64 value from a StatsGWTimeValuePair
func tvFloat64(tv StatsGWTimeValuePair) (float64, bool) {
	v := tvFirstValue(tv)
	if v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// mapGraphqlItemStats maps GraphQL expanded itemStats DTO to protobuf ItemStats.
// Only TSDB metrics come from StatsGW. itemExtId is populated from the parent item's extId
// (following volumes pattern where volumeGroupExtId is set from parent, not from stats query).
func mapGraphqlItemStats(dto *ItemStatsExpandGraphQLDto, parentExtId string) *statsPb.ItemStats {
	if dto == nil {
		return nil
	}

	agePairs := parseTimeValuePairs(dto.Age)
	heartRatePairs := parseTimeValuePairs(dto.HeartRate)
	foodIntakePairs := parseTimeValuePairs(dto.FoodIntake)

	log.Infof("📋 [mapGraphqlItemStats] Parsed: age=%d, heartRate=%d, foodIntake=%d, parentExtId=%s",
		len(agePairs), len(heartRatePairs), len(foodIntakePairs), parentExtId)

	stat := &statsPb.ItemStats{}

	if parentExtId != "" {
		stat.ItemExtId = &parentExtId
	}

	if len(agePairs) > 0 {
		pairs := make([]*statsPb.IntegerTimeValuePair, 0, len(agePairs))
		for _, tv := range agePairs {
			if f, ok := tvFloat64(tv); ok {
				v32 := int32(f)
				ts := timestamppb.New(time.UnixMicro(tv.Time))
				pairs = append(pairs, &statsPb.IntegerTimeValuePair{Value: &v32, Timestamp: ts})
			}
		}
		stat.Age = &statsPb.IntegerTimeValuePairArrayWrapper{Value: pairs}
	}
	if len(heartRatePairs) > 0 {
		pairs := make([]*statsPb.IntegerTimeValuePair, 0, len(heartRatePairs))
		for _, tv := range heartRatePairs {
			if f, ok := tvFloat64(tv); ok {
				v32 := int32(f)
				ts := timestamppb.New(time.UnixMicro(tv.Time))
				pairs = append(pairs, &statsPb.IntegerTimeValuePair{Value: &v32, Timestamp: ts})
			}
		}
		stat.HeartRate = &statsPb.IntegerTimeValuePairArrayWrapper{Value: pairs}
	}
	if len(foodIntakePairs) > 0 {
		pairs := make([]*statsPb.DoubleTimeValuePair, 0, len(foodIntakePairs))
		for _, tv := range foodIntakePairs {
			if f, ok := tvFloat64(tv); ok {
				ts := timestamppb.New(time.UnixMicro(tv.Time))
				pairs = append(pairs, &statsPb.DoubleTimeValuePair{Value: &f, Timestamp: ts})
			}
		}
		stat.FoodIntake = &statsPb.DoubleTimeValuePairArrayWrapper{Value: pairs}
	}

	if stat.Age == nil && stat.HeartRate == nil && stat.FoodIntake == nil {
		return nil
	}
	return stat
}

// MapGroupedGraphqlToItemGroups maps grouped GraphQL response to protobuf ItemGroups
// This follows categories' makeGroupsResponseGraphql pattern.
func MapGroupedGraphqlToItemGroups(graphqlRet *ItemGroupGraphqlReturnDto, expansionKey string, groupByColumn string) ([]*pb.ItemGroup, int64) {
	if graphqlRet == nil {
		return nil, 0
	}

	itemGroups := make([]*pb.ItemGroup, 0, len(graphqlRet.ItemGroups))

	for _, groupDto := range graphqlRet.ItemGroups {
		// Map items in this group
		items := make([]*pb.Item, 0, len(groupDto.Items))
		for _, itemDto := range groupDto.Items {
			item := mapGraphqlItemFields(&itemDto, expansionKey)
			items = append(items, item)
		}

		// Build group key from group_by_column_value
		groupKey := buildGroupKeyFromGraphql(groupDto.GroupByColumnValue, groupByColumn)
		if groupKey == nil {
			log.Warnf("Could not build group key from GraphQL value: %v (column: %s)", groupDto.GroupByColumnValue, groupByColumn)
			continue
		}

		// Build aggregates from AggregateColumnSummaries
		var aggregatesWrapper *pb.ItemAggregateArrayWrapper
		aggregates := mapGraphqlAggregates(groupDto.AggregateColumnSummaries)
		if len(aggregates) > 0 {
			aggregatesWrapper = &pb.ItemAggregateArrayWrapper{Value: aggregates}
		}

		itemGroup := &pb.ItemGroup{
			Data: &pb.ItemGroup_ItemArrayData{
				ItemArrayData: &pb.ItemArrayWrapper{Value: items},
			},
			Aggregates: aggregatesWrapper,
			Metadata: &response.ApiResponseMetadata{
				TotalAvailableResults: proto.Int32(int32(groupDto.GroupEntityCount)),
			},
		}

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
		}

		itemGroups = append(itemGroups, itemGroup)
	}

	return itemGroups, graphqlRet.TotalGroupCount
}

// buildGroupKeyFromGraphql converts a GraphQL group_by_column_value to a protobuf group key
func buildGroupKeyFromGraphql(value interface{}, groupByColumn string) interface{} {
	if value == nil {
		return nil
	}

	log.Infof("📋 [buildGroupKey] value=%v (type=%T), groupByColumn=%s", value, value, groupByColumn)

	switch groupByColumn {
	case "item_type":
		if strVal, ok := value.(string); ok {
			return &pb.ItemGroup_StringGroup{StringGroup: &pb.StringWrapper{Value: proto.String(strVal)}}
		}
		if numVal, ok := value.(float64); ok {
			return &pb.ItemGroup_StringGroup{StringGroup: &pb.StringWrapper{Value: proto.String(fmt.Sprintf("%.0f", numVal))}}
		}
	case "is_active":
		if bVal, ok := value.(bool); ok {
			return &pb.ItemGroup_BooleanGroup{BooleanGroup: &pb.BooleanWrapper{Value: proto.Bool(bVal)}}
		}
		if strVal, ok := value.(string); ok {
			b := strVal == "true" || strVal == "1"
			return &pb.ItemGroup_BooleanGroup{BooleanGroup: &pb.BooleanWrapper{Value: proto.Bool(b)}}
		}
	case "item_id", "quantity", "priority":
		if numVal, ok := value.(float64); ok {
			v := int64(numVal)
			return &pb.ItemGroup_Int64Group{Int64Group: &pb.Int64Wrapper{Value: &v}}
		}
	case "price":
		if numVal, ok := value.(float64); ok {
			return &pb.ItemGroup_DoubleGroup{DoubleGroup: &pb.DoubleWrapper{Value: &numVal}}
		}
	default:
		if strVal, ok := value.(string); ok {
			return &pb.ItemGroup_StringGroup{StringGroup: &pb.StringWrapper{Value: proto.String(strVal)}}
		}
		if numVal, ok := value.(float64); ok {
			s := fmt.Sprintf("%v", numVal)
			return &pb.ItemGroup_StringGroup{StringGroup: &pb.StringWrapper{Value: &s}}
		}
	}

	s := fmt.Sprintf("%v", value)
	return &pb.ItemGroup_StringGroup{StringGroup: &pb.StringWrapper{Value: &s}}
}

// mapGraphqlAggregates maps GraphQL aggregate DTOs to protobuf ItemAggregate
func mapGraphqlAggregates(summaries []AggregateDataGraphqlDto) []*pb.ItemAggregate {
	if len(summaries) == 0 {
		return nil
	}
	aggregates := make([]*pb.ItemAggregate, 0, len(summaries))
	for _, summary := range summaries {
		if summary.Data == nil {
			continue
		}
		label := fmt.Sprintf("%s(%s)", summary.Operator, summary.Data.Name)
		agg := &pb.ItemAggregate{
			Label: &label,
		}
		if len(summary.Data.Values) > 0 {
			val := summary.Data.Values[0].Values
			agg.Result = &pb.ItemAggregate_Int64Result{Int64Result: &pb.Int64Wrapper{Value: &val}}
		}
		aggregates = append(aggregates, agg)
	}
	return aggregates
}

func parseInt32(s string) (int32, error) {
	v, err := strconv.ParseInt(s, 10, 32)
	return int32(v), err
}

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func parseFloat64(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}
