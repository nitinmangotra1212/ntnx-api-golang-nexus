/*
 * IDF Repository Implementation for Cat Entity
 * Handles CRUD operations for Cat, CatStats, PetFood, PetCare
 * Maps between protobuf models (camelCase) and IDF attributes (snake_case)
 *
 * Supports Config+Stats Feature:
 *   - Stats expansion with time-series parameters ($startTime, $endTime, $samplingInterval)
 *   - Query flipping when ordering by stats attributes
 *   - GraphQL queries to StatsGateway for stats data
 *
 * Based on Confluence: "Fetch Stats and Config Data Together using V4 APIs"
 */

package idf

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nutanix-core/go-cache/insights/insights_interface"
	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/external"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/models"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/odata"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

// CatRepository defines the interface for Cat data operations
type CatRepository interface {
	ListCats(queryParams *models.QueryParams) ([]*pb.Cat, int64, error)
	GetCatById(extId string) (*pb.Cat, error)
	CreateCat(cat *pb.Cat) error
	UpdateCat(extId string, cat *pb.Cat) error
	DeleteCat(extId string) error
}

// CatRepositoryImpl implements CatRepository using IDF
type CatRepositoryImpl struct{}

// NewCatRepository creates a new CatRepository instance
func NewCatRepository() CatRepository {
	return &CatRepositoryImpl{}
}

// ListCats retrieves a list of cats from IDF with pagination and filtering
// Uses the SAME OData parser as Items - no duplication!
//
// Supports Config+Stats Feature:
//   - Stats expansion: $expand=stats($startTime=...;$endTime=...;$statType=AVG)
//   - Query flipping: $expand=stats($orderby=stats/heartRate desc)
//   - Nested OData: $expand=stats($filter=heartRate gt 70;$select=heartRate,weight)
func (r *CatRepositoryImpl) ListCats(queryParams *models.QueryParams) ([]*pb.Cat, int64, error) {
	var cats []*pb.Cat
	var totalCount int64

	// Check if $expand is requested
	if queryParams.Expand != "" {
		log.Infof("Using expand path for Cat: %s", queryParams.Expand)

		// Parse the expand parameter for stats-specific options
		statsParams := odata.ParseStatsExpand(queryParams.Expand)
		
		// Check if this is a stats expansion with query flipping
		if statsParams != nil && statsParams.ShouldFlipQuery() {
			log.Infof("🔄 Query flipping detected - will query stats first, then join config")
			return r.listCatsWithFlippedQuery(queryParams, statsParams)
		}

		// Check if this is a stats expansion (use StatsGW)
		expandFields := odata.GetExpandFieldNames(queryParams.Expand)
		for _, field := range expandFields {
			if odata.IsStatsExpansion(field) {
				log.Infof("📊 Stats expansion detected - using StatsGW for %s", field)
				return r.listCatsWithStatsExpansion(queryParams, statsParams)
			}
		}
		
		// Fallback: Non-stats expansion (IDF-only path)
		queryParamsWithoutExpand := *queryParams
		queryParamsWithoutExpand.Expand = ""

		// Get cats first
		queryArg, err := GenerateListQuery(&queryParamsWithoutExpand, catListPath, catEntityTypeName, catIdAttr)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to generate IDF query: %w", err)
		}

		idfClient := external.Interfaces().IdfClient()
		queryResponse, err := idfClient.GetEntitiesWithMetricsRet(queryArg)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to query IDF: %w", err)
		}

		groupResults := queryResponse.GetGroupResultsList()
		if len(groupResults) == 0 {
			return []*pb.Cat{}, 0, nil
		}

		entitiesWithMetric := groupResults[0].GetRawResults()
		entities := ConvertEntitiesWithMetricToEntities(entitiesWithMetric)
		for _, entity := range entities {
			cat := r.mapIdfAttributeToCat(entity)
			cats = append(cats, cat)
		}

		// Now fetch expanded entities based on $expand value (non-stats entities)
		if err := r.fetchExpandedEntities(cats, queryParams.Expand); err != nil {
			log.Warnf("Failed to fetch expanded entities: %v", err)
			// Continue without expanded data
		}

		totalCount = groupResults[0].GetTotalEntityCount()
		log.Infof("✅ Retrieved %d cats from IDF (total: %d) with expand", len(cats), totalCount)

	} else {
		// Regular IDF path (no expand)
		queryArg, err := GenerateListQuery(queryParams, catListPath, catEntityTypeName, catIdAttr)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to generate IDF query: %w", err)
		}

		idfClient := external.Interfaces().IdfClient()
		queryResponse, err := idfClient.GetEntitiesWithMetricsRet(queryArg)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to query IDF: %w", err)
		}

		groupResults := queryResponse.GetGroupResultsList()
		if len(groupResults) == 0 {
			return []*pb.Cat{}, 0, nil
		}

		entitiesWithMetric := groupResults[0].GetRawResults()
		entities := ConvertEntitiesWithMetricToEntities(entitiesWithMetric)
		for _, entity := range entities {
			cat := r.mapIdfAttributeToCat(entity)
			cats = append(cats, cat)
		}

		totalCount = groupResults[0].GetTotalEntityCount()
		log.Infof("✅ Retrieved %d cats from IDF (total: %d)", len(cats), totalCount)
	}

	return cats, totalCount, nil
}

