package db

import (
	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/stats" // Note: stats protobuf
	"github.com/nutanix/ntnx-api-golang-nexus/golang-nexus-service/models"
)

// ItemStatsRepository defines the interface for item stats operations
type ItemStatsRepository interface {
	ListItemStats(queryParams *models.QueryParams) ([]*pb.ItemStats, int64, error)
	ListItemStatsWithGroupBy(queryParams *models.QueryParams) ([]*pb.ItemStatsGroup, int64, error) // For GroupBy - returns ItemStatsGroup
}


