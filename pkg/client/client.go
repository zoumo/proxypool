package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
)

type Client struct {
	Validator
	ProxyPool

	conn   *grpc.ClientConn
	cfg    *Config
	ctx    context.Context
	cancel context.CancelFunc
}

func New(cfg Config) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("no available endpoint")
	}
	return newClient(&cfg)
}

func newClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		cfg = &Config{}
	}

	baseCtx := context.TODO()
	if cfg.Context != nil {
		baseCtx = cfg.Context
	}

	ctx, cancel := context.WithCancel(baseCtx)

	client := &Client{
		conn:   nil,
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}

	opts := make([]grpc.DialOption, 0)
	if cfg.Insecure {
		opts = append(opts, grpc.WithInsecure())
	}

	conn, err := client.dial(cfg.Endpoint, opts...)
	if err != nil {
		return nil, err
	}

	client.conn = conn
	client.Validator = NewValidator(client)
	client.ProxyPool = NewProxyPool(client)
	return client, nil
}

func (c *Client) dial(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	dctx := c.ctx
	if c.cfg.DialTimeout > 0 {
		var cancel context.CancelFunc
		dctx, cancel = context.WithTimeout(c.ctx, c.cfg.DialTimeout)
		defer cancel()
	}
	conn, err := grpc.DialContext(dctx, target, opts...)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