// listCatsWithStatsExpansion fetches cats with stats using StatsGW GraphQL
// This is the Config → Stats path (normal query)
func (r *CatRepositoryImpl) listCatsWithStatsExpansion(queryParams *models.QueryParams, statsParams *odata.StatsExpandParams) ([]*pb.Cat, int64, error) {
	ctx := context.Background()
	
	// Step 1: Generate GraphQL query
	entityConfig := odata.CatStatsEntityConfig()
	graphqlQuery, err := odata.GenerateStatsExpansionQuery(entityConfig, statsParams)
	if err != nil {
		log.Errorf("Failed to generate GraphQL query for stats expansion: %v", err)
		// Fallback to IDF-only path
		return r.listCatsWithIdfStats(queryParams)
	}

	log.Infof("Generated GraphQL query for stats expansion:\n%s", graphqlQuery.Query)

	// Step 2: Execute GraphQL query via StatsGW
	executor := odata.NewStatsExecutor()
	response, err := executor.ExecuteGraphqlQuerySync(ctx, graphqlQuery.Query)
	if err != nil {
		log.Warnf("StatsGW query failed, falling back to IDF: %v", err)
		// Fallback to IDF-only path
		return r.listCatsWithIdfStats(queryParams)
	}

	// Step 3: Map GraphQL response to protobuf
	cats, totalCount := r.mapGraphqlResponseToCats(response, graphqlQuery.IsFlipped)
	
	log.Infof("✅ Retrieved %d cats with stats from StatsGW (total: %d)", len(cats), totalCount)
	return cats, totalCount, nil
}

// listCatsWithFlippedQuery handles the case when ordering by stats attribute
// This uses the Stats → Config path (flipped query)
func (r *CatRepositoryImpl) listCatsWithFlippedQuery(queryParams *models.QueryParams, statsParams *odata.StatsExpandParams) ([]*pb.Cat, int64, error) {
	ctx := context.Background()
	
	log.Infof("🔄 Executing flipped query (Stats → Config) for orderby: %s", statsParams.FlippedOrderBy)

	// Step 1: Generate flipped GraphQL query
	entityConfig := odata.CatStatsEntityConfig()
	graphqlQuery, err := odata.GenerateStatsExpansionQuery(entityConfig, statsParams)
	if err != nil {
		log.Errorf("Failed to generate flipped GraphQL query: %v", err)
		return r.listCatsWithIdfStats(queryParams)
	}

	log.Infof("Generated FLIPPED GraphQL query:\n%s", graphqlQuery.Query)

	// Step 2: Execute GraphQL query
	executor := odata.NewStatsExecutor()
	response, err := executor.ExecuteGraphqlQuerySync(ctx, graphqlQuery.Query)
	if err != nil {
		log.Warnf("StatsGW flipped query failed, falling back to IDF: %v", err)
		return r.listCatsWithIdfStats(queryParams)
	}

	// Step 3: Transform flipped response back to normal format
	response = odata.TransformFlippedResponse(response)

	// Step 4: Map to protobuf
	cats, totalCount := r.mapGraphqlResponseToCats(response, false)
	
	log.Infof("✅ Retrieved %d cats with flipped stats query (total: %d)", len(cats), totalCount)
	return cats, totalCount, nil
}

