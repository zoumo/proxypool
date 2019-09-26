package helper

import (
	"github.com/zoumo/proxypool/pkg/validator"
)

// C ...
type C struct {
	f func(queue validator.Queue)
}

// Crawl inplements spider interface
func (c *C) Crawl(queue validator.Queue) {
	c.f(queue)
}

// Spider wrap function to be Spider interface
func Spider(f func(queue validator.Queue)) *C {
	return &C{f}
}
