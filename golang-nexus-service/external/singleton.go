package external

import (
	"sync"

	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/external/idf"
)

type NexusInterfaces interface {
	IdfClient() idf.IdfClientIfc
}

type singletonService struct {
	idfClient idf.IdfClientIfc
}

var (
	singleton     NexusInterfaces
	singletonOnce sync.Once
	idfClientOnce sync.Once
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

func SetSingletonServices(idfClientIfc idf.IdfClientIfc) {
	singleton = &singletonService{
		idfClient: idfClientIfc,
	}
}