// listCatsWithIdfStats is the fallback path using only IDF (no StatsGW)
func (r *CatRepositoryImpl) listCatsWithIdfStats(queryParams *models.QueryParams) ([]*pb.Cat, int64, error) {
	log.Infof("Using IDF-only fallback for stats expansion")
	
	// Get cats without expand first
	queryParamsWithoutExpand := *queryParams
	queryParamsWithoutExpand.Expand = ""
	
	queryArg, err := GenerateListQuery(&queryParamsWithoutExpand, catListPath, catEntityTypeName, catIdAttr)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to generate IDF query: %w", err)
	}

	idfClient := external.Interfaces().IdfClient()
	queryResponse, err := idfClient.GetEntitiesWithMetricsRet(queryArg)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query IDF: %w", err)
	}

	groupResults := queryResponse.GetGroupResultsList()
	if len(groupResults) == 0 {
		return []*pb.Cat{}, 0, nil
	}

	var cats []*pb.Cat
	entitiesWithMetric := groupResults[0].GetRawResults()
	entities := ConvertEntitiesWithMetricToEntities(entitiesWithMetric)
	for _, entity := range entities {
		cat := r.mapIdfAttributeToCat(entity)
		cats = append(cats, cat)
	}

	// Fetch stats from IDF
	if err := r.fetchCatStats(cats, nil); err != nil {
		log.Warnf("Failed to fetch cat stats from IDF: %v", err)
	}

	totalCount := groupResults[0].GetTotalEntityCount()
	return cats, totalCount, nil
}

// mapGraphqlResponseToCats maps GraphQL response to Cat protobuf objects
func (r *CatRepositoryImpl) mapGraphqlResponseToCats(response *odata.StatsGraphqlResponse, isFlipped bool) ([]*pb.Cat, int64) {
	if response == nil {
		return []*pb.Cat{}, 0
	}

	cats := make([]*pb.Cat, 0, len(response.ConfigEntities))
	
	for _, configEntity := range response.ConfigEntities {
		cat := &pb.Cat{}
		
		// Map config fields
		if catId, ok := getGraphqlInt(configEntity, "cat_id"); ok {
			val := int32(catId)
			cat.CatId = &val
		}
		if catName, ok := getGraphqlString(configEntity, "cat_name"); ok {
			cat.CatName = &catName
		}
		if catType, ok := getGraphqlString(configEntity, "cat_type"); ok {
			cat.CatType = &catType
		}
		if weight, ok := getGraphqlFloat(configEntity, "weight"); ok {
			cat.Weight = &weight
		}
		if age, ok := getGraphqlInt(configEntity, "age"); ok {
			val := int32(age)
			cat.Age = &val
		}
		if extId, ok := getGraphqlString(configEntity, "ext_id"); ok {
			cat.ExtId = &extId
		}
		if extId, ok := getGraphqlString(configEntity, "_entity_id_"); ok {
			cat.ExtId = &extId
		}

		// Map nested stats if present
		if statsData, ok := configEntity["cat_stats"].([]interface{}); ok {
			var stats []*pb.CatStats
			for _, s := range statsData {
				if statMap, ok := s.(map[string]interface{}); ok {
					stat := r.mapGraphqlToCatStats(statMap)
					stats = append(stats, stat)
				}
			}
			if len(stats) > 0 {
				cat.Stats = &pb.CatStatsArrayWrapper{Value: stats}
			}
		}

		cats = append(cats, cat)
	}

	totalCount := response.TotalCount
	if totalCount == 0 {
		totalCount = int64(len(cats))
	}

	return cats, totalCount
}

