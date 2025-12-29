/*
 * EDM Entity Bindings for Cat-related entities
 * Defines OData field mappings for: Cat, CatStats, PetFood, PetCare
 *
 * These bindings enable OData operations ($filter, $orderby, $select, $expand)
 * for all cat-related tables without additional OData implementation.
 */

package idf

import (
	"github.com/nutanix-core/ntnx-api-odata-go/odata/edm"
)

// IDF Entity Type Names (snake_case)
const (
	catEntityTypeName      = "cat"
	catStatsEntityTypeName = "cat_stats"
	petFoodEntityTypeName  = "pet_food"
	petCareEntityTypeName  = "pet_care"
)

// IDF Attribute Names for Cat (snake_case)
const (
	catIdAttr     = "cat_id"
	catNameAttr   = "cat_name"
	catTypeAttr   = "cat_type"
	catWeightAttr = "weight"
	catAgeAttr    = "age"
	catExtIdAttr  = "ext_id"
)

// IDF Attribute Names for CatStats (snake_case)
const (
	statsCatIdAttr      = "cat_id"
	statsTimestampAttr  = "timestamp"
	statsHeartRateAttr  = "heart_rate"
	statsFoodIntakeAttr = "food_intake"
	statsSleepHoursAttr = "sleep_hours"
	statsWeightAttr     = "weight"
	statsAgeAttr        = "age"
)

// IDF Attribute Names for PetFood (snake_case)
const (
	petfoodIdAttr          = "petfood_id"
	petfoodNameAttr        = "petfood_name"
	petfoodDescriptionAttr = "petfood_description"
	petfoodPetIdAttr       = "petfood_pet_id"
	petfoodPriceAttr       = "petfood_price"
)

// IDF Attribute Names for PetCare (snake_case)
const (
	petcareIdAttr          = "petcare_id"
	petcareNameAttr        = "petcare_name"
	petcareDescriptionAttr = "petcare_description"
	petcarePetIdAttr       = "petcare_pet_id"
	petcareAddressAttr     = "petcare_address"
)

// Resource paths for OData
const (
	catListPath      = "/cats"
	catStatsListPath = "/catstats"
	petFoodListPath  = "/petfoods"
	petCareListPath  = "/petcares"
)

