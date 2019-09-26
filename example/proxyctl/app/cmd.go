package app

import (
	"github.com/spf13/cobra"
	"k8s.io/klog"

	"github.com/zoumo/proxypool/example/proxyctl/app/options"
	"github.com/zoumo/proxypool/pkg/client"
)

var (
	poolClient *client.Client
)

// NewCommand returns a new Command
func NewCommand() *cobra.Command {
	s := options.New()

	cmd := &cobra.Command{
		Use:  "proxyctl",
		Long: ``,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			c, err := s.Config()
			if err != nil {
				klog.Exitln(err)
			}
			cfg := client.Config{
				Endpoint: c.Endpoint,
				Insecure: true,
			}
			poolClient, err = client.New(cfg)
			if err != nil {
				klog.Exitln(err)
			}

		},
	}

	fs := cmd.Flags()
	fs.AddFlagSet(s.Flags())

	fs.Set("logtostderr", "true")

	cmd.AddCommand(newGetCommand())

	return cmd
}
