package app

import (
	"context"

	"github.com/davecgh/go-spew/spew"
	"github.com/zoumo/golib/queue"
	"k8s.io/klog"

	v2 "github.com/zoumo/proxypool/pkg/apis/v2"
	"github.com/zoumo/proxypool/pkg/client"
	cloudmusicvalidator "github.com/zoumo/proxypool/pkg/validator/cloudmusic"
)

type CloudMusicValidator struct {
	name      string
	queue     *queue.Queue
	client    *client.Client
	validated chan *v2.Proxy
}

func NewValidator(poolAddr string) (*CloudMusicValidator, error) {
	cfg := client.Config{
		Endpoint: poolAddr,
		Insecure: true,
	}

	client, err := client.New(cfg)
	if err != nil {
		return nil, err
	}

	v := &CloudMusicValidator{
		name:      "cloudmusic-example",
		client:    client,
		validated: make(chan *v2.Proxy, 10),
	}

	v.queue = queue.NewQueue(v.validate)

	return v, nil
}

func (v *CloudMusicValidator) RunForever() error {
	defer func() {
		klog.Infof("stopping validator %v", v.name)
		v.queue.ShutDown()
	}()
	v.queue.Run(100)

	klog.Infof("starting validator %v", v.name)
	stream := v.client.ValidateStream(context.TODO(), v.name, v.validated)
	for resp := range stream {
		klog.V(2).Infof("get response: %v", spew.Sdump(resp))
		if resp.Err != nil {
			klog.Errorf("recv error: %v", resp.Err)
			return resp.Err
		}
		if resp.Created {
			klog.Info("validate stream created")
		} else if resp.Canceled {
			klog.Info("validate stream canceled")
			return nil
		}

		if resp.ValidateRequest != nil {
			v.queue.Enqueue(resp.ValidateRequest.Addr)
		}
	}
	return nil
}

func (v *CloudMusicValidator) validate(obj interface{}) error {
	addr := obj.(string)

	err := cloudmusicvalidator.Validate(addr)
	if err != nil {
		// drop it
		return nil
	}
	// add a validated proxy to proxypool
	v.validated <- &v2.Proxy{
		Validator: v.name,
		Addr:      addr,
	}
	return nil
}