// mapGraphqlToCatStats maps a single GraphQL stats entity to CatStats protobuf
func (r *CatRepositoryImpl) mapGraphqlToCatStats(statMap map[string]interface{}) *pb.CatStats {
	stat := &pb.CatStats{}

	if catId, ok := getGraphqlString(statMap, "cat_id"); ok {
		stat.CatId = &catId
	}
	if timestamp, ok := getGraphqlInt(statMap, "timestamp"); ok {
		stat.Timestamp = &timestamp
	}
	if heartRate, ok := getGraphqlInt(statMap, "heart_rate"); ok {
		val := int32(heartRate)
		stat.HeartRate = &val
	}
	if foodIntake, ok := getGraphqlFloat(statMap, "food_intake"); ok {
		stat.FoodIntake = &foodIntake
	}
	if sleepHours, ok := getGraphqlFloat(statMap, "sleep_hours"); ok {
		stat.SleepHours = &sleepHours
	}
	if weight, ok := getGraphqlFloat(statMap, "weight"); ok {
		stat.Weight = &weight
	}
	if age, ok := getGraphqlInt(statMap, "age"); ok {
		val := int32(age)
		stat.Age = &val
	}

	return stat
}

// Helper functions for extracting GraphQL response values
func getGraphqlString(entity map[string]interface{}, key string) (string, bool) {
	// Try array first (GraphQL parallel array pattern)
	if arr, ok := entity[key].([]interface{}); ok && len(arr) > 0 {
		if str, ok := arr[0].(string); ok {
			return str, true
		}
	}
	// Try direct value
	if str, ok := entity[key].(string); ok {
		return str, true
	}
	return "", false
}

func getGraphqlInt(entity map[string]interface{}, key string) (int64, bool) {
	if arr, ok := entity[key].([]interface{}); ok && len(arr) > 0 {
		if num, ok := arr[0].(float64); ok {
			return int64(num), true
		}
	}
	if num, ok := entity[key].(float64); ok {
		return int64(num), true
	}
	return 0, false
}

func getGraphqlFloat(entity map[string]interface{}, key string) (float64, bool) {
	if arr, ok := entity[key].([]interface{}); ok && len(arr) > 0 {
		if num, ok := arr[0].(float64); ok {
			return num, true
		}
	}
	if num, ok := entity[key].(float64); ok {
		return num, true
	}
	return 0, false
}

// GetCatById retrieves a cat by its external ID
func (r *CatRepositoryImpl) GetCatById(extId string) (*pb.Cat, error) {
	getArg := &insights_interface.GetEntitiesArg{
		EntityGuidList: []*insights_interface.EntityGuid{
			{
				EntityTypeName: proto.String(catEntityTypeName),
				EntityId:       &extId,
			},
		},
	}

	idfClient := external.Interfaces().IdfClient()
	getResponse, err := idfClient.GetEntityRet(getArg)
	if err != nil {
		return nil, fmt.Errorf("failed to get cat by ID %s: %w", extId, err)
	}

	if len(getResponse.GetEntity()) == 0 {
		return nil, fmt.Errorf("cat not found: %s", extId)
	}

	entity := getResponse.GetEntity()[0]
	cat := r.mapIdfAttributeToCat(entity)

	return cat, nil
}

// CreateCat creates a new cat in IDF
func (r *CatRepositoryImpl) CreateCat(cat *pb.Cat) error {
	idfClient := external.Interfaces().IdfClient()

	// Generate UUID for ExtId if not provided
	var extIdUuid string
	if cat.ExtId != nil && *cat.ExtId != "" {
		extIdUuid = *cat.ExtId
	} else {
		extIdUuid = uuid.New().String()
	}

	attributeDataArgList := []*insights_interface.AttributeDataArg{}

	// Map protobuf fields to IDF attributes
	if cat.CatId != nil {
		AddAttribute(&attributeDataArgList, catIdAttr, *cat.CatId)
	}
	AddAttribute(&attributeDataArgList, catExtIdAttr, extIdUuid)
	if cat.CatName != nil {
		AddAttribute(&attributeDataArgList, catNameAttr, *cat.CatName)
	}
	if cat.CatType != nil {
		AddAttribute(&attributeDataArgList, catTypeAttr, *cat.CatType)
	}
	if cat.Weight != nil {
		AddDoubleAttribute(&attributeDataArgList, catWeightAttr, *cat.Weight)
	}
	if cat.Age != nil {
		AddAttribute(&attributeDataArgList, catAgeAttr, *cat.Age)
	}

	updateArg := &insights_interface.UpdateEntityArg{
		EntityGuid: &insights_interface.EntityGuid{
			EntityTypeName: proto.String(catEntityTypeName),
			EntityId:       &extIdUuid,
		},
		AttributeDataArgList: attributeDataArgList,
	}

	_, err := idfClient.UpdateEntityRet(updateArg)
	if err != nil {
		return fmt.Errorf("failed to create cat: %w", err)
	}

	log.Infof("Cat created successfully with extId: %s", extIdUuid)
	return nil
}

