package client

import (
	"context"
	"fmt"
	"sync"

	gogostatus "github.com/gogo/status"
	"github.com/zoumo/golib/retry"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"k8s.io/klog"

	v2 "github.com/zoumo/proxypool/pkg/apis/v2"
)

type Validator interface {
	ValidateStream(ctx context.Context, name string, validatedProxy <-chan *v2.Proxy) <-chan *ValidateStreamResponse
	Close() error
}

type ValidateStreamResponse struct {
	streamID        int64
	ValidateRequest *v2.Proxy
	Created         bool
	Canceled        bool
	Err             error
}

type validateStreamRequest interface {
	toPB() *v2.ValidateStreamRequest
}

type validateStreamCreateRequest struct {
	ctx       context.Context
	name      string
	validated <-chan *v2.Proxy
	retCh     chan chan *ValidateStreamResponse
}

func (r *validateStreamCreateRequest) toPB() *v2.ValidateStreamRequest {
	return &v2.ValidateStreamRequest{
		RequestUnion: &v2.ValidateStreamRequest_CreateRequest{
			CreateRequest: &v2.ValidateStreamCreateRequest{
				Name: r.name,
			},
		},
	}
}

type validateStreamRequestWrapper struct {
	cancelRequest *v2.ValidateStreamCancelRequest
	proxyResponse *v2.ProxyValidateResponse
}

func (r *validateStreamRequestWrapper) toPB() *v2.ValidateStreamRequest {
	if r.cancelRequest != nil {
		return &v2.ValidateStreamRequest{
			RequestUnion: &v2.ValidateStreamRequest_CancelRequest{
				CancelRequest: r.cancelRequest,
			},
		}
	}
	if r.proxyResponse != nil {
		return &v2.ValidateStreamRequest{
			RequestUnion: &v2.ValidateStreamRequest_ProxyResponse{
				ProxyResponse: r.proxyResponse,
			},
		}
	}
	return nil
}

type validator struct {
	ctx     context.Context
	remote  v2.ValidatorClient
	streams sync.Map // grpc stream
}

func NewValidator(c *Client) Validator {
	v := &validator{
		ctx:     c.ctx,
		remote:  v2.NewValidatorClient(c.conn),
		streams: sync.Map{},
	}

	if v.ctx == nil {
		v.ctx = context.TODO()
	}
	return v

}

func (v *validator) ValidateStream(ctx context.Context, name string, validated <-chan *v2.Proxy) <-chan *ValidateStreamResponse {
	// use validator's context to control grpc stream
	grpcStream := v.loadOrCreateGrpcStream()

	closeCh := make(chan *ValidateStreamResponse, 1)

	// createCtx := context.Background()
	createRequest := &validateStreamCreateRequest{
		ctx:       ctx,
		name:      name,
		validated: validated,
		retCh:     make(chan chan *ValidateStreamResponse, 1),
	}

	sendOk := false
	select {
	case grpcStream.reqc <- createRequest:
		sendOk = true
	case <-createRequest.ctx.Done():
		// user canceled
	case <-grpcStream.ctx.Done():
		if grpcStream.closeErr != nil {
			closeCh <- &ValidateStreamResponse{
				Canceled: true,
				Err:      grpcStream.closeErr,
			}
			break
		}
		// retry
		return v.ValidateStream(ctx, name, validated)
	}

	if sendOk {
		select {
		case ret := <-createRequest.retCh:
			// wait response
			return ret
		case <-createRequest.ctx.Done():
			// user canceled
		case <-grpcStream.ctx.Done():
			if grpcStream.closeErr != nil {
				closeCh <- &ValidateStreamResponse{
					Canceled: true,
					Err:      grpcStream.closeErr,
				}
				break
			}
			return v.ValidateStream(ctx, name, validated)
		}
	}

	close(closeCh)
	return closeCh
}

func (v *validator) Close() error {
	var err error
	v.streams.Range(func(key, value interface{}) bool {
		grpcStream := value.(*validatorGRPCStream)
		err = grpcStream.close()
		if err != nil {
			return false
		}
		return true
	})
	return err
}

