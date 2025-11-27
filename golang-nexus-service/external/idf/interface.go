package idf

import (
	"github.com/nutanix-core/go-cache/insights/insights_interface"
)

type IdfClientIfc interface {
	GetEntityRet(getArg *insights_interface.GetEntitiesArg) (*insights_interface.GetEntitiesRet, error)
	UpdateEntityRet(updateArg *insights_interface.UpdateEntityArg) (*insights_interface.UpdateEntityRet, error)
	GetEntitiesWithMetricsRet(getEntitiesWithMetricsArg *insights_interface.GetEntitiesWithMetricsArg) (*insights_interface.GetEntitiesWithMetricsRet, error)
	GetInsightsService() insights_interface.InsightsServiceInterface
}

type IdfClientImpl struct {
	IdfSvc insights_interface.InsightsServiceInterface
}

func NewIdfClient(host string, port uint16) IdfClientIfc {
	IdfService := insights_interface.NewInsightsService(host, port)
	return &IdfClientImpl{
		IdfSvc: IdfService,
	}
}