// UpdateCat updates an existing cat in IDF
func (r *CatRepositoryImpl) UpdateCat(extId string, cat *pb.Cat) error {
	attributeDataArgList := []*insights_interface.AttributeDataArg{}

	if cat.CatId != nil {
		AddAttribute(&attributeDataArgList, catIdAttr, *cat.CatId)
	}
	
	updateExtId := extId
	if cat.ExtId != nil && *cat.ExtId != "" {
		updateExtId = *cat.ExtId
	}
	if updateExtId != "" {
		AddAttribute(&attributeDataArgList, catExtIdAttr, updateExtId)
	}

	if cat.CatName != nil {
		AddAttribute(&attributeDataArgList, catNameAttr, *cat.CatName)
	}
	if cat.CatType != nil {
		AddAttribute(&attributeDataArgList, catTypeAttr, *cat.CatType)
	}
	if cat.Weight != nil {
		AddDoubleAttribute(&attributeDataArgList, catWeightAttr, *cat.Weight)
	}
	if cat.Age != nil {
		AddAttribute(&attributeDataArgList, catAgeAttr, *cat.Age)
	}

	updateArg := &insights_interface.UpdateEntityArg{
		EntityGuid: &insights_interface.EntityGuid{
			EntityTypeName: proto.String(catEntityTypeName),
			EntityId:       &extId,
		},
		AttributeDataArgList: attributeDataArgList,
	}

	idfClient := external.Interfaces().IdfClient()
	_, err := idfClient.UpdateEntityRet(updateArg)
	if err != nil {
		return fmt.Errorf("failed to update cat %s: %w", extId, err)
	}

	log.Infof("Cat updated successfully: %s", extId)
	return nil
}

// DeleteCat deletes a cat from IDF
func (r *CatRepositoryImpl) DeleteCat(extId string) error {
	log.Warnf("DeleteCat not yet implemented for IDF. ExtId: %s", extId)
	return fmt.Errorf("delete operation not yet implemented")
}

// mapIdfAttributeToCat maps IDF attributes (snake_case) to protobuf Cat (camelCase)
func (r *CatRepositoryImpl) mapIdfAttributeToCat(entity *insights_interface.Entity) *pb.Cat {
	cat := &pb.Cat{}

	// Get extId from EntityGuid
	if entity.GetEntityGuid() != nil && entity.GetEntityGuid().GetEntityId() != "" {
		extId := entity.GetEntityGuid().GetEntityId()
		cat.ExtId = &extId
	}

	for _, attr := range entity.GetAttributeDataMap() {
		switch attr.GetName() {
		case catIdAttr:
			if attr.GetValue() != nil && attr.GetValue().GetInt64Value() != 0 {
				val := int32(attr.GetValue().GetInt64Value())
				cat.CatId = &val
			}
		case catNameAttr:
			if attr.GetValue() != nil {
				val := attr.GetValue().GetStrValue()
				cat.CatName = &val
			}
		case catTypeAttr:
			if attr.GetValue() != nil {
				val := attr.GetValue().GetStrValue()
				cat.CatType = &val
			}
		case catWeightAttr:
			if attr.GetValue() != nil {
				val := attr.GetValue().GetDoubleValue()
				cat.Weight = &val
			}
		case catAgeAttr:
			if attr.GetValue() != nil {
				val := int32(attr.GetValue().GetInt64Value())
				cat.Age = &val
			}
		case catExtIdAttr:
			if attr.GetValue() != nil {
				val := attr.GetValue().GetStrValue()
				if val != "" {
					cat.ExtId = &val
				}
			}
		}
	}

	return cat
}

