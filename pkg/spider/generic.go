package spider

import (
	"github.com/zoumo/golib/registry"
	"github.com/zoumo/proxypool/pkg/spider/ip66"
)

var (
	spider = registry.New(nil)
)

func init() {
	ip66.AddToSpider(spider)
}

func Spiders() []Spider {
	spiders := spider.Values()
	ret := make([]Spider, len(spiders))
	for i, v := range spiders {
		ret[i] = v.(Spider)
	}
	return ret
}
