package db

import (
	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config"
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/models"
)

type ItemRepository interface {
	CreateItem(itemEntity *models.ItemEntity) error
	GetItemById(extId string) (*models.ItemEntity, error)
	ListItems(queryParams *models.QueryParams) ([]*pb.Item, int64, error)
	UpdateItem(extId string, itemEntity *models.ItemEntity) error
	DeleteItem(extId string) error
}
