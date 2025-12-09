package external

import (
	"sync"

	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/constants"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/external/idf"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/external/statsgw"
)

type NexusInterfaces interface {
	IdfClient() idf.IdfClientIfc
	StatsGWClient() statsgw.StatsGWClientIfc
}

type singletonService struct {
	idfClient     idf.IdfClientIfc
	statsGWClient statsgw.StatsGWClientIfc
}

var (
	singleton         NexusInterfaces
	singletonOnce     sync.Once
	idfClientOnce     sync.Once
	statsGWClientOnce sync.Once
)

func Interfaces() NexusInterfaces {
	singletonOnce.Do(func() {
		if singleton == nil {
			singleton = &singletonService{}
		}
	})
	return singleton
}

func (s *singletonService) IdfClient() idf.IdfClientIfc {
	idfClientOnce.Do(func() {
		if s.idfClient == nil {
			// Default to localhost:2027 (IDF standard port)
			client := idf.NewIdfClient("127.0.0.1", 2027)
			s.idfClient = client
		}
	})
	return s.idfClient
}

func (s *singletonService) StatsGWClient() statsgw.StatsGWClientIfc {
	statsGWClientOnce.Do(func() {
		if s.statsGWClient == nil {
			// Default to localhost:8084 (statsGW standard port)
			client, err := statsgw.NewStatsGWClient(constants.StatsGWHost, constants.StatsGWPort)
			if err != nil {
				// Log error but don't fail - client will be nil and can be retried
				return
			}
			s.statsGWClient = client
		}
	})
	return s.statsGWClient
}

func SetSingletonServices(idfClientIfc idf.IdfClientIfc, statsGWClientIfc statsgw.StatsGWClientIfc) {
	singleton = &singletonService{
		idfClient:     idfClientIfc,
		statsGWClient: statsGWClientIfc,
	}
	// Reset once flags to allow re-initialization
	singletonOnce = sync.Once{}
	idfClientOnce = sync.Once{}
	statsGWClientOnce = sync.Once{}
}
