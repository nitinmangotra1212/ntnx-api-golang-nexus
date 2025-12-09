/*
 * GraphQL Response Mapper
 * Maps GraphQL response (JSON) to protobuf Item objects
 * Based on categories implementation pattern
 */

package idf

import (
	"encoding/json"
	"fmt"

	"github.com/nutanix-core/ntnx-api-odata-go/db/idfgraphql"
	"github.com/nutanix-core/ntnx-api-odata-go/odata/edm"
	"github.com/nutanix-core/ntnx-api-odata-go/odata/uri/parser"
	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/models"
	log "github.com/sirupsen/logrus"
)

// ItemGraphqlRetDto represents the GraphQL response structure
type ItemGraphqlRetDto struct {
	Items         []ItemGraphqlDto `json:"item"`
	TotalCount    int              `json:"total_entity_count"`
	FilteredCount int              `json:"filtered_entity_count"`
}

// ItemGraphqlDto represents a single item in GraphQL response
type ItemGraphqlDto struct {
	ItemId      []int32  `json:"item_id"`
	ItemName    []string `json:"item_name"`
	ItemType    []string `json:"item_type"`
	Description []string `json:"description"`
	ExtId       []string `json:"extId"`
	// Expanded entities
	Associations *AssociationGraphQLDto `json:"associations,omitempty"`
}

// AssociationGraphQLDto represents expanded associations in GraphQL response
type AssociationGraphQLDto struct {
	ItemId     []string `json:"item_id"`
	EntityType []string `json:"entity_type"`
	EntityId   []string `json:"entity_id"`
	Count      []int32  `json:"count"`
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

// MapGraphqlToItems maps GraphQL DTOs to protobuf Item objects
func MapGraphqlToItems(graphqlRet *ItemGraphqlRetDto, expansionKey string) ([]*pb.Item, error) {
	if graphqlRet == nil {
		return nil, fmt.Errorf("nil GraphQL response")
	}

	items := make([]*pb.Item, 0, len(graphqlRet.Items))

	for _, itemDto := range graphqlRet.Items {
		item := &pb.Item{}

		// Map basic fields (take first element from arrays, following categories pattern)
		if len(itemDto.ItemId) > 0 {
			item.ItemId = &itemDto.ItemId[0]
		}
		if len(itemDto.ItemName) > 0 {
			item.ItemName = &itemDto.ItemName[0]
		}
		if len(itemDto.ItemType) > 0 {
			item.ItemType = &itemDto.ItemType[0]
		}
		if len(itemDto.Description) > 0 {
			item.Description = &itemDto.Description[0]
		}
		if len(itemDto.ExtId) > 0 {
			item.ExtId = &itemDto.ExtId[0]
		}

		// Map expanded associations if present
		if expansionKey != "" && itemDto.Associations != nil {
			associations := mapGraphqlAssociations(itemDto.Associations)

			// Apply nested expand options (filter, select, orderby)
			// Examples:
			//   - $expand=associations($filter=entityType eq 'vm')
			//   - $expand=associations($select=entityType,count)
			//   - $expand=associations($orderby=entityType asc)
			expandOptions := ParseExpandOptions(expansionKey)
			if expandOptions != nil {
				associations = ApplyExpandOptions(associations, expandOptions)
				log.Debugf("Applied expand options to GraphQL associations: filter=%v, select=%v, orderby=%v",
					expandOptions.Filter != nil, expandOptions.Select != nil, expandOptions.OrderBy != nil)
			}

			item.Associations = &pb.ItemAssociationArrayWrapper{
				Value: associations,
			}
			log.Debugf("Mapped %d associations for item %s", len(associations), itemDto.ExtId[0])
		}

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
		if i < len(ascGraphql.Count) {
			assoc.Count = &ascGraphql.Count[i]
		}

		associations = append(associations, assoc)
	}

	return associations
}

// GenerateGraphQLQuery generates GraphQL query string from OData parsed query
// This uses the GraphQL evaluator from ntnx-api-odata-go
func GenerateGraphQLQuery(queryParams *models.QueryParams, resourcePath string) (string, error) {
	// Get entity bindings for nexus module (including ItemAssociation for expand)
	entityBindingList := GetNexusEntityBindings()

	// Create EDM provider
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
	log.Debugf("Parsing OData query with expand: %s, resourcePath: %s", queryParams.Expand, resourcePath)
	uriInfo, parseErr := odataParser.ParserWithQueryParam(queryParam, resourcePath)
	if parseErr != nil {
		log.Errorf("❌ Failed to Parse OData expression for GraphQL: %v", parseErr)
		log.Errorf("   Expand parameter: %s", queryParams.Expand)
		log.Errorf("   Resource path: %s", resourcePath)
		log.Errorf("   EDM bindings count: %d", len(entityBindingList))
		for i, binding := range entityBindingList {
			if binding.EntityType != nil {
				log.Errorf("   EDM binding[%d]: entity=%s, entitySet=%s", i, binding.EntityType.Name, binding.EntitySet.Name)
			}
		}
		return "", fmt.Errorf("invalid OData query: %w", parseErr)
	}

	// Use GraphQL query evaluator
	idfGraphqlQueryEval := idfgraphql.IdfGraphqlQueryEvaluator{}
	graphqlQuery, err := idfGraphqlQueryEval.GetQuery(uriInfo, resourcePath)
	if err != nil {
		log.Errorf("Failed to Evaluate GraphQL expression: %v", err)
		return "", fmt.Errorf("failed to evaluate GraphQL query: %w", err)
	}

	log.Infof("Generated GraphQL query: %s", graphqlQuery)
	return graphqlQuery, nil
}
