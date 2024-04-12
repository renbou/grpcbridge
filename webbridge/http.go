package webbridge

import (
	"context"
	"io"
	"net/http"

	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/routing"
	"github.com/renbou/grpcbridge/transcoding"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type HTTPTranscodedBridge struct {
	router     routing.HTTPRouter
	transcoder transcoding.HTTPTranscoder
}

func NewHTTPTranscodedBridge(router routing.HTTPRouter, transcoder transcoding.HTTPTranscoder) *HTTPTranscodedBridge {
	return &HTTPTranscodedBridge{router: router, transcoder: transcoder}
}

func (b *HTTPTranscodedBridge) ServeHTTP(unwrappedRW http.ResponseWriter, r *http.Request) {
	w := &responseWrapper{ResponseWriter: unwrappedRW}

	conn, route, err := b.router.RouteHTTP(r)
	if err != nil {
		writeError(w, r, nil, err)
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
		writeError(w, r, nil, err)
		return
	}

	// At this point all responses including errors should be transcoded to get properly parsed by a client.
	outgoing, err := conn.Stream(r.Context(), route.Method.RPCName)
	if err != nil {
		writeError(w, r, resptc, err)
		return
	}

	err = grpcadapter.ForwardServerToClient(r.Context(), grpcadapter.ForwardS2C{
		Incoming: &unaryHTTPStream{w: w, r: r, reqtc: reqtc, resptc: resptc},
		Outgoing: outgoing,
		Method:   route.Method,
	})
	if err != nil {
		writeError(w, r, resptc, err)
	}
}

type unaryHTTPStream struct {
	w      *responseWrapper
	r      *http.Request
	reqtc  transcoding.HTTPRequestTranscoder
	resptc transcoding.HTTPResponseTranscoder
	read   bool
	sent   bool
}

func (s *unaryHTTPStream) Send(_ context.Context, msg proto.Message) error {
	if s.sent {
		return status.Error(codes.Internal, "grpcbridge: tried sending second response on unary stream")
	}

	b, err := s.resptc.Transcode(msg)
	if err != nil {
		return requestTranscodingError(err)
	}

	// sent is set here, because Write can perform a partial write and still return an error
	s.sent = true

	s.w.Header()[contentTypeHeader] = []string{s.resptc.ContentType(msg)}
	_, err = s.w.Write(b)
	return err
}

func (s *unaryHTTPStream) Recv(_ context.Context, msg proto.Message) error {
	if s.read {
		return io.EOF
	}

	b, err := io.ReadAll(s.r.Body)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to read request body: %s", err)
	}

	s.read = true

	return responseTranscodingError(s.reqtc.Transcode(b, msg))
}
