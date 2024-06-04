package webbridge

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/internal/bridgetest/testpb"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func mustTranscodedHTTPBridge(t *testing.T) (*testpb.TestService, *TranscodedHTTPBridge) {
	testsvc, router, transcoder := mustTranscodedTestSvc(t)
	bridge := NewTranscodedHTTPBridge(router, TranscodedHTTPBridgeOpts{
		Transcoder: transcoder,
		Forwarder: grpcadapter.NewProxyForwarder(grpcadapter.ProxyForwarderOpts{
			Filter: grpcadapter.NewProxyMDFilter(grpcadapter.ProxyMDFilterOpts{
				AllowRequestMD: []string{testpb.FlowMetadataKey},
			}),
		}),
	})

	return testsvc, bridge
}

type brokenReader struct{}

func (br *brokenReader) Read([]byte) (int, error) {
	return 0, errors.New("broken reader")
}

// Test_TranscodedHTTPBridge_Unary tests the TranscodedHTTPBridge.ServeHTTP method for a basic unary RPC.
func Test_TranscodedHTTPBridge_Unary(t *testing.T) {
	t.Parallel()

	baseRequest := func() *http.Request {
		return httptest.NewRequest("POST", "/service/unary/stttrrr/4242", strings.NewReader(`"`+base64.StdEncoding.EncodeToString([]byte("byteesssss"))+`"`))
	}
	baseResponse := testpb.PrepareResponse(&testpb.Combined{
		Scalars: nil,
		NonScalars: &testpb.NonScalars{
			Str2StrMap: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
	}, nil)
	baseWantRequest := &testpb.Scalars{
		StringValue:  "stttrrr",
		Fixed64Value: 4242,
		BytesValue:   []byte("byteesssss"),
	}

	tests := []struct {
		name         string
		request      *http.Request
		response     *testpb.PreparedResponse[*testpb.Combined]
		wantRequest  *testpb.Scalars
		wantResponse proto.Message
		wantStatus   int
	}{
		{
			name:         "basic",
			request:      baseRequest(),
			response:     baseResponse,
			wantRequest:  baseWantRequest,
			wantResponse: baseResponse.Response,
			wantStatus:   http.StatusOK,
		},
		{
			name:         "error",
			request:      baseRequest(),
			response:     testpb.PrepareResponse[*testpb.Combined](nil, status.Errorf(codes.AlreadyExists, "already exists")),
			wantRequest:  baseWantRequest,
			wantResponse: &spb.Status{Code: int32(codes.AlreadyExists), Message: "already exists"},
			wantStatus:   http.StatusConflict,
		},
		{
			name: "timeout",
			request: func() *http.Request {
				r := baseRequest()
				r.Header.Set("Grpc-Timeout", "1n") // 1 nanosecond will surely time out
				return r
			}(),
			response:     baseResponse,
			wantRequest:  nil,
			wantResponse: &spb.Status{Code: int32(codes.DeadlineExceeded), Message: "context deadline exceeded"},
			wantStatus:   http.StatusGatewayTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			testsvc, bridge := mustTranscodedHTTPBridge(t)
			testsvc.UnaryBoundResponse = tt.response
			recorder := httptest.NewRecorder()

			// Act
			bridge.ServeHTTP(recorder, tt.request)

			// Assert
			// wantRequest is nil if it isn't meant to arrive to the actual gRPC server.
			if tt.wantRequest != nil {
				if diff := cmp.Diff(tt.wantRequest, testsvc.UnaryBoundRequest, protocmp.Transform()); diff != "" {
					t.Errorf("TestService.UnaryBound() received request differing from expected (-want+got):\n%s", diff)
				}
			}

			if recorder.Code != tt.wantStatus {
				t.Errorf("TranscodedHTTPBridge.ServeHTTP() returned unexpected status code = %d, want %d", recorder.Code, tt.wantStatus)
			}

			if contentType := recorder.Result().Header.Get(contentTypeHeader); contentType != ctJson {
				t.Errorf("TranscodedHTTPBridge.ServeHTTP() returned unexpected content type = %q, want %q", contentType, ctJson)
			}

			gotResponse := tt.wantResponse.ProtoReflect().New().Interface()
			if err := unmarshalJSON(recorder.Body.Bytes(), gotResponse); err != nil {
				t.Fatalf("TranscodedHTTPBridge.ServeHTTP() returned invalid response, failed to unmarshal: %s", err)
			}

			if diff := cmp.Diff(tt.wantResponse, gotResponse, protocmp.Transform()); diff != "" {
				t.Errorf("TranscodedHTTPBridge.ServeHTTP() returned response differing from expected (-want+got):\n%s", diff)
			}
		})
	}
}