func (v *validator) closeGrpcStream(gs *validatorGRPCStream) {
	gs.close()
	v.streams.Delete(gs.ctxKey)
}

func (v *validator) loadOrCreateGrpcStream() *validatorGRPCStream {
	ctxKey := streamKeyFromCtx(v.ctx)
	var grpcStream *validatorGRPCStream
	value, loaded := v.streams.Load(ctxKey)
	if loaded {
		// check firstly
		klog.V(5).Info("load grpc stream from cache")
		grpcStream = value.(*validatorGRPCStream)
	} else {
		// create grpc stream
		grpcStream = v.newValidateGrpcStream()
		actual, loaded := v.streams.LoadOrStore(ctxKey, grpcStream)
		if loaded {
			// secondary confirmation
			grpcStream.close()
			klog.V(5).Info("load grpc stream from cache")
			return actual.(*validatorGRPCStream)
		}
		klog.V(5).Info("create new grpc stream")
	}
	return grpcStream
}

func (v *validator) newValidateGrpcStream() *validatorGRPCStream {
	ctx, cancel := context.WithCancel(v.ctx)
	grpcStream := &validatorGRPCStream{
		owner:      v,
		remote:     v.remote,
		ctx:        ctx,
		ctxKey:     streamKeyFromCtx(v.ctx),
		cancel:     cancel,
		substreams: sync.Map{},
		reqc:       make(chan validateStreamRequest),
		respc:      make(chan *v2.ValidateStreamResponse),
		errc:       make(chan error, 1),
		closec:     make(chan struct{}),
	}
	go grpcStream.runLoop()
	return grpcStream
}

type validatorGRPCStream struct {
	owner  *validator
	remote v2.ValidatorClient
	ctx    context.Context
	ctxKey string
	cancel context.CancelFunc

	substreams sync.Map // validate stream

	reqc  chan validateStreamRequest
	respc chan *v2.ValidateStreamResponse

	errc chan error

	closec   chan struct{}
	closeErr error

	wg sync.WaitGroup
}

func (s *validatorGRPCStream) runLoop() {
	var client v2.Validator_ValidateStreamClient
	var closeErr error

	defer func() {
		s.closeErr = closeErr
		// close substream
		s.owner.closeGrpcStream(s)
	}()

	if client, closeErr = s.newStreamClientAndRecv(); closeErr != nil {
		return
	}

	for {
		select {
		case reqi := <-s.reqc:
			var reqpb *v2.ValidateStreamRequest
			switch req := reqi.(type) {
			case *validateStreamCreateRequest:
				name := req.name
				if _, ok := s.substreams.Load(name); ok {
					// conflict
					closec := make(chan *ValidateStreamResponse, 1)
					req.retCh <- closec
					closec <- &ValidateStreamResponse{
						Err: fmt.Errorf("validator %v is already exists", name),
					}
					close(closec)
					break
				}

				vs := &validatorGRPCSubstream{
					createReq: req,
					respc:     make(chan *v2.ValidateStreamResponse),
					outputc:   make(chan *ValidateStreamResponse, 1),
					inputc:    req.validated,
					closec:    make(chan struct{}),
				}

				s.wg.Add(1)
				go s.serveSubstream(vs)
				s.substreams.Store(name, vs)
				reqpb = req.toPB()
			default:
				reqpb = reqi.toPB()
			}

			if reqpb == nil {
				break
			}
			if reqpb.Validate() != nil {
				klog.V(2).Infof("drop invalid request: %v", reqpb)
				break
			}

			// recv request from local, send it to server
			klog.V(2).Infof("send request to server: %v", reqpb)
			client.Send(reqpb)
		case resp := <-s.respc:
			// try to dispatch response to substream
			klog.V(2).Infof("recv response: %v", resp)
			ok := s.dispatch(resp)
			if ok {
				break
			}

			switch resp.ResponseUnion.(type) {
			case *v2.ValidateStreamResponse_CancelResponse:
				// ignore cancel response here
				break
			default:
				// recv a response we can not dispatch it, send cancel request
				// to close it
				klog.Warningf("recv an unexpected response, %v", resp)
				req := &v2.ValidateStreamRequest{
					RequestUnion: &v2.ValidateStreamRequest_CancelRequest{
						CancelRequest: &v2.ValidateStreamCancelRequest{
							Name: getValidatorNameFromResponse(resp),
						},
					},
				}
				client.Send(req)
			}
		case err := <-s.errc:
			// TODO: retry once?
			closeErr = err
			return
		case <-s.ctx.Done():
			return

		}
	}

}