// createCatEntityBinding creates an EDM binding for the Cat entity (Config)
// This maps OData field names (camelCase) to IDF attribute names (snake_case)
func createCatEntityBinding() *edm.EdmEntityBinding {
	binding := new(edm.EdmEntityBinding)

	// Property Mappings (OData field name → IDF column name)
	binding.PropertyMappings = make(map[string]string)
	binding.PropertyMappings["catId"] = catIdAttr       // "cat_id"
	binding.PropertyMappings["catName"] = catNameAttr   // "cat_name"
	binding.PropertyMappings["catType"] = catTypeAttr   // "cat_type"
	binding.PropertyMappings["weight"] = catWeightAttr  // "weight"
	binding.PropertyMappings["age"] = catAgeAttr        // "age"
	binding.PropertyMappings["extId"] = catExtIdAttr    // "ext_id"

	// Filterable properties (can be used in $filter)
	filterProperties := make(map[string]bool)
	filterProperties["catId"] = true
	filterProperties["catName"] = true
	filterProperties["catType"] = true
	filterProperties["weight"] = true
	filterProperties["age"] = true
	filterProperties["extId"] = true

	// Sortable properties (can be used in $orderby)
	sortableProperties := make(map[string]bool)
	sortableProperties["catId"] = true
	sortableProperties["catName"] = true
	sortableProperties["catType"] = true
	sortableProperties["weight"] = true
	sortableProperties["age"] = true

	// Create properties for Cat entity
	var properties []*edm.EdmProperty

	// catId property
	catIdProp := new(edm.EdmProperty)
	catIdProp.Name = "catId"
	catIdProp.IsCollection = false
	catIdProp.Type = string(edm.EdmInt64)
	catIdProp.MappedName = binding.PropertyMappings["catId"]
	catIdProp.IsFilterable = filterProperties["catId"]
	catIdProp.IsSortable = sortableProperties["catId"]
	properties = append(properties, catIdProp)

	// catName property
	catNameProp := new(edm.EdmProperty)
	catNameProp.Name = "catName"
	catNameProp.IsCollection = false
	catNameProp.Type = string(edm.EdmString)
	catNameProp.MappedName = binding.PropertyMappings["catName"]
	catNameProp.IsFilterable = filterProperties["catName"]
	catNameProp.IsSortable = sortableProperties["catName"]
	properties = append(properties, catNameProp)

	// catType property
	catTypeProp := new(edm.EdmProperty)
	catTypeProp.Name = "catType"
	catTypeProp.IsCollection = false
	catTypeProp.Type = string(edm.EdmString)
	catTypeProp.MappedName = binding.PropertyMappings["catType"]
	catTypeProp.IsFilterable = filterProperties["catType"]
	catTypeProp.IsSortable = sortableProperties["catType"]
	properties = append(properties, catTypeProp)

	// weight property
	weightProp := new(edm.EdmProperty)
	weightProp.Name = "weight"
	weightProp.IsCollection = false
	weightProp.Type = string(edm.EdmDouble)
	weightProp.MappedName = binding.PropertyMappings["weight"]
	weightProp.IsFilterable = filterProperties["weight"]
	weightProp.IsSortable = sortableProperties["weight"]
	properties = append(properties, weightProp)

	// age property
	ageProp := new(edm.EdmProperty)
	ageProp.Name = "age"
	ageProp.IsCollection = false
	ageProp.Type = string(edm.EdmInt64)
	ageProp.MappedName = binding.PropertyMappings["age"]
	ageProp.IsFilterable = filterProperties["age"]
	ageProp.IsSortable = sortableProperties["age"]
	properties = append(properties, ageProp)

	// extId property
	extIdProp := new(edm.EdmProperty)
	extIdProp.Name = "extId"
	extIdProp.IsCollection = false
	extIdProp.Type = string(edm.EdmString)
	extIdProp.MappedName = binding.PropertyMappings["extId"]
	extIdProp.IsFilterable = filterProperties["extId"]
	extIdProp.IsSortable = false
	properties = append(properties, extIdProp)

	// Set Entity Type
	entityType := new(edm.EdmEntityType)
	entityType.Name = "cat"
	entityType.Properties = properties

	// Add navigation properties for $expand
	var navigationProperties []*edm.EdmNavigationProperty

	// stats navigation property (Cat → CatStats)
	statsNavProp := new(edm.EdmNavigationProperty)
	statsNavProp.Name = "stats"
	statsNavProp.IsCollection = true
	statsNavProp.Type = edm.GetFullQualifiedName(edm.NamespaceEntities, "catstats")
	statsNavProp.LeftEntityKey = "extId"   // Cat.extId
	statsNavProp.RightEntityKey = "catId"  // CatStats.catId (FK)
	navigationProperties = append(navigationProperties, statsNavProp)

	// petFood navigation property (Cat → PetFood)
	petFoodNavProp := new(edm.EdmNavigationProperty)
	petFoodNavProp.Name = "petFood"
	petFoodNavProp.IsCollection = true
	petFoodNavProp.Type = edm.GetFullQualifiedName(edm.NamespaceEntities, "petfood")
	petFoodNavProp.LeftEntityKey = "extId"       // Cat.extId
	petFoodNavProp.RightEntityKey = "petfoodPetId" // PetFood.petfoodPetId (FK)
	navigationProperties = append(navigationProperties, petFoodNavProp)

	// petCare navigation property (Cat → PetCare)
	petCareNavProp := new(edm.EdmNavigationProperty)
	petCareNavProp.Name = "petCare"
	petCareNavProp.IsCollection = true
	petCareNavProp.Type = edm.GetFullQualifiedName(edm.NamespaceEntities, "petcare")
	petCareNavProp.LeftEntityKey = "extId"       // Cat.extId
	petCareNavProp.RightEntityKey = "petcarePetId" // PetCare.petcarePetId (FK)
	navigationProperties = append(navigationProperties, petCareNavProp)

	entityType.NavigationProperties = navigationProperties
	binding.EntityType = entityType

	// Set Entity Set
	entitySet := new(edm.EdmEntitySet)
	entitySet.Name = "cats"
	entitySet.EntityType = edm.GetFullQualifiedName(edm.NamespaceEntities, "cat")
	entitySet.IncludeInServiceDocument = true
	entitySet.TableName = catEntityTypeName // "cat"

	// Add navigation property bindings for $expand
	var navigationPropertyBindings []*edm.EdmNavigationPropertyBinding

	statsNavBinding := new(edm.EdmNavigationPropertyBinding)
	statsNavBinding.Path = "stats"
	statsNavBinding.Target = "catstatsSet"
	navigationPropertyBindings = append(navigationPropertyBindings, statsNavBinding)

	petFoodNavBinding := new(edm.EdmNavigationPropertyBinding)
	petFoodNavBinding.Path = "petFood"
	petFoodNavBinding.Target = "petfoodSet"
	navigationPropertyBindings = append(navigationPropertyBindings, petFoodNavBinding)

	petCareNavBinding := new(edm.EdmNavigationPropertyBinding)
	petCareNavBinding.Path = "petCare"
	petCareNavBinding.Target = "petcareSet"
	navigationPropertyBindings = append(navigationPropertyBindings, petCareNavBinding)

	entitySet.NavigationPropertyBindings = navigationPropertyBindings
	binding.EntitySet = entitySet

	return binding
}

