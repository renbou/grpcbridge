// Package webbridge contains implementations of various handlers for bridging web-originated requests to gRPC.
// It allows using gRPC-only services through all kinds of interfaces supporting all the possible streaming variants.
//
// The available functionality can be separated into two different kinds by the API format:
//   - Typical REST-like API implemented using classic single-request-single-response and streamed HTTP requests,
//     streaming WebSocket connections, and Server-Sent Events, all coming with support for request path parameters,
//     query parameters, and custom body path specification.
//   - Modern gRPC-Web API supporting both unary and streaming RPCs for HTTP and WebSocket requests,
//     with WebTransport support planned, too.
package webbridge

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/renbou/grpcbridge/transcoding"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	contentTypeHeader        = http.CanonicalHeaderKey("Content-Type")
	contentTypeOptionsHeader = http.CanonicalHeaderKey("X-Content-Type-Options")
)

const httpStatusCanceled = 499

// responseWrapper wraps a ResponseWriter to avoid error handling when the response has already been partially written.
type responseWrapper struct {
	http.ResponseWriter
	writtenStatus bool
}

func (w *responseWrapper) WriteHeader(statusCode int) {
	w.writtenStatus = true
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWrapper) Write(data []byte) (int, error) {
	w.writtenStatus = true
	return w.ResponseWriter.Write(data)
}

func errorStatus(err error) (*status.Status, int) {
	st := status.Convert(err)

	respStatus := runtime.HTTPStatusFromCode(st.Code())
	if s, ok := err.(interface{ HTTPStatus() int }); ok {
		respStatus = s.HTTPStatus()
	}

	return st, respStatus
}

func writeTextError(w *responseWrapper, err error) {
	st, respStatus := errorStatus(err)
	http.Error(w, st.Message(), respStatus)
}

func transcodeError(w *responseWrapper, t transcoding.HTTPResponseTranscoder, err error) {
	st, respStatus := errorStatus(err)
	stProto := st.Proto()

	respData, transcodeErr := t.Transcode(stProto)
	if transcodeErr == nil {
		ct, _ := t.ContentType(stProto)
		w.Header()[contentTypeHeader] = []string{ct}
	} else {
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "unable to transcode response status code = %s desc = %s: %s\n", st.Code(), st.Message(), transcodeErr)

		// copied from http.Error function, seems safe
		w.Header()[contentTypeHeader] = []string{"text/plain; charset=utf-8"}
		w.Header()[contentTypeOptionsHeader] = []string{"nosniff"}
	}

	w.WriteHeader(respStatus)
	w.Write(respData)
}

func writeError(w *responseWrapper, r *http.Request, t transcoding.HTTPResponseTranscoder, err error) {
	if w.writtenStatus {
		// No point in writing an error when the response has already been partially received, it would be meaningless
		// (wrong status, wrong content-type, unexpected mixing of different message formats, etc).
		return
	}

	if requestCanceled(r) {
		// Avoid wasting more resources if client request is not waiting.
		w.WriteHeader(httpStatusCanceled)
		return
	}

	if t == nil {
		writeTextError(w, err)
		return
	}

	transcodeError(w, t, err)
}

func requestCanceled(r *http.Request) bool {
	select {
	case <-r.Context().Done():
		return true
	default:
		return false
	}
}

// requestTranscodingError should be used to convert non-status errors received from a request transcoder.
func requestTranscodingError(err error) error {
	return wrapTranscodingError(err, codes.InvalidArgument)
}

// responseTranscodingError should be used to convert non-status errors received from a response transcoder.
func responseTranscodingError(err error) error {
	return wrapTranscodingError(err, codes.Internal)
}

func wrapTranscodingError(err error, defaultCode codes.Code) error {
	type grpcstatus interface{ GRPCStatus() *status.Status }

	// Manual type check to only use status errors coming from the actual transcoder, not wrapped ones.
	if err == nil {
		return nil
	} else if _, ok := err.(grpcstatus); ok {
		return err
	}

	return status.Error(defaultCode, err.Error())
}
