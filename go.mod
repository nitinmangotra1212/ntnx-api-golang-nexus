module github.com/nutanix/ntnx-api-golang-nexus

go 1.24.0

require (
	github.com/grpc-ecosystem/go-grpc-middleware v1.4.0
	github.com/nutanix-core/ntnx-api-utils-go v1.0.38
	github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/dto v0.0.0
	github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/config v0.0.0-00010101000000-000000000000
	github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/response v0.0.0-00010101000000-000000000000
	github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config v0.0.0-00010101000000-000000000000
	github.com/sirupsen/logrus v1.9.3
	google.golang.org/grpc v1.77.0
	google.golang.org/protobuf v1.36.10
)

require (
	github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/error v0.0.0-00010101000000-000000000000 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251111163417-95abcf5c77ba // indirect
)

// Replace directives to use local generated code from ntnx-api-golang-nexus-pc
replace github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/dto => ../ntnx-api-golang-nexus-pc/generated-code/dto/src

replace github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config => ../ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config

replace github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/error => ../ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/error

replace github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/config => ../ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/config

replace github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/response => ../ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/response

// Use local clone of ntnx-api-utils-go
replace github.com/nutanix-core/ntnx-api-utils-go => ../ntnx-api-utils-go