// createCatStatsEntityBinding creates an EDM binding for the CatStats entity (Stats)
func createCatStatsEntityBinding() *edm.EdmEntityBinding {
	binding := new(edm.EdmEntityBinding)

	// Property Mappings
	binding.PropertyMappings = make(map[string]string)
	binding.PropertyMappings["catId"] = statsCatIdAttr           // "cat_id"
	binding.PropertyMappings["timestamp"] = statsTimestampAttr   // "timestamp"
	binding.PropertyMappings["heartRate"] = statsHeartRateAttr   // "heart_rate"
	binding.PropertyMappings["foodIntake"] = statsFoodIntakeAttr // "food_intake"
	binding.PropertyMappings["sleepHours"] = statsSleepHoursAttr // "sleep_hours"
	binding.PropertyMappings["weight"] = statsWeightAttr         // "weight"
	binding.PropertyMappings["age"] = statsAgeAttr               // "age"

	// Filterable properties
	filterProperties := make(map[string]bool)
	filterProperties["catId"] = true
	filterProperties["timestamp"] = true
	filterProperties["heartRate"] = true
	filterProperties["weight"] = true
	filterProperties["age"] = true

	// Sortable properties
	sortableProperties := make(map[string]bool)
	sortableProperties["timestamp"] = true
	sortableProperties["heartRate"] = true
	sortableProperties["weight"] = true

	// Create properties
	var properties []*edm.EdmProperty

	catIdProp := new(edm.EdmProperty)
	catIdProp.Name = "catId"
	catIdProp.Type = string(edm.EdmString)
	catIdProp.MappedName = binding.PropertyMappings["catId"]
	catIdProp.IsFilterable = filterProperties["catId"]
	catIdProp.IsSortable = false
	properties = append(properties, catIdProp)

	timestampProp := new(edm.EdmProperty)
	timestampProp.Name = "timestamp"
	timestampProp.Type = string(edm.EdmInt64)
	timestampProp.MappedName = binding.PropertyMappings["timestamp"]
	timestampProp.IsFilterable = filterProperties["timestamp"]
	timestampProp.IsSortable = sortableProperties["timestamp"]
	properties = append(properties, timestampProp)

	heartRateProp := new(edm.EdmProperty)
	heartRateProp.Name = "heartRate"
	heartRateProp.Type = string(edm.EdmInt64)
	heartRateProp.MappedName = binding.PropertyMappings["heartRate"]
	heartRateProp.IsFilterable = filterProperties["heartRate"]
	heartRateProp.IsSortable = sortableProperties["heartRate"]
	properties = append(properties, heartRateProp)

	foodIntakeProp := new(edm.EdmProperty)
	foodIntakeProp.Name = "foodIntake"
	foodIntakeProp.Type = string(edm.EdmDouble)
	foodIntakeProp.MappedName = binding.PropertyMappings["foodIntake"]
	foodIntakeProp.IsFilterable = false
	foodIntakeProp.IsSortable = false
	properties = append(properties, foodIntakeProp)

	sleepHoursProp := new(edm.EdmProperty)
	sleepHoursProp.Name = "sleepHours"
	sleepHoursProp.Type = string(edm.EdmDouble)
	sleepHoursProp.MappedName = binding.PropertyMappings["sleepHours"]
	sleepHoursProp.IsFilterable = false
	sleepHoursProp.IsSortable = false
	properties = append(properties, sleepHoursProp)

	weightProp := new(edm.EdmProperty)
	weightProp.Name = "weight"
	weightProp.Type = string(edm.EdmDouble)
	weightProp.MappedName = binding.PropertyMappings["weight"]
	weightProp.IsFilterable = filterProperties["weight"]
	weightProp.IsSortable = sortableProperties["weight"]
	properties = append(properties, weightProp)

	ageProp := new(edm.EdmProperty)
	ageProp.Name = "age"
	ageProp.Type = string(edm.EdmInt64)
	ageProp.MappedName = binding.PropertyMappings["age"]
	ageProp.IsFilterable = filterProperties["age"]
	ageProp.IsSortable = false
	properties = append(properties, ageProp)

	// Set Entity Type
	entityType := new(edm.EdmEntityType)
	entityType.Name = "catstats"
	entityType.Properties = properties
	binding.EntityType = entityType

	// Set Entity Set - Name must match the resource path (without leading /)
	entitySet := new(edm.EdmEntitySet)
	entitySet.Name = "catstats"  // Must match resource path /catstats
	entitySet.EntityType = edm.GetFullQualifiedName(edm.NamespaceEntities, "catstats")
	entitySet.IncludeInServiceDocument = true
	entitySet.TableName = catStatsEntityTypeName // "cat_stats"
	binding.EntitySet = entitySet

	return binding
}