// fetchExpandedEntities fetches related entities based on $expand parameter
func (r *CatRepositoryImpl) fetchExpandedEntities(cats []*pb.Cat, expand string) error {
	if len(cats) == 0 {
		return nil
	}

	// Collect all cat extIds
	extIds := make([]string, 0, len(cats))
	for _, cat := range cats {
		if cat.ExtId != nil && *cat.ExtId != "" {
			extIds = append(extIds, *cat.ExtId)
		}
	}

	if len(extIds) == 0 {
		return nil
	}

	// Parse expand options and fetch appropriate data
	expandOptions := ParseExpandOptions(expand)
	expandField := getExpandFieldName(expand)

	switch expandField {
	case "stats":
		return r.fetchCatStats(cats, extIds)
	case "petFood":
		return r.fetchPetFood(cats, extIds)
	case "petCare":
		return r.fetchPetCare(cats, extIds)
	default:
		log.Warnf("Unknown expand field: %s (options: %+v)", expand, expandOptions)
	}

	return nil
}

// getExpandFieldName extracts the field name from $expand parameter
func getExpandFieldName(expand string) string {
	// Handle nested expand like "stats($filter=...)"
	if len(expand) > 0 {
		// Find the first ( or end of string
		for i, c := range expand {
			if c == '(' {
				return expand[:i]
			}
		}
		return expand
	}
	return ""
}

// fetchCatStats fetches stats for cats and attaches them
func (r *CatRepositoryImpl) fetchCatStats(cats []*pb.Cat, catExtIds []string) error {
	idfClient := external.Interfaces().IdfClient()

	// Query cat_stats entity
	queryArg, err := GenerateListQueryForEntity(catStatsEntityTypeName,
		[]string{statsCatIdAttr, statsTimestampAttr, statsHeartRateAttr, statsFoodIntakeAttr, statsSleepHoursAttr, statsWeightAttr, statsAgeAttr})
	if err != nil {
		return err
	}

	queryResponse, err := idfClient.GetEntitiesWithMetricsRet(queryArg)
	if err != nil {
		return err
	}

	// Build map of catId -> stats
	statsMap := make(map[string][]*pb.CatStats)
	groupResults := queryResponse.GetGroupResultsList()
	if len(groupResults) > 0 {
		entitiesWithMetric := groupResults[0].GetRawResults()
		entities := ConvertEntitiesWithMetricToEntities(entitiesWithMetric)
		for _, entity := range entities {
			stat := r.mapIdfAttributeToCatStats(entity)
			if stat.CatId != nil {
				statsMap[*stat.CatId] = append(statsMap[*stat.CatId], stat)
			}
		}
	}

	// Attach stats to cats
	for _, cat := range cats {
		if cat.ExtId != nil {
			if stats, found := statsMap[*cat.ExtId]; found {
				cat.Stats = &pb.CatStatsArrayWrapper{Value: stats}
				log.Debugf("Attached %d stats to cat %s", len(stats), *cat.ExtId)
			}
		}
	}

	return nil
}

// fetchPetFood fetches pet food for cats and attaches them
func (r *CatRepositoryImpl) fetchPetFood(cats []*pb.Cat, catExtIds []string) error {
	idfClient := external.Interfaces().IdfClient()

	queryArg, err := GenerateListQueryForEntity(petFoodEntityTypeName,
		[]string{petfoodIdAttr, petfoodNameAttr, petfoodDescriptionAttr, petfoodPetIdAttr, petfoodPriceAttr})
	if err != nil {
		return err
	}

	queryResponse, err := idfClient.GetEntitiesWithMetricsRet(queryArg)
	if err != nil {
		return err
	}

	// Build map of petId -> petFood
	petFoodMap := make(map[string][]*pb.PetFood)
	groupResults := queryResponse.GetGroupResultsList()
	if len(groupResults) > 0 {
		entitiesWithMetric := groupResults[0].GetRawResults()
		entities := ConvertEntitiesWithMetricToEntities(entitiesWithMetric)
		for _, entity := range entities {
			food := r.mapIdfAttributeToPetFood(entity)
			if food.PetfoodPetId != nil {
				petFoodMap[*food.PetfoodPetId] = append(petFoodMap[*food.PetfoodPetId], food)
			}
		}
	}

	// Attach pet food to cats
	for _, cat := range cats {
		if cat.ExtId != nil {
			if foods, found := petFoodMap[*cat.ExtId]; found {
				cat.PetFood = &pb.PetFoodArrayWrapper{Value: foods}
				log.Debugf("Attached %d pet foods to cat %s", len(foods), *cat.ExtId)
			}
		}
	}

	return nil
}

