package app

import (
	"context"

	"github.com/spf13/cobra"
	"k8s.io/klog"

	v2 "github.com/zoumo/proxypool/pkg/apis/v2"
)

func newGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "get",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				klog.Exitln("no validator provided")
			}
			validator := args[0]
			proxy, err := poolClient.GetProxy(context.TODO(), &v2.GetProxyRequest{Validator: validator})
			if err != nil {
				klog.Exitln(err)
			}
			klog.Info(proxy)
		},
	}
	return cmd
}
