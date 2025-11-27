package idf

import (
	"fmt"

	"github.com/nutanix-core/go-cache/insights/insights_interface"
	log "github.com/sirupsen/logrus"
)

func (idf *IdfClientImpl) GetEntityRet(getArg *insights_interface.GetEntitiesArg) (*insights_interface.GetEntitiesRet, error) {
	if getArg == nil {
		log.Error("Nil get argument while trying to read from IDF")
		return nil, fmt.Errorf("invalid argument")
	}
	getResponse := &insights_interface.GetEntitiesRet{}
	// Use actual IDF service method name: "GetEntities" (not "GetOperationIdf")
	err := idf.IdfSvc.SendMsgWithTimeout("GetEntities", getArg, getResponse, nil, 30)
	return getResponse, err
}

func (idf *IdfClientImpl) UpdateEntityRet(updateArg *insights_interface.UpdateEntityArg) (*insights_interface.UpdateEntityRet, error) {
	if updateArg == nil {
		log.Error("Invalid update argument")
		return nil, fmt.Errorf("invalid argument")
	}
	updateResponse := &insights_interface.UpdateEntityRet{}
	// Use actual IDF service method name: "UpdateEntity" (not "UpdateOperationIdf")
	err := idf.IdfSvc.SendMsgWithTimeout("UpdateEntity", updateArg, updateResponse, nil, 30)
	return updateResponse, err
}

func (idf *IdfClientImpl) GetEntitiesWithMetricsRet(getEntitiesWithMetricsArg *insights_interface.GetEntitiesWithMetricsArg) (*insights_interface.GetEntitiesWithMetricsRet, error) {
	if getEntitiesWithMetricsArg == nil {
		log.Error("Invalid getEntitiesWithMetrics argument")
		return nil, fmt.Errorf("invalid argument")
	}
	getResponse := &insights_interface.GetEntitiesWithMetricsRet{}
	// Use actual IDF service method name: "GetEntitiesWithMetrics" (not "GetEntitiesWithMetricsOperationIdf")
	err := idf.IdfSvc.SendMsgWithTimeout("GetEntitiesWithMetrics", getEntitiesWithMetricsArg, getResponse, nil, 30)
	return getResponse, err
}

func (idf *IdfClientImpl) GetInsightsService() insights_interface.InsightsServiceInterface {
	return idf.IdfSvc
}