// fetchPetCare fetches pet care for cats and attaches them
func (r *CatRepositoryImpl) fetchPetCare(cats []*pb.Cat, catExtIds []string) error {
	idfClient := external.Interfaces().IdfClient()

	queryArg, err := GenerateListQueryForEntity(petCareEntityTypeName,
		[]string{petcareIdAttr, petcareNameAttr, petcareDescriptionAttr, petcarePetIdAttr, petcareAddressAttr})
	if err != nil {
		return err
	}

	queryResponse, err := idfClient.GetEntitiesWithMetricsRet(queryArg)
	if err != nil {
		return err
	}

	// Build map of petId -> petCare
	petCareMap := make(map[string][]*pb.PetCare)
	groupResults := queryResponse.GetGroupResultsList()
	if len(groupResults) > 0 {
		entitiesWithMetric := groupResults[0].GetRawResults()
		entities := ConvertEntitiesWithMetricToEntities(entitiesWithMetric)
		for _, entity := range entities {
			care := r.mapIdfAttributeToPetCare(entity)
			if care.PetcarePetId != nil {
				petCareMap[*care.PetcarePetId] = append(petCareMap[*care.PetcarePetId], care)
			}
		}
	}

	// Attach pet care to cats
	for _, cat := range cats {
		if cat.ExtId != nil {
			if cares, found := petCareMap[*cat.ExtId]; found {
				cat.PetCare = &pb.PetCareArrayWrapper{Value: cares}
				log.Debugf("Attached %d pet cares to cat %s", len(cares), *cat.ExtId)
			}
		}
	}

	return nil
}

// mapIdfAttributeToCatStats maps IDF attributes to CatStats protobuf
func (r *CatRepositoryImpl) mapIdfAttributeToCatStats(entity *insights_interface.Entity) *pb.CatStats {
	stat := &pb.CatStats{}

	for _, attr := range entity.GetAttributeDataMap() {
		switch attr.GetName() {
		case statsCatIdAttr:
			if attr.GetValue() != nil {
				val := attr.GetValue().GetStrValue()
				stat.CatId = &val
			}
		case statsTimestampAttr:
			if attr.GetValue() != nil {
				val := attr.GetValue().GetInt64Value()
				stat.Timestamp = &val
			}
		case statsHeartRateAttr:
			if attr.GetValue() != nil {
				val := int32(attr.GetValue().GetInt64Value())
				stat.HeartRate = &val
			}
		case statsFoodIntakeAttr:
			if attr.GetValue() != nil {
				val := attr.GetValue().GetDoubleValue()
				stat.FoodIntake = &val
			}
		case statsSleepHoursAttr:
			if attr.GetValue() != nil {
				val := attr.GetValue().GetDoubleValue()
				stat.SleepHours = &val
			}
		case statsWeightAttr:
			if attr.GetValue() != nil {
				val := attr.GetValue().GetDoubleValue()
				stat.Weight = &val
			}
		case statsAgeAttr:
			if attr.GetValue() != nil {
				val := int32(attr.GetValue().GetInt64Value())
				stat.Age = &val
			}
		}
	}

	return stat
}