// createPetFoodEntityBinding creates an EDM binding for the PetFood entity
func createPetFoodEntityBinding() *edm.EdmEntityBinding {
	binding := new(edm.EdmEntityBinding)

	// Property Mappings
	binding.PropertyMappings = make(map[string]string)
	binding.PropertyMappings["petfoodId"] = petfoodIdAttr               // "petfood_id"
	binding.PropertyMappings["petfoodName"] = petfoodNameAttr           // "petfood_name"
	binding.PropertyMappings["petfoodDescription"] = petfoodDescriptionAttr // "petfood_description"
	binding.PropertyMappings["petfoodPetId"] = petfoodPetIdAttr         // "petfood_pet_id"
	binding.PropertyMappings["petfoodPrice"] = petfoodPriceAttr         // "petfood_price"

	// Filterable properties
	filterProperties := make(map[string]bool)
	filterProperties["petfoodName"] = true
	filterProperties["petfoodPetId"] = true

	// Create properties
	var properties []*edm.EdmProperty

	petfoodIdProp := new(edm.EdmProperty)
	petfoodIdProp.Name = "petfoodId"
	petfoodIdProp.Type = string(edm.EdmInt64)
	petfoodIdProp.MappedName = binding.PropertyMappings["petfoodId"]
	petfoodIdProp.IsFilterable = false
	petfoodIdProp.IsSortable = true
	properties = append(properties, petfoodIdProp)

	petfoodNameProp := new(edm.EdmProperty)
	petfoodNameProp.Name = "petfoodName"
	petfoodNameProp.Type = string(edm.EdmString)
	petfoodNameProp.MappedName = binding.PropertyMappings["petfoodName"]
	petfoodNameProp.IsFilterable = filterProperties["petfoodName"]
	petfoodNameProp.IsSortable = true
	properties = append(properties, petfoodNameProp)

	petfoodDescProp := new(edm.EdmProperty)
	petfoodDescProp.Name = "petfoodDescription"
	petfoodDescProp.Type = string(edm.EdmString)
	petfoodDescProp.MappedName = binding.PropertyMappings["petfoodDescription"]
	petfoodDescProp.IsFilterable = false
	petfoodDescProp.IsSortable = false
	properties = append(properties, petfoodDescProp)

	petfoodPetIdProp := new(edm.EdmProperty)
	petfoodPetIdProp.Name = "petfoodPetId"
	petfoodPetIdProp.Type = string(edm.EdmString)
	petfoodPetIdProp.MappedName = binding.PropertyMappings["petfoodPetId"]
	petfoodPetIdProp.IsFilterable = filterProperties["petfoodPetId"]
	petfoodPetIdProp.IsSortable = false
	properties = append(properties, petfoodPetIdProp)

	petfoodPriceProp := new(edm.EdmProperty)
	petfoodPriceProp.Name = "petfoodPrice"
	petfoodPriceProp.Type = string(edm.EdmString)
	petfoodPriceProp.MappedName = binding.PropertyMappings["petfoodPrice"]
	petfoodPriceProp.IsFilterable = false
	petfoodPriceProp.IsSortable = false
	properties = append(properties, petfoodPriceProp)

	// Set Entity Type
	entityType := new(edm.EdmEntityType)
	entityType.Name = "petfood"
	entityType.Properties = properties
	binding.EntityType = entityType

	// Set Entity Set
	entitySet := new(edm.EdmEntitySet)
	entitySet.Name = "petfoodSet"
	entitySet.EntityType = edm.GetFullQualifiedName(edm.NamespaceEntities, "petfood")
	entitySet.IncludeInServiceDocument = true
	entitySet.TableName = petFoodEntityTypeName // "pet_food"
	binding.EntitySet = entitySet

	return binding
}

