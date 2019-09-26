package api

import (
	"context"
	"fmt"

	"github.com/gogo/protobuf/types"
	"github.com/gogo/status"
	"google.golang.org/grpc/codes"

	v2 "github.com/zoumo/proxypool/pkg/apis/v2"
	"github.com/zoumo/proxypool/pkg/server"
)

type poolServer struct {
	validators server.Validators
}

func NewPoolServer(pool *server.Server) v2.PoolServer {
	return &poolServer{
		validators: pool.Validators(),
	}
}

func (s *poolServer) GetProxy(ctx context.Context, req *v2.GetProxyRequest) (*v2.Proxy, error) {
	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	validator, ok := s.validators.Load(req.Validator)
	if !ok {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("validator %v no found", req.Validator))
	}
	p := validator.Get()
	if p == nil {
		return nil, status.Error(codes.OutOfRange, fmt.Sprintf("no more proxy"))
	}
	return p, nil
}

func (s *poolServer) DeleteProxy(ctx context.Context, req *v2.DeleteProxyRequest) (*types.Empty, error) {
	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	validator, ok := s.validators.Load(req.Proxy.Validator)
	if !ok {
		// ignore validator not found
		return nil, nil
	}
	validator.Delete(req.Proxy)
	return nil, nil

}
