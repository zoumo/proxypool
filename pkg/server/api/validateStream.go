package api

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"

	gogostatus "github.com/gogo/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"

	v2 "github.com/zoumo/proxypool/pkg/apis/v2"
	"github.com/zoumo/proxypool/pkg/server"
	"github.com/zoumo/proxypool/pkg/validator"
)

type validateStreamServer struct {
	validators server.Validators
	streamid   int64
}

func NewValidateStreamServer(pool *server.Server) v2.ValidatorServer {
	return &validateStreamServer{
		validators: pool.Validators(),
	}
}

func (s *validateStreamServer) ValidateStream(stream v2.Validator_ValidateStreamServer) (err error) {
	newid := atomic.AddInt64(&s.streamid, 1)

	defer func() {
		klog.Infof("[stream %v] server validator stream closed", newid)
	}()
	klog.Infof("[stream %v] server validator stream created", newid)
	svs := serverValidatorStream{
		id:               newid,
		validators:       s.validators,
		streamValidators: sync.Map{},
		grpcStream:       stream,
		closec:           make(chan struct{}),
		sendbuffer:       make(chan *v2.ValidateStreamResponse, 100),
	}

	// send loop
	svs.wg.Add(1)
	go svs.sendLoop()

	// recv loop
	errc := make(chan error, 1)
	go func() {
		if rerr := svs.recvLoop(); rerr != nil {
			if isClientCtxErr(stream.Context().Err(), rerr) {
				klog.Warningf("[stream %v] failted to receive validator stream request from grpc, %v", svs.id, rerr)
			}
			errc <- rerr
		}
	}()

	select {
	case err = <-errc:
	case <-stream.Context().Done():
		err = stream.Context().Err()
	}

	svs.close()
	return err
}

type serverValidatorStream struct {
	id               int64
	validators       server.Validators
	streamValidators sync.Map
	grpcStream       v2.Validator_ValidateStreamServer
	wg               sync.WaitGroup
	sendbuffer       chan *v2.ValidateStreamResponse
	closec           chan struct{}
}

func (svs *serverValidatorStream) close() {
	close(svs.closec)
	svs.wg.Wait()
	// delete validators attaching to the grpcStream
	svs.streamValidators.Range(func(key, value interface{}) bool {
		svs.validators.Delete(key.(string))
		return true
	})
}

func (svs *serverValidatorStream) recvLoop() error {
	for {
		req, err := svs.grpcStream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			klog.Errorf("[stream %v] recv err: %v", svs.id, err)
			return err
		}

		klog.V(5).Infof("[stream %v] recv request: %v", svs.id, req)

		if err := req.Validate(); err != nil {
			klog.Warningf("[stream %v] recv invalid request: %v", svs.id, err)
			continue
		}

		switch ru := req.RequestUnion.(type) {
		case *v2.ValidateStreamRequest_CreateRequest:
			name := ru.CreateRequest.Name
			// create validator
			validator := validator.New(name, func(addr string) {
				req := &v2.ValidateStreamResponse{
					ResponseUnion: &v2.ValidateStreamResponse_ProxyRequest{
						ProxyRequest: &v2.ProxyValidateRequest{
							Validator: name,
							ProxyAddr: addr,
						},
					},
				}
				svs.sendbuffer <- req
			})

			_, ok := svs.validators.LoadOrStore(name, validator)
			var err *gogostatus.Status
			created := true
			if ok {
				// conflict
				created = false
				err = gogostatus.New(codes.AlreadyExists, fmt.Sprintf("validator %v already exists", name))
				klog.Warning(err.Message())
			} else {
				// store validator name
				klog.Infof("[stream %v] validator %v added", svs.id, name)
				// the validator is stored in server not here, we only need name in this stream
				svs.streamValidators.Store(name, nil)
			}

			// send response
			resp := &v2.ValidateStreamResponse{
				ResponseUnion: &v2.ValidateStreamResponse_CreateResponse{
					CreateResponse: &v2.ValidateStreamCreateResponse{
						Name:    name,
						Created: created,
						Status:  err.Proto(),
					},
				},
			}

			if !svs.send(resp) {
				return nil
			}

			if !created {
				return nil
			}

		case *v2.ValidateStreamRequest_CancelRequest:
			name := ru.CancelRequest.Name

			svs.streamValidators.Delete(name)
			svs.validators.Delete(name)

			resp := &v2.ValidateStreamResponse{
				ResponseUnion: &v2.ValidateStreamResponse_CancelResponse{
					CancelResponse: &v2.ValidateStreamCancelResponse{
						Name:     name,
						Canceled: true,
					},
				},
			}
			svs.send(resp)
			return nil
		case *v2.ValidateStreamRequest_ProxyResponse:
			name := ru.ProxyResponse.Proxy.Validator
			validator, ok := svs.validators.Load(name)
			if !ok {
				// validator is gone
				break
			}
			validator.Add(ru.ProxyResponse.Proxy)
		}
	}

}

func (svs *serverValidatorStream) send(resp *v2.ValidateStreamResponse) bool {
	resp.StreamID = svs.id
	select {
	case svs.sendbuffer <- resp:
		return true
	case <-svs.closec:
	}
	return false
}

func (svs *serverValidatorStream) sendLoop() {
	defer func() {
		svs.wg.Done()
	}()
	for {
		select {
		case resp, ok := <-svs.sendbuffer:
			if !ok {
				// sendbuffer closed
				return
			}
			if err := svs.grpcStream.Send(resp); err != nil {
				klog.Errorf("[stream %v] error sending response: %v, err: %v", svs.id, resp, err)
				return
			}

			klog.V(5).Infof("[stream %v] send response successfully: %v", svs.id, resp)
		case <-svs.closec:
			return
		}
	}
}

func isClientCtxErr(ctxErr error, err error) bool {
	if ctxErr != nil {
		return true
	}

	ev, ok := status.FromError(err)
	if !ok {
		return false
	}

	switch ev.Code() {
	case codes.Canceled, codes.DeadlineExceeded:
		// client-side context cancel or deadline exceeded
		// "rpc error: code = Canceled desc = context canceled"
		// "rpc error: code = DeadlineExceeded desc = context deadline exceeded"
		return true
	case codes.Unavailable:
		msg := ev.Message()
		// client-side context cancel or deadline exceeded with TLS ("http2.errClientDisconnected")
		// "rpc error: code = Unavailable desc = client disconnected"
		if msg == "client disconnected" {
			return true
		}
		// "grpc/transport.ClientTransport.CloseStream" on canceled streams
		// "rpc error: code = Unavailable desc = stream error: stream ID 21; CANCEL")
		if strings.HasPrefix(msg, "stream error: ") && strings.HasSuffix(msg, "; CANCEL") {
			return true
		}
	}
	return false
}
