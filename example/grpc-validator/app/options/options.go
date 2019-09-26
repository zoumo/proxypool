package options

import (
	goflag "flag"

	"github.com/spf13/pflag"
	"k8s.io/klog"

	"github.com/zoumo/proxypool/example/grpc-validator/app/config"
)

// Options ...
type Options struct {
	// Port          int
	Endpoint string
}

// New returns new options
func New() *Options {
	return &Options{
		// Port:          18081,
		Endpoint: "127.0.0.1:18080",
	}
}

// Flags ...
func (s *Options) Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("options", pflag.ExitOnError)
	// fs.IntVar(&s.Port, "port", s.Port, "Listening to the port")
	fs.StringVar(&s.Endpoint, "endpoint", s.Endpoint, "proxy pool address")

	// init log
	gofs := goflag.NewFlagSet("klog", goflag.ExitOnError)
	klog.InitFlags(gofs)
	fs.AddGoFlagSet(gofs)

	return fs
}

// Config return a controller config objective
func (s *Options) Config() (*config.Config, error) {
	return &config.Config{
		// Port:          s.Port,
		Endpoint: s.Endpoint,
	}, nil
}
