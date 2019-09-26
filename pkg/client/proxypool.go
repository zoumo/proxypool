package client

import (
	"context"

	v2 "github.com/zoumo/proxypool/pkg/apis/v2"
)

type ProxyPool interface {
	GetProxy(context.Context, *v2.GetProxyRequest) (*v2.Proxy, error)
	DeleteProxy(context.Context, *v2.Proxy) error
}

type proxypool struct {
	remote v2.PoolClient
}

func NewProxyPool(c *Client) ProxyPool {
	return &proxypool{
		remote: v2.NewPoolClient(c.conn),
	}
}

func (p *proxypool) GetProxy(ctx context.Context, req *v2.GetProxyRequest) (*v2.Proxy, error) {
	return p.remote.GetProxy(ctx, req)
}

func (p *proxypool) DeleteProxy(ctx context.Context, proxy *v2.Proxy) error {
	_, err := p.remote.DeleteProxy(ctx, &v2.DeleteProxyRequest{
		Proxy: proxy,
	})
	if err != nil {
		return err
	}
	return nil
}
