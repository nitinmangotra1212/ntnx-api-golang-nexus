/*
 * gRPC Server - REAL gRPC Implementation (like Guru!)
 * Port 50051 - Cat Service
 */

package main

import (
	"net"
	"os"

	pb "github.com/nutanix/ntnx-api-golang-mock-pc/generated-code/protobuf/mock/v4/config"
	grpcService "github.com/nutanix/ntnx-api-golang-mock/grpc"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	initLogger()

	log.Info("🚀 Starting gRPC Server (REAL gRPC - like Guru!)")
	log.Info("================================================")
	log.Info("Protocol: gRPC (HTTP/2 + Protocol Buffers)")
	log.Info("Port: 50051")
	log.Info("================================================")

	// Create TCP listener
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("❌ Failed to listen on port 50051: %v", err)
	}

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Register Cat Service
	catService := grpcService.NewCatGrpcService()
	pb.RegisterCatServiceServer(grpcServer, catService)
	log.Info("✅ Registered CatService (gRPC)")

	// Register Task Service (TODO: implement TaskGrpcService)
	// taskService := grpcService.NewTaskGrpcService()
	// pb.RegisterTaskServiceServer(grpcServer, taskService)
	// log.Info("✅ Registered TaskService (gRPC)")

	// Register reflection service (for grpcurl)
	reflection.Register(grpcServer)
	log.Info("✅ Registered reflection service")

	log.Info("")
	log.Info("📝 Available gRPC Services:")
	log.Info("  mock.v4.config.CatService")
	log.Info("    - ListCats")
	log.Info("    - GetCat")
	log.Info("    - CreateCat")
	log.Info("    - UpdateCat")
	log.Info("    - DeleteCat")
	log.Info("    - GetCatAsync")
	log.Info("")
	log.Info("🧪 Test with grpcurl:")
	log.Info("  grpcurl -plaintext localhost:50051 list")
	log.Info("  grpcurl -plaintext localhost:50051 mock.v4.config.CatService/ListCats")
	log.Info("")
	log.Infof("✅ gRPC server listening on %s", lis.Addr())
	log.Info("Press CTRL+C to stop")
	log.Info("")

	// Start serving
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("❌ Failed to serve gRPC: %v", err)
	}
}

func initLogger() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		ForceColors:     true,
	})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
}
