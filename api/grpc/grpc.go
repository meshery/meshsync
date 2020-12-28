package grpc

import (
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/layer5io/meshsync/internal/proto"
	"github.com/layer5io/meshsync/meshsync"
)

// Service object holds all the information about the server parameters.
type Service struct {
	Name      string    `json:"name"`
	Port      string    `json:"port"`
	Version   string    `json:"version"`
	StartedAt time.Time `json:"string,startedat"`

	Handler *meshsync.Handler
	Channel chan interface{}
}

// panicHandler is the handler function to handle panic errors.
func panicHandler(r interface{}) error {
	fmt.Println("600 Error")
	return ErrPanic(r)
}

// Start starts grpc server.
func Start(s *Service) error {
	address := fmt.Sprintf(":%s", s.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return ErrGrpcListener(err)
	}

	middlewares := middleware.ChainUnaryServer(
		grpc_recovery.UnaryServerInterceptor(
			grpc_recovery.WithRecoveryHandler(panicHandler),
		),
	)

	server := grpc.NewServer(
		grpc.UnaryInterceptor(middlewares),
	)
	reflection.Register(server)

	//Register Proto
	proto.RegisterMeshsyncServer(server, s)

	// Start serving requests
	if err = server.Serve(listener); err != nil {
		return ErrGrpcServer(err)
	}
	return nil
}