// mapIdfAttributeToPetFood maps IDF attributes to PetFood protobuf
func (r *CatRepositoryImpl) mapIdfAttributeToPetFood(entity *insights_interface.Entity) *pb.PetFood {
	food := &pb.PetFood{}

	for _, attr := range entity.GetAttributeDataMap() {
		switch attr.GetName() {
		case petfoodIdAttr:
			if attr.GetValue() != nil {
				val := int32(attr.GetValue().GetInt64Value())
				food.PetfoodId = &val
			}
		case petfoodNameAttr:
			if attr.GetValue() != nil {
				val := attr.GetValue().GetStrValue()
				food.PetfoodName = &val
			}
		case petfoodDescriptionAttr:
			if attr.GetValue() != nil {
				val := attr.GetValue().GetStrValue()
				food.PetfoodDescription = &val
			}
		case petfoodPetIdAttr:
			if attr.GetValue() != nil {
				val := attr.GetValue().GetStrValue()
				food.PetfoodPetId = &val
			}
		case petfoodPriceAttr:
			if attr.GetValue() != nil {
				val := attr.GetValue().GetStrValue()
				food.PetfoodPrice = &val
			}
		}
	}

	return food
}

// mapIdfAttributeToPetCare maps IDF attributes to PetCare protobuf
func (r *CatRepositoryImpl) mapIdfAttributeToPetCare(entity *insights_interface.Entity) *pb.PetCare {
	care := &pb.PetCare{}

	for _, attr := range entity.GetAttributeDataMap() {
		switch attr.GetName() {
		case petcareIdAttr:
			if attr.GetValue() != nil {
				val := int32(attr.GetValue().GetInt64Value())
				care.PetcareId = &val
			}
		case petcareNameAttr:
			if attr.GetValue() != nil {
				val := attr.GetValue().GetStrValue()
				care.PetcareName = &val
			}
		case petcareDescriptionAttr:
			if attr.GetValue() != nil {
				val := attr.GetValue().GetStrValue()
				care.PetcareDescription = &val
			}
		case petcarePetIdAttr:
			if attr.GetValue() != nil {
				val := attr.GetValue().GetStrValue()
				care.PetcarePetId = &val
			}
		case petcareAddressAttr:
			if attr.GetValue() != nil {
				val := attr.GetValue().GetStrValue()
				care.PetcareAddress = &val
			}
		}
	}

	return care
}

// GenerateListQueryForEntity generates a simple IDF query for an entity type
func GenerateListQueryForEntity(entityType string, columns []string) (*insights_interface.GetEntitiesWithMetricsArg, error) {
	queryParams := &models.QueryParams{
		Page:  0,
		Limit: 1000, // Get all for expand
	}

	// Use a generic path for expand queries
	resourcePath := "/" + entityType + "s"

	return GenerateListQuery(queryParams, resourcePath, entityType, columns[0])
}

// AddDoubleAttribute adds a double attribute to the attribute list
func AddDoubleAttribute(attributeDataArgList *[]*insights_interface.AttributeDataArg, name string, value float64) {
	*attributeDataArgList = append(*attributeDataArgList, &insights_interface.AttributeDataArg{
		AttributeData: &insights_interface.AttributeData{
			Name: proto.String(name),
			Value: &insights_interface.DataValue{
				ValueType: &insights_interface.DataValue_DoubleValue{
					DoubleValue: value,
				},
			},
		},
	})
}

// ListCatStats retrieves stats for a specific cat or all cats
func (r *CatRepositoryImpl) ListCatStats(ctx context.Context, queryParams *models.QueryParams) ([]*pb.CatStats, int64, error) {
	queryArg, err := GenerateListQuery(queryParams, catStatsListPath, catStatsEntityTypeName, statsTimestampAttr)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to generate IDF query for cat_stats: %w", err)
	}

	idfClient := external.Interfaces().IdfClient()
	queryResponse, err := idfClient.GetEntitiesWithMetricsRet(queryArg)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query IDF for cat_stats: %w", err)
	}

	var stats []*pb.CatStats
	groupResults := queryResponse.GetGroupResultsList()
	if len(groupResults) == 0 {
		return []*pb.CatStats{}, 0, nil
	}

	entitiesWithMetric := groupResults[0].GetRawResults()
	entities := ConvertEntitiesWithMetricToEntities(entitiesWithMetric)
	for _, entity := range entities {
		stat := r.mapIdfAttributeToCatStats(entity)
		stats = append(stats, stat)
	}

	totalCount := groupResults[0].GetTotalEntityCount()
	log.Infof("✅ Retrieved %d cat stats from IDF (total: %d)", len(stats), totalCount)

	return stats, totalCount, nil
}

