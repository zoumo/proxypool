package app

import (
	"fmt"
	"net"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"

	"github.com/zoumo/proxypool/cmd/proxypool/app/config"
	"github.com/zoumo/proxypool/cmd/proxypool/app/options"
	"github.com/zoumo/proxypool/pkg/server"
	"github.com/zoumo/proxypool/pkg/server/api"
	_ "github.com/zoumo/proxypool/pkg/validator/cloudmusic" // install
	_ "github.com/zoumo/proxypool/pkg/validator/httpbin" // install
)

// NewCommand returns a new Command
func NewCommand() *cobra.Command {
	s := options.New()

	cmd := &cobra.Command{
		Use:  "proxypool",
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
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", c.Port))
	if err != nil {
		klog.Fatalf("failed to listen: %v", err)
	}
	klog.Infof("Listening on %d", c.Port)

	pool := server.New()
	pool.Run()

	s := api.Server(pool)

	return s.Serve(lis)
}
