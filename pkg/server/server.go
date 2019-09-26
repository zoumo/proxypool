package server

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/zoumo/proxypool/pkg/spider"
	validator "github.com/zoumo/proxypool/pkg/validator"
)

type Server struct {
	validators *validator.Validators
	spiders    []spider.Spider
	stopCh     chan struct{}
	streamid   int64
}

func New() *Server {
	pp := &Server{
		validators: validator.NewValidators(),
		spiders:    spider.Spiders(),
		stopCh:     make(chan struct{}),
	}
	return pp
}

func (s *Server) Run() {

	s.validators.Run()

	go wait.PollUntil(5*time.Second, func() (done bool, err error) {
		for _, c := range s.spiders {
			go c.Crawl(s.validators)
		}
		return false, nil
	}, s.stopCh)
}
