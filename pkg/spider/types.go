package spider

import (
	"github.com/zoumo/proxypool/pkg/validator"
)

// Spider ...
type Spider interface {
	// Crawl crawls the proxy website to
	Crawl(queue validator.Queue)
}
