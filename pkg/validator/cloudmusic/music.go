package cloudmusic

import (
	"fmt"
	"net/http"
	"time"

	httpstat "github.com/tcnksm/go-httpstaT"
	"k8s.io/klog"

	"github.com/zoumo/proxypool/pkg/validator"
)

func init() {
	validator.RegisterValidateFunc("cloudmusic", Validate)
}

// Validate validates input addr
func Validate(addr string) error {
	// Create a new HTTP request
	req, _ := http.NewRequest("GET", "http://music.163.com/klogin", nil)
	// Create a httpstat powered context
	var result httpstat.Result
	// add context
	req = req.WithContext(httpstat.WithHTTPStat(req.Context(), &result))
	resp, err := validator.HTTPClientWithProxy(addr).Do(req)
	if err != nil {
		// drop
		return err
	}
	defer resp.Body.Close()

	// TCPConnection := int(result.TCPConnection / time.Millisecond)
	ServerProcessing := int(result.ServerProcessing / time.Millisecond)
	if ServerProcessing > 100 {
		// klog.Infof("drop proxy %v because it's too slow %v", addr, ServerProcessing)
		return fmt.Errorf("proxy addr %v too slow", addr)
	}

	klog.Infof("[cloudmusic] proxy %v, Server processing: %d ms", addr, ServerProcessing)
	return nil
}
