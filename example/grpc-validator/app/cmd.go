package app

import (
	"github.com/zoumo/proxypool/example/grpc-validator/app/options"
	"github.com/zoumo/proxypool/example/grpc-validator/app/config"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"

)

// NewCommand returns a new Command
func NewCommand() *cobra.Command {
	s := options.New()

	cmd := &cobra.Command{
		Use:  "cloudmusic",
		Long: ``,
		Run: func(cmd *cobra.Command, args []string) {
			c, err := s.Config()
			if err != nil {
				klog.Exitln(err)
			}
			if err := Run(c, wait.NeverStop); err != nil {
				klog.Exitln(err)
			}
		},
	}

	fs := cmd.Flags()
	fs.AddFlagSet(s.Flags())

	fs.Set("logtostderr", "true")

	return cmd
}

// Run runs the Config.  This should never exit.
func Run(c *config.Config, stopCh <-chan struct{}) error {
	validator, err := NewValidator(c.Endpoint)
	if err != nil {
		return err
	}

	if err := validator.RunForever(); err != nil {
		return err
	}
	return nil
}
