package webbridge

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/renbou/grpcbridge/internal/bridgetest/testpb"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
)

func mustTranscodedHTTPBridge(t *testing.T) (*testpb.TestService, *TranscodedHTTPBridge) {
	testsvc, router, transcoder := mustTranscodedTestSvc(t)
	bridge := NewTranscodedHTTPBridge(router, TranscodedHTTPBridgeOpts{Transcoder: transcoder})

	return testsvc, bridge
}

type brokenReader struct{}

func (br *brokenReader) Read([]byte) (int, error) {
	return 0, errors.New("broken reader")
}

// Test_TranscodedHTTPBridge_Unary tests the TranscodedHTTPBridge.ServeHTTP method for a basic unary RPC.
func Test_TranscodedHTTPBridge_Unary(t *testing.T) {
	t.Parallel()

	wantRequest := &testpb.Scalars{
		StringValue:  "stttrrr",
		Fixed64Value: 4242,
		BytesValue:   []byte("byteesssss"),
	}

	wantResponse := &testpb.Combined{
		Scalars: nil,
		NonScalars: &testpb.NonScalars{
			Str2StrMap: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
	}

	// Arrange
	testsvc, bridge := mustTranscodedHTTPBridge(t)
	testsvc.UnaryBoundResponse = testpb.PrepareResponse(wantResponse, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/service/unary/stttrrr/4242", strings.NewReader(`"`+base64.StdEncoding.EncodeToString([]byte("byteesssss"))+`"`))

	// Act
	bridge.ServeHTTP(recorder, request)

	// Assert
	if diff := cmp.Diff(wantRequest, testsvc.UnaryBoundRequest, protocmp.Transform()); diff != "" {
		t.Errorf("TestService.UnaryBound() received request differing from expected (-want+got):\n%s", diff)
	}

	if recorder.Code != http.StatusOK {
		t.Errorf("TranscodedHTTPBridge.ServeHTTP() returned unexpected non-ok response code = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	if contentType := recorder.Result().Header.Get(contentTypeHeader); contentType != ctJson {
		t.Errorf("TranscodedHTTPBridge.ServeHTTP() returned unexpected content type = %q, want %q", contentType, ctJson)
	}

	gotResponse := new(testpb.Combined)
	if err := unmarshalJSON(recorder.Body.Bytes(), gotResponse); err != nil {
		t.Fatalf("TranscodedHTTPBridge.ServeHTTP() returned invalid response, failed to unmarshal: %s", err)
	}

	if diff := cmp.Diff(wantResponse, gotResponse, protocmp.Transform()); diff != "" {
		t.Errorf("TranscodedHTTPBridge.ServeHTTP() returned response differing from expected (-want+got):\n%s", diff)
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
			httpCode:    http.StatusBadRequest,
			isMarshaled: true,
			statusCode:  codes.InvalidArgument,
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
				t.Errorf("TranscodedHTTPBridge.ServeHTTP() returned response code = %d, want %d", recorder.Code, tt.httpCode)
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
