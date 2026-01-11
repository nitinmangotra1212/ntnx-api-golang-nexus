/*
 * Copyright (c) 2025 Nutanix Inc. All rights reserved.
 *
 * The underlying grpc server that exports the various services. Services may
 * be added in the registerServices() implementation.
 */

package grpc

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"sync"

	grpcmiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	pb "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config"
)

const maxStackSize = 1 << 16

// Server encapsulates the grpc server.
type Server interface {
	// Start the server. waitGroup will be used to track execution of the server.
	Start(waitGroup *sync.WaitGroup)
	Stop()
	// RegisterItemService registers the ItemService with the gRPC server
	RegisterItemService(itemService *ItemGrpcService)
	// RegisterFileService registers the FileService with the gRPC server
	RegisterFileService(fileService *FileGrpcService)
}

type ServerImpl struct {
	port     uint64
	listener net.Listener
	gserver  *grpc.Server
}

// NewServer creates a new GRPC server that services can be exported with.
// The connections are conditionally secured by mTLS. Errors are fatal.
func NewServer(port uint64) (server Server) {
	s := &ServerImpl{port: port}

	// Configure recovery options for panic handling
	recoverOpts := []grpc_recovery.Option{
		grpc_recovery.WithRecoveryHandlerContext(func(ctx context.Context, rec interface{}) (err error) {
			buf := make([]byte, maxStackSize)
			stackSize := runtime.Stack(buf, true)
			log.Errorf("gRPC panic: %v\n%s", rec, string(buf[0:stackSize]))
			return status.Errorf(codes.Internal, "Internal server error")
		}),
	}

	// Create logging interceptor for debug logs
	loggingInterceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		log.Debugf("gRPC Request: Method=%s, Request=%+v", info.FullMethod, req)
		resp, err := handler(ctx, req)
		if err != nil {
			log.Debugf("gRPC Response Error: Method=%s, Error=%v", info.FullMethod, err)
		} else {
			log.Debugf("gRPC Response: Method=%s, Response=%+v", info.FullMethod, resp)
		}
		return resp, err
	}

	streamLoggingInterceptor := func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		log.Debugf("gRPC Stream Request: Method=%s, StreamType=%v", info.FullMethod, info.IsClientStream)
		err := handler(srv, ss)
		if err != nil {
			log.Debugf("gRPC Stream Response Error: Method=%s, Error=%v", info.FullMethod, err)
		} else {
			log.Debugf("gRPC Stream Response: Method=%s", info.FullMethod)
		}
		return err
	}

	// Create gRPC server with chained interceptors
	s.gserver = grpc.NewServer(
		grpc.UnaryInterceptor(
			grpcmiddleware.ChainUnaryServer(
				loggingInterceptor,
				grpc_recovery.UnaryServerInterceptor(recoverOpts...),
			)),
		grpc.StreamInterceptor(
			grpcmiddleware.ChainStreamServer(
				streamLoggingInterceptor,
				grpc_recovery.StreamServerInterceptor(recoverOpts...),
			)),
	)

	s.registerServices()
	return s
}

// registerServices is a central place for the grpc services that need to be
// registered with the server before it is started.
func (server *ServerImpl) registerServices() {
	log.Info("Registering services with the gRPC server...")
	// ItemService will be registered with IDF repository in main.go
	// Register reflection service (for grpcurl)
	reflection.Register(server.gserver)
	log.Info("Registered reflection service")
}

// RegisterItemService registers the ItemService with the gRPC server
// This is called from main.go after IDF is initialized
func (server *ServerImpl) RegisterItemService(itemService *ItemGrpcService) {
	pb.RegisterItemServiceServer(server.gserver, itemService)
	log.Info("Registered ItemService with the gRPC server")
}

// RegisterFileService registers the FileService with the gRPC server
// This is called from main.go after file repository is initialized
func (server *ServerImpl) RegisterFileService(fileService *FileGrpcService) {
	pb.RegisterFileServiceServer(server.gserver, fileService)
	log.Info("Registered FileService with the gRPC server")
}

// Start listening and serve. Errors are fatal (todo).
func (server *ServerImpl) Start(waitGroup *sync.WaitGroup) {
	addr := ":" + strconv.FormatUint(server.port, 10)
	log.Info(fmt.Sprintf("Starting Golang Nexus gRPC server on %s.", addr))
	var err error
	server.listener, err = net.Listen("tcp4", addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v.", err)
	}
	log.Infof("Golang Nexus gRPC server listening on %s.", addr)

	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		if err := server.gserver.Serve(server.listener); err != nil {
			log.Fatalf("Failed to serve: %v.", err)
		}
	}()
}

// Stop stops the grpc server.
func (server *ServerImpl) Stop() {
	if server.gserver != nil {
		server.gserver.GracefulStop()
	}
}