func (s *validatorGRPCStream) close() error {
	s.cancel()
	// wait for all substream closed
	s.wg.Wait()

	select {
	case err := <-s.errc:
		return err
	default:
		return nil
	}
}

func (s *validatorGRPCStream) closeSubstream(name string) {
	// s.retChs.Delete(name)
	substream, ok := s.substreams.LoadOrStore(name, nil)
	if !ok {
		return
	}
	s.substreams.Delete(name)
	if substream == nil {
		return
	}
	ss := substream.(*validatorGRPCSubstream)

	// send channel response in case stream was never established
	select {
	case ss.createReq.retCh <- ss.outputc:
	default:
	}

	close(ss.closec)
}

func (s *validatorGRPCStream) dispatch(resp *v2.ValidateStreamResponse) bool {
	name := getValidatorNameFromResponse(resp)
	actual, loaded := s.substreams.Load(name)
	if !loaded {
		return false
	}
	if actual == nil {
		return false
	}

	substream := actual.(*validatorGRPCSubstream)

	select {
	case substream.respc <- resp:
	case <-substream.closec:
		return false
	case <-s.ctx.Done():
		return false
	}

	return true
}

func convertResponse(resp *v2.ValidateStreamResponse) *ValidateStreamResponse {
	vsresp := &ValidateStreamResponse{
		streamID: resp.StreamID,
	}
	switch respT := resp.ResponseUnion.(type) {
	case *v2.ValidateStreamResponse_CreateResponse:
		cr := respT.CreateResponse
		vsresp.Created = cr.Created
		vsresp.Err = gogostatus.ErrorProto(cr.Status)
	case *v2.ValidateStreamResponse_CancelResponse:
		cr := respT.CancelResponse
		vsresp.Canceled = cr.Canceled
		vsresp.Err = gogostatus.ErrorProto(cr.Status)
	case *v2.ValidateStreamResponse_ProxyRequest:
		pr := respT.ProxyRequest
		vsresp.ValidateRequest = &v2.Proxy{
			Validator: pr.Validator,
			Addr:      pr.ProxyAddr,
		}
	}
	return vsresp
}

func (s *validatorGRPCStream) serveSubstream(ss *validatorGRPCSubstream) {
	defer func() {
		close(ss.outputc)
		s.wg.Done()
	}()

	validatorName := ss.createReq.name
	for {
		select {
		case resp := <-ss.respc:
			// recv response from server, and grpc stream dispatch resp to substream
			switch respT := resp.ResponseUnion.(type) {
			case *v2.ValidateStreamResponse_CreateResponse:
				cr := respT.CreateResponse
				ss.createReq.retCh <- ss.outputc
				if !cr.Created {
					// create validator error, close stream
					s.closeSubstream(validatorName)
				}
			case *v2.ValidateStreamResponse_CancelResponse:
				cr := respT.CancelResponse
				if cr.Canceled {
					s.closeSubstream(validatorName)
				}
			}
			// send to outputc
			ss.send(convertResponse(resp))
		case validated := <-ss.inputc:
			validated.Validator = validatorName
			s.send(&validateStreamRequestWrapper{
				proxyResponse: &v2.ProxyValidateResponse{
					Proxy: validated,
				},
			})

		case <-ss.closec:
			// system close
			return
		case <-ss.createReq.ctx.Done():
			// user close substream
			klog.Infof("user cancels the stream")
			// send to server
			s.send(&validateStreamRequestWrapper{
				cancelRequest: &v2.ValidateStreamCancelRequest{
					Name: validatorName,
				},
			})
			// send to outputc
			ss.send(&ValidateStreamResponse{
				Canceled: true,
			})
			// close
			s.closeSubstream(validatorName)
			return
		case <-s.ctx.Done():
			// grpc stream close
			return
		}
	}
}

