package ip66

import (
	"fmt"
	"strings"

	"github.com/gocolly/colly"
	"github.com/zoumo/golib/registry"

	"github.com/zoumo/proxypool/pkg/spider/helper"
	"github.com/zoumo/proxypool/pkg/validator"
)

// AddToSpider ...
func AddToSpider(c registry.Registry) {
	c.Register("ip66", helper.Spider(crawl))
}

func crawl(queue validator.Queue) {
	c := colly.NewCollector()
	c.OnResponse(func(e *colly.Response) {
		resp := string(e.Body)
		ret := strings.Split(resp, "\n")
		for _, line := range ret {
			if !strings.Contains(line, "<br />") {
				continue
			}
			line = strings.TrimSpace(line)
			line = strings.TrimRight(line, "<br />")
			line = "http://" + line
			// got new proxy addr
			queue.Enqueue(line)
		}
	})
	for i := 2; i < 5; i++ {
		// zj, _ := Utf8ToGbk("浙江")
		// zj = url.QueryEscape(zj)
		url := fmt.Sprintf("http://www.66ip.cn/nmtq.php?getnum=30&isp=0&anonymoustype=%d&start=&ports=&export=&ipaddress=&area=1&proxytype=0&api=66ip", i)
		c.Visit(url)
	}
}
