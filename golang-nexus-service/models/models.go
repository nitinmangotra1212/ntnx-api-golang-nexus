package models

import (
	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config"
)

type QueryParams struct {
	Page    int32
	Limit   int32
	Filter  string
	Orderby string
	Select  string
	Expand  string
}

type ItemEntity struct {
	Item *pb.Item
}