func (s *validatorGRPCStream) send(req validateStreamRequest) {
	select {
	case s.reqc <- req:
	case <-s.ctx.Done():
	}
}

func (s *validatorGRPCStream) recvLoop(client v2.Validator_ValidateStreamClient) {
	for {
		resp, err := client.Recv()
		if err != nil {
			klog.Errorf("[GRPC] recv err: %v", err)

			select {
			case s.errc <- err:
			case <-s.ctx.Done():
			}
			return
		}

		if err := resp.Validate(); err != nil {
			// invalid reponse, drop it
			klog.Warningf("[GRPC] drop an invalid response: %v", err)
			break
		}

		select {
		case s.respc <- resp:
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *validatorGRPCStream) newStreamClientAndRecv() (v2.Validator_ValidateStreamClient, error) {
	client, err := s.openStreamClient()
	if err != nil {
		return nil, err
	}
	go s.recvLoop(client)
	return client, nil
}

func (s *validatorGRPCStream) openStreamClient() (client v2.Validator_ValidateStreamClient, err error) {
	retry.RetryOn(
		retry.DefaultBackoff,
		func() error {
			select {
			case <-s.ctx.Done():
				if err == nil {
					err = s.ctx.Err()
					return err
				}
			default:
			}

			client, err = s.remote.ValidateStream(s.ctx)
			if client != nil && err == nil {
				return nil
			}
			return err
		},
		func(err error) bool {
			// continue to try if err is unavailable
			return isUnavailableErr(s.ctx, err)
		},
	)
	return
}

type validatorGRPCSubstream struct {
	createReq *validateStreamCreateRequest

	respc   chan *v2.ValidateStreamResponse
	outputc chan *ValidateStreamResponse
	inputc  <-chan *v2.Proxy

	buf []*ValidateStreamResponse

	closec chan struct{}
}

func (s *validatorGRPCSubstream) send(resp *ValidateStreamResponse) {
	if resp == nil {
		return
	}

	select {
	case s.outputc <- resp:
	case <-s.closec:
	}

}

func streamKeyFromCtx(ctx context.Context) string {
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		return fmt.Sprintf("%+v", md)
	}
	return ""
}

// isHaltErr returns true if the given error and context indicate no forward
// progress can be made, even after reconnecting.
func isHaltErr(ctx context.Context, err error) bool {
	if ctx != nil && ctx.Err() != nil {
		return true
	}
	if err == nil {
		return false
	}
	ev, _ := status.FromError(err)
	// Unavailable codes mean the system will be right back.
	// (e.g., can't connect, lost leader)
	// Treat Internal codes as if something failed, leaving the
	// system in an inconsistent state, but retrying could make progress.
	// (e.g., failed in middle of send, corrupted frame)
	// TODO: are permanent Internal errors possible from grpc?
	return ev.Code() != codes.Unavailable && ev.Code() != codes.Internal
}

// isUnavailableErr returns true if the given error is an unavailable error
func isUnavailableErr(ctx context.Context, err error) bool {
	if ctx != nil && ctx.Err() != nil {
		return false
	}
	if err == nil {
		return false
	}
	ev, _ := status.FromError(err)
	// Unavailable codes mean the system will be right back.
	// (e.g., can't connect, lost leader)
	return ev.Code() == codes.Unavailable
}

func getValidatorNameFromResponse(resp *v2.ValidateStreamResponse) string {
	switch respT := resp.ResponseUnion.(type) {
	case *v2.ValidateStreamResponse_CreateResponse:
		return respT.CreateResponse.Name
	case *v2.ValidateStreamResponse_CancelResponse:
		return respT.CancelResponse.Name
	case *v2.ValidateStreamResponse_ProxyRequest:
		return respT.ProxyRequest.Validator
	}
	return ""
}