// Test_TranscodedHTTPBridge_Errors tests how TranscodedHTTPBridge.ServeHTTP handles and returns errors with various codes and formats.
func Test_TranscodedHTTPBridge_Errors(t *testing.T) {
	t.Parallel()

	_, bridge := mustTranscodedHTTPBridge(t)

	tests := []struct {
		name        string
		request     *http.Request
		httpCode    int
		isMarshaled bool
		statusCode  codes.Code
	}{
		{
			name:        "404",
			request:     httptest.NewRequest("GET", "/notaroute", nil),
			httpCode:    http.StatusNotFound,
			isMarshaled: false,
		},
		{
			name: "unsupported content-type",
			request: func() *http.Request {
				req := httptest.NewRequest("POST", "/grpcbridge.internal.bridgetest.testpb.TestService/UnaryUnbound", nil)
				req.Header.Set(contentTypeHeader, "img/png")
				return req
			}(),
			httpCode:    http.StatusUnsupportedMediaType,
			isMarshaled: false,
		},
		{
			name: "canceled request",
			request: func() *http.Request {
				req := httptest.NewRequest("POST", "/grpcbridge.internal.bridgetest.testpb.TestService/UnaryUnbound", nil)
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				return req.WithContext(ctx)
			}(),
			httpCode:    httpStatusCanceled,
			isMarshaled: false,
		},
		{
			name:        "unreadable request",
			request:     httptest.NewRequest("POST", "/grpcbridge.internal.bridgetest.testpb.TestService/UnaryUnbound", new(brokenReader)),
			httpCode:    http.StatusServiceUnavailable,
			isMarshaled: true,
			statusCode:  codes.Unavailable,
		},
		{
			name:        "invalid request",
			request:     httptest.NewRequest("POST", "/service/unary/stttrrr/4242", strings.NewReader(`bad`)),
			httpCode:    http.StatusBadRequest,
			isMarshaled: true,
			statusCode:  codes.InvalidArgument,
		},
		{
			name:        "failed response marshal",
			request:     httptest.NewRequest("POST", "/service/bad-response-path", nil),
			httpCode:    http.StatusInternalServerError,
			isMarshaled: true,
			statusCode:  codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			recorder := httptest.NewRecorder()

			// Act
			bridge.ServeHTTP(recorder, tt.request)

			// Assert
			if recorder.Code != tt.httpCode {
				t.Errorf("TranscodedHTTPBridge.ServeHTTP() returned status code = %d, want %d", recorder.Code, tt.httpCode)
			}

			if !tt.isMarshaled {
				return
			}

			gotStatusPB := new(spb.Status)
			if err := unmarshalJSON(recorder.Body.Bytes(), gotStatusPB); err != nil {
				t.Fatalf("TranscodedHTTPBridge.ServeHTTP() returned invalid status response, failed to unmarshal: %s", err)
			}

			if gotStatus := status.FromProto(gotStatusPB); gotStatus.Code() != tt.statusCode {
				t.Errorf("TranscodedHTTPBridge.ServeHTTP() returned status code = %s, want %s", gotStatus.Code(), tt.statusCode)
			}
		})
	}
}

func Test_SSE(t *testing.T) {
	t.Parallel()

	wantStream := "data:{\"message\":\"hello\"}\n\ndata:{\"message\":\"world\"}\n\n"

	// Arrange
	testsvc, bridge := mustTranscodedHTTPBridge(t)

	flowID := testsvc.AddFlow([]*testpb.FlowAction{
		{Action: &testpb.FlowAction_SendMessage{SendMessage: &testpb.FlowMessage{Message: "hello"}}},
		{Action: &testpb.FlowAction_SendMessage{SendMessage: &testpb.FlowMessage{Message: "world"}}},
	})

	server := httptest.NewServer(bridge)
	t.Cleanup(server.Close)

	// Act
	req, err := http.NewRequest("GET", server.URL+"/flow/server", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() returned non-nil error = %q", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set(testpb.FlowMetadataKey, flowID)

	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /flow/server returned non-nil error = %q", err)
	}

	// Assert
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("io.ReadAll() returned non-nil error = %q", err)
	}

	if diff := cmp.Diff(wantStream, string(respBody)); diff != "" {
		t.Errorf("GET /flow/server returned unexpected body (-want+got):\n%s", diff)
	}
}
