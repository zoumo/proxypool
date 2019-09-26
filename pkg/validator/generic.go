package validator

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zoumo/golib/queue"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	v2 "github.com/zoumo/proxypool/pkg/apis/v2"
)

const (
	defaultExpiredSeconds = 5 * 60
)

// HTTPClientWithProxy returns a new http.Client with the proxyURl
// it will desable keepalive
func HTTPClientWithProxy(proxy string) *http.Client {
	if !strings.HasPrefix(proxy, "http://") {
		proxy = "http://" + proxy
	}
	proxyURL, _ := url.Parse(proxy)
	return &http.Client{
		Transport: &http.Transport{
			Dial:              (&net.Dialer{Timeout: 500 * time.Millisecond}).Dial,
			Proxy:             http.ProxyURL(proxyURL),
			DisableKeepAlives: true,
		},
		Timeout: 1 * time.Second,
	}
}

// New ...
func New(name string, validate func(add string)) Validator {
	return newGeneric(name, validate)
}

type validator struct {
	name           string
	expiredSeconds int32
	proxyStore     workqueue.RateLimitingInterface

	workQueue *queue.Queue

	// asynchronous validate function, generally send to grpc validator service
	validate func(add string)
	stopc    chan struct{}
}

func newGeneric(name string, validate func(add string)) *validator {
	v := &validator{
		name:           name,
		validate:       validate,
		expiredSeconds: defaultExpiredSeconds,
		proxyStore:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
		stopc:          make(chan struct{}),
	}
	v.workQueue = queue.NewQueue(v.worker)
	return v
}

func (v *validator) Run() {
	klog.Infof("starting validator %v, defaultExpiredSeconds %v", v.name, v.expiredSeconds)

	v.workQueue.Run(100)

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for {
			select {
			case <-v.stopc:
				ticker.Stop()
				return
			case <-ticker.C:
				v.expire()
			}
		}
	}()
}

func (v *validator) Stop() {
	klog.Infof("stop validator %v", v.name)
	close(v.stopc)
	v.workQueue.ShutDown()
	v.proxyStore.ShutDown()
}

func (v *validator) worker(obj interface{}) error {
	addr := obj.(string)
	// async
	if v.validate == nil {
		v.Add(&v2.Proxy{Validator: v.name, Addr: addr})
	} else {
		v.validate(addr)
	}
	return nil
}

func (v *validator) Validate(addr string) {
	v.workQueue.Enqueue(addr)
}

func (v *validator) expire() {
	var num int
	defer func() {
		klog.Infof("validator %v: %v proxies are out of date, %v proxies are valid", v.name, num, v.proxyStore.Len())
	}()
	for {
		if v.proxyStore.Len() == 0 {
			return
		}
		i, quit := v.proxyStore.Get()
		if quit {
			return
		}
		p := i.(v2.Proxy)
		// mark the proxy done
		if p.Expired(p.ExpiredSeconds) {
			v.proxyStore.Done(i)
			// drop it
			num++
			continue
		}
		v.proxyStore.Done(p)
		v.proxyStore.Add(p)
		break
	}
}

func (v *validator) Get() *v2.Proxy {
	var ret *v2.Proxy
	for {
		if v.proxyStore.Len() == 0 {
			klog.Warningf("validator %v: get proxy from empty store", v.name)
			return nil
		}
		i, quit := v.proxyStore.Get()
		if quit {
			klog.Warningf("validator %v: get proxy from shutdown queue", v.name)
			return nil
		}
		p := i.(v2.Proxy)
		// mark the proxy done
		v.proxyStore.Done(i)
		if p.Expired(p.ExpiredSeconds) {
			// drop it
			continue
		}
		ret = &p
		break
	}
	// requeue
	v.proxyStore.AddAfter(*ret, 10*time.Millisecond)

	return ret
}

func (v *validator) Add(in *v2.Proxy) {
	if in == nil {
		return
	}
	// refresh create time
	in.CreateTime = time.Now().Format(time.RFC3339)
	if in.ExpiredSeconds == 0 {
		in.ExpiredSeconds = v.expiredSeconds
	}
	v.proxyStore.Add(*in)
	klog.V(4).Infof("validator %v: a valid proxy added %v, proxies count %v", v.name, in.Addr, v.proxyStore.Len())
}

func (v *validator) Delete(in *v2.Proxy) {
	if in == nil {
		return
	}
	v.proxyStore.Done(*in)
}

func (v *validator) Count() int {
	return v.proxyStore.Len()
}
