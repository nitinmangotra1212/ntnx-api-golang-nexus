/*
 * IDF Utility Functions
 * Following az-manager patterns for IDF attribute handling
 */

package idf

import (
	idfIfc "github.com/nutanix-core/go-cache/insights/insights_interface"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

// AddAttribute adds an attribute to the attribute data arg list
// This converts Go types to IDF AttributeDataArg
func AddAttribute(attributeDataArgList *[]*idfIfc.AttributeDataArg, name string, value interface{}) {
	dataArg := CreateDataArg(name, value)
	if dataArg == nil {
		log.Errorf("failed to create data arg for attribute %s", name)
		return
	}
	*attributeDataArgList = append(*attributeDataArgList, dataArg)
}

// CreateDataArg creates a data arg for the given name and value based on the value type
// Supports: string, int32, int64, bool, []string, []int64, etc.
func CreateDataArg(name string, value interface{}) *idfIfc.AttributeDataArg {
	dataValue := &idfIfc.DataValue{}

	switch val := value.(type) {
	case string:
		dataValue.ValueType = &idfIfc.DataValue_StrValue{
			StrValue: val,
		}
	case int32:
		dataValue.ValueType = &idfIfc.DataValue_Int64Value{
			Int64Value: int64(val),
		}
	case int64:
		dataValue.ValueType = &idfIfc.DataValue_Int64Value{
			Int64Value: val,
		}
	case bool:
		dataValue.ValueType = &idfIfc.DataValue_BoolValue{
			BoolValue: val,
		}
	case []string:
		dataValue.ValueType = &idfIfc.DataValue_StrList_{
			StrList: &idfIfc.DataValue_StrList{
				ValueList: val,
			},
		}
	default:
		log.Errorf("Unsupported type for attribute %s: %T", name, value)
		return nil
	}

	dataArg := &idfIfc.AttributeDataArg{
		AttributeData: &idfIfc.AttributeData{
			Name:  proto.String(name),
			Value: dataValue,
		},
	}
	return dataArg
}

// ConvertEntityWithMetricToEntity converts *insights_interface.EntityWithMetric to *insights_interface.Entity
// This is needed when querying with GetEntitiesWithMetricsRet
func ConvertEntityWithMetricToEntity(entityWithMetric *idfIfc.EntityWithMetric) *idfIfc.Entity {
	entity := &idfIfc.Entity{}
	entity.EntityGuid = entityWithMetric.EntityGuid
	attributeArgList := []*idfIfc.NameTimeValuePair{}
	
	for _, metric := range entityWithMetric.MetricDataList {
		if len(metric.ValueList) != 0 {
			attributeArg := idfIfc.NameTimeValuePair{
				Name:           metric.Name,
				Value:          metric.ValueList[0].Value,
				TimestampUsecs: metric.ValueList[0].TimestampUsecs,
			}
			attributeArgList = append(attributeArgList, &attributeArg)
		}
	}
	entity.AttributeDataMap = attributeArgList
	return entity
}

// ConvertEntitiesWithMetricToEntities converts []*insights_interface.EntityWithMetric to []*insights_interface.Entity
func ConvertEntitiesWithMetricToEntities(entitiesMetric []*idfIfc.EntityWithMetric) []*idfIfc.Entity {
	var entities []*idfIfc.Entity
	for _, entityMetric := range entitiesMetric {
		entity := ConvertEntityWithMetricToEntity(entityMetric)
		entities = append(entities, entity)
	}
	return entities
}

