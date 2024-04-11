package webbridge

import (
	"context"
	"io"
	"net/http"

	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/routing"
	"github.com/renbou/grpcbridge/transcoding"
	"google.golang.org/protobuf/proto"
)

type HTTPTranscodedBridge struct {
	router     routing.HTTPRouter
	transcoder transcoding.HTTPTranscoder
}

func NewHTTPTranscodedBridge(router routing.HTTPRouter, transcoder transcoding.HTTPTranscoder) *HTTPTranscodedBridge {
	return &HTTPTranscodedBridge{router: router, transcoder: transcoder}
}

func (b *HTTPTranscodedBridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, route, err := b.router.RouteHTTP(r)
	if err != nil {
		writeError(w, err)
		return
	}

	reqtc, resptc, err := b.transcoder.Bind(transcoding.HTTPRequest{
		Target:     route.Target,
		Service:    route.Service,
		Method:     route.Method,
		Binding:    route.Binding,
		RawRequest: r,
		PathParams: route.PathParams,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	outgoing, err := conn.Stream(r.Context(), route.Method.RPCName)
	if err != nil {
		writeError(w, err)
		return
	}

	err = grpcadapter.ForwardServerToClient(r.Context(), grpcadapter.ForwardS2C{
		Incoming: &unaryHTTPStream{w: w, r: r, reqtc: reqtc, resptc: resptc},
		Outgoing: outgoing,
		Method:   route.Method,
	})
	if err != nil {
		writeError(w, err)
	}
}

type unaryHTTPStream struct {
	w      http.ResponseWriter
	r      *http.Request
	reqtc  transcoding.HTTPRequestTranscoder
	resptc transcoding.HTTPResponseTranscoder
	read   bool
}

func (s *unaryHTTPStream) Send(_ context.Context, msg proto.Message) error {
	b, err := s.resptc.Transcode(msg)
	if err != nil {
		return err
	}

	_, err = s.w.Write(b)
	return err
}

func (s *unaryHTTPStream) Recv(_ context.Context, msg proto.Message) error {
	if s.read {
		return io.EOF
	}

	b, err := io.ReadAll(s.r.Body)
	if err != nil {
		return err
	}

	s.read = true

	return s.reqtc.Transcode(b, msg)
}
