package options

import (
	goflag "flag"

	"github.com/spf13/pflag"
	"k8s.io/klog"

	"github.com/zoumo/proxypool/cmd/proxypool/app/config"
)

// Options ...
type Options struct {
	Port int
}

// New returns new options
func New() *Options {
	return &Options{
		Port: 18080,
	}
}

// Flags ...
func (s *Options) Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("options", pflag.ExitOnError)
	fs.IntVar(&s.Port, "port", s.Port, "Listening to the port")

	// init log
	gofs := goflag.NewFlagSet("klog", goflag.ExitOnError)
	klog.InitFlags(gofs)
	fs.AddGoFlagSet(gofs)

	return fs
}

// Config return a controller config objective
func (s *Options) Config() (*config.Config, error) {
	return &config.Config{
		Port: s.Port,
	}, nil
}
