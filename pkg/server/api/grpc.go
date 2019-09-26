package api

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	v2 "github.com/zoumo/proxypool/pkg/apis/v2"
	"github.com/zoumo/proxypool/pkg/server"
)

// Server returns grpc server
func Server(pool *server.Server) *grpc.Server {
	s := grpc.NewServer()

	v2.RegisterValidatorServer(s, NewValidateStreamServer(pool))
	v2.RegisterPoolServer(s, NewPoolServer(pool))

	reflection.Register(s)
	return s
}
