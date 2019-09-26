package httpbin

import (
	"fmt"
	"net/http"
	"time"

	httpstat "github.com/tcnksm/go-httpstaT"
	"k8s.io/klog"

	"github.com/zoumo/proxypool/pkg/validator"
)

const (
	httpbin  = "http://httpbin.org/ip"
	httpsbin = "https://httpbin.org/ip"
)

func init() {
	validator.RegisterValidateFunc("http", Validate)
}

// Validate check the proxy's basic connectivity
func Validate(addr string) error {
	// Create a new HTTP request
	req, _ := http.NewRequest("GET", httpbin, nil)
	// Create a httpstat powered context
	var result httpstat.Result
	// add context
	req = req.WithContext(httpstat.WithHTTPStat(req.Context(), &result))
	resp, err := validator.HTTPClientWithProxy(addr).Do(req)
	if err != nil {
		klog.V(5).Infof("Error validate proxy %v, %v", addr, err)
		return err
	}
	defer resp.Body.Close()
	ServerProcessing := int(result.ServerProcessing / time.Millisecond)
	if ServerProcessing > 200 {
		klog.V(5).Infof("drop proxy %v because it's too slow %v", addr, ServerProcessing)
		return fmt.Errorf("too slow")
	}
	klog.Infof("[httpbin] proxy %v, Server processing: %d ms", addr, ServerProcessing)
	return nil
}