// createPetCareEntityBinding creates an EDM binding for the PetCare entity
func createPetCareEntityBinding() *edm.EdmEntityBinding {
	binding := new(edm.EdmEntityBinding)

	// Property Mappings
	binding.PropertyMappings = make(map[string]string)
	binding.PropertyMappings["petcareId"] = petcareIdAttr               // "petcare_id"
	binding.PropertyMappings["petcareName"] = petcareNameAttr           // "petcare_name"
	binding.PropertyMappings["petcareDescription"] = petcareDescriptionAttr // "petcare_description"
	binding.PropertyMappings["petcarePetId"] = petcarePetIdAttr         // "petcare_pet_id"
	binding.PropertyMappings["petcareAddress"] = petcareAddressAttr     // "petcare_address"

	// Filterable properties
	filterProperties := make(map[string]bool)
	filterProperties["petcareName"] = true
	filterProperties["petcarePetId"] = true

	// Create properties
	var properties []*edm.EdmProperty

	petcareIdProp := new(edm.EdmProperty)
	petcareIdProp.Name = "petcareId"
	petcareIdProp.Type = string(edm.EdmInt64)
	petcareIdProp.MappedName = binding.PropertyMappings["petcareId"]
	petcareIdProp.IsFilterable = false
	petcareIdProp.IsSortable = true
	properties = append(properties, petcareIdProp)

	petcareNameProp := new(edm.EdmProperty)
	petcareNameProp.Name = "petcareName"
	petcareNameProp.Type = string(edm.EdmString)
	petcareNameProp.MappedName = binding.PropertyMappings["petcareName"]
	petcareNameProp.IsFilterable = filterProperties["petcareName"]
	petcareNameProp.IsSortable = true
	properties = append(properties, petcareNameProp)

	petcareDescProp := new(edm.EdmProperty)
	petcareDescProp.Name = "petcareDescription"
	petcareDescProp.Type = string(edm.EdmString)
	petcareDescProp.MappedName = binding.PropertyMappings["petcareDescription"]
	petcareDescProp.IsFilterable = false
	petcareDescProp.IsSortable = false
	properties = append(properties, petcareDescProp)

	petcarePetIdProp := new(edm.EdmProperty)
	petcarePetIdProp.Name = "petcarePetId"
	petcarePetIdProp.Type = string(edm.EdmString)
	petcarePetIdProp.MappedName = binding.PropertyMappings["petcarePetId"]
	petcarePetIdProp.IsFilterable = filterProperties["petcarePetId"]
	petcarePetIdProp.IsSortable = false
	properties = append(properties, petcarePetIdProp)

	petcareAddressProp := new(edm.EdmProperty)
	petcareAddressProp.Name = "petcareAddress"
	petcareAddressProp.Type = string(edm.EdmString)
	petcareAddressProp.MappedName = binding.PropertyMappings["petcareAddress"]
	petcareAddressProp.IsFilterable = false
	petcareAddressProp.IsSortable = false
	properties = append(properties, petcareAddressProp)

	// Set Entity Type
	entityType := new(edm.EdmEntityType)
	entityType.Name = "petcare"
	entityType.Properties = properties
	binding.EntityType = entityType

	// Set Entity Set
	entitySet := new(edm.EdmEntitySet)
	entitySet.Name = "petcareSet"
	entitySet.EntityType = edm.GetFullQualifiedName(edm.NamespaceEntities, "petcare")
	entitySet.IncludeInServiceDocument = true
	entitySet.TableName = petCareEntityTypeName // "pet_care"
	binding.EntitySet = entitySet

	return binding
}

// GetCatEntityBindings returns all Cat-related EDM entity bindings
// Called by GetNexusEntityBindings() in odata_parser.go
func GetCatEntityBindings() []*edm.EdmEntityBinding {
	var bindings []*edm.EdmEntityBinding

	// Cat entity (Config - main entity)
	bindings = append(bindings, createCatEntityBinding())

	// CatStats entity (Stats - time-series metrics)
	bindings = append(bindings, createCatStatsEntityBinding())

	// PetFood entity (for $expand=petFood)
	bindings = append(bindings, createPetFoodEntityBinding())

	// PetCare entity (for $expand=petCare)
	bindings = append(bindings, createPetCareEntityBinding())

	return bindings
}

