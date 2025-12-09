module github.com/nutanix/ntnx-api-golang-nexus

go 1.24.0

require (
	github.com/google/uuid v1.6.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.4.0
	github.com/nutanix-core/go-cache v0.0.0-20251014060132-91a71b98a157
	github.com/nutanix-core/ntnx-api-odata-go v1.0.27
	github.com/nutanix-core/ntnx-api-utils-go v1.0.38
	github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/dto v0.0.0
	github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/config v0.0.0-00010101000000-000000000000
	github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/response v0.0.0-00010101000000-000000000000
	github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config v0.0.0-00010101000000-000000000000
	github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/error v0.0.0-00010101000000-000000000000
	github.com/sirupsen/logrus v1.9.3
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251111163417-95abcf5c77ba
	google.golang.org/grpc v1.77.0
	google.golang.org/protobuf v1.36.10
)

require (
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/glog v1.2.5 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.2 // indirect
	github.com/nutanix-core/go-backports/golang.org/x/crypto2 v0.0.0-20240808214654-54b39e574625 // indirect
	github.com/thoas/go-funk v0.9.3 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0 // indirect
	go.opentelemetry.io/contrib/propagators/jaeger v1.35.0 // indirect
	go.opentelemetry.io/otel v1.38.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.38.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.38.0 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	go.opentelemetry.io/proto/otlp v1.7.1 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20251022142026-3a174f9686a8 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

// Replace directives to use local generated code from ntnx-api-golang-nexus-pc
replace github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/dto => ../ntnx-api-golang-nexus-pc/generated-code/dto/src

replace github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config => ../ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config

replace github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/error => ../ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/error

replace github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/config => ../ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/config

replace github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/response => ../ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/response

// Use local clone of ntnx-api-utils-go
replace github.com/nutanix-core/ntnx-api-utils-go => ../ntnx-api-utils-go

// Fix sarama dependency issue (same as az-manager and guru)
replace github.com/Shopify/sarama => github.com/Shopify/sarama v1.17.0
