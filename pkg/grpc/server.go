package grpc

import (
	"context"
	"github.com/cukhoaimon/SimpleBank/internal/delivery/grpc/gapi"
	"github.com/cukhoaimon/SimpleBank/internal/delivery/grpc/pb"
	db "github.com/cukhoaimon/SimpleBank/internal/usecase/sqlc"
	token2 "github.com/cukhoaimon/SimpleBank/pkg/token"
	"github.com/cukhoaimon/SimpleBank/utils"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
	"log"
	"net"
	"net/http"
)

// Server serves gRPC request
type Server struct {
	Handler *gapi.Handler
}

// NewServer will return new gRPC server
func NewServer(store db.Store, config utils.Config) (*Server, error) {
	maker, err := token2.NewPasetoMaker(config.TokenSymmetricKey)
	if err != nil {
		return nil, err
	}

	handler := &gapi.Handler{
		Store:      store,
		TokenMaker: maker,
		Config:     config,
	}

	return &Server{Handler: handler}, nil
}

// Run will run gRPC server with provided store and config
func Run(store db.Store, config utils.Config) {
	server, err := NewServer(store, config)
	if err != nil {
		log.Fatalf("Cannot create gRPC server: %s", err)
	}

	gRPCServer := grpc.NewServer()
	pb.RegisterSimpleBankServer(gRPCServer, server.Handler)
	// allow client to know what RPCs currently available in server
	reflection.Register(gRPCServer)

	listener, err := net.Listen("tcp", config.GrpcServerAddress)
	if err != nil {
		log.Fatalf("Cannot create tcp-listener for gRPC server: %s", err)
	}

	log.Printf("start gRPC server at: %s", listener.Addr().String())
	if err = gRPCServer.Serve(listener); err != nil {
		log.Fatalf("Cannot serve gRPC server: %s", err)
	}
}

// RunGatewayServer will run gRPC Gateway with provided store and config
// to serve HTTP Request
func RunGatewayServer(store db.Store, config utils.Config) {
	server, err := NewServer(store, config)
	if err != nil {
		log.Fatalf("Cannot create gRPC server: %s", err)
	}

	jsonOpts := runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
		MarshalOptions: protojson.MarshalOptions{
			UseProtoNames: true,
		},
		UnmarshalOptions: protojson.UnmarshalOptions{
			DiscardUnknown: true,
		},
	})

	grpcMux := runtime.NewServeMux(jsonOpts)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err = pb.RegisterSimpleBankHandlerServer(ctx, grpcMux, server.Handler); err != nil {
		log.Fatalf("Cannot register handler server: %s", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", grpcMux)

	fs := http.FileServer(http.Dir("./docs/swagger"))
	mux.Handle("/swagger/", http.StripPrefix("/swagger/", fs))

	listener, err := net.Listen("tcp", config.HttpServerAddress)
	if err != nil {
		log.Fatalf("Cannot create tcp-listener for gateway server: %s", err)
	}

	log.Printf("start HTTP gateway server at: %s", listener.Addr().String())
	if err = http.Serve(listener, mux); err != nil {
		log.Fatalf("cannot HTTP gateway server: %s", err)
	}
}
