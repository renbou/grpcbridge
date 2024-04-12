package transcoding

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/internal/bridgetest"
	"github.com/renbou/grpcbridge/internal/bridgetest/testpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

type requestValues struct {
	message     bridgedesc.Message
	bodyPath    string
	pathParams  map[string]string
	queryParams url.Values
}

func (r *requestValues) prepare() HTTPRequest {
	return HTTPRequest{
		Target: &bridgedesc.Target{
			Name:         "testsvc",
			FileResolver: testpb.TestServiceFileResolver,
			TypeResolver: testpb.TestServiceTypesResolver,
		},
		Binding:    &bridgedesc.Binding{RequestBodyPath: r.bodyPath},
		PathParams: r.pathParams,
		RawRequest: &http.Request{
			Header: http.Header{"Content-Type": []string{contentTypeJSON}},
			URL:    &url.URL{RawQuery: r.queryParams.Encode()},
		},
	}
}

type responseValues struct {
	responsePath string
}

func (r *responseValues) prepare() HTTPRequest {
	return HTTPRequest{
		Target: &bridgedesc.Target{
			Name:         "testsvc",
			FileResolver: testpb.TestServiceFileResolver,
			TypeResolver: testpb.TestServiceTypesResolver,
		},
		Binding: &bridgedesc.Binding{ResponseBodyPath: r.responsePath},
		RawRequest: &http.Request{
			Header: http.Header{"Content-Type": []string{contentTypeJSON}},
		},
	}
}

func mustBind(t *testing.T, ht HTTPTranscoder, req HTTPRequest) (HTTPRequestTranscoder, HTTPResponseTranscoder) {
	reqTranscoder, respTranscoder, err := ht.Bind(req)
	if err != nil {
		t.Fatalf("Bind() returned error = %q, expected Bind to succeed and return transcoders", err)
	}

	if reqTranscoder.ContentType() != contentTypeJSON {
		t.Fatalf("RequestTranscoder.ContentType() = %q, want %q", reqTranscoder.ContentType(), contentTypeJSON)
	}

	if respTranscoder.ContentType() != contentTypeJSON {
		t.Fatalf("ResponseTranscoder.ContentType() = %q, want %q", respTranscoder.ContentType(), contentTypeJSON)
	}

	if _, ok := reqTranscoder.(RequestStreamTranscoder); !ok {
		t.Fatalf("RequestTranscoder does not implement RequestStreamTranscoder")
	}

	if _, ok := respTranscoder.(ResponseStreamTranscoder); !ok {
		t.Fatalf("ResponseTranscoder does not implement ResponseStreamTranscoder")
	}

	return reqTranscoder, respTranscoder
}

// Test_StandardTranscoder_Request_Ok tests that the transcoding of incoming requests works as expected.
func Test_StandardTranscoder_Request_Ok(t *testing.T) {
	t.Parallel()

	// Set up a shared transcoder for all subtests.
	transcoder := NewStandardTranscoder(StandardTranscoderOpts{})

	tests := []struct {
		name    string
		request requestValues
		data    string
		wantMsg proto.Message
	}{
		{
			name: "* body",
			request: requestValues{
				message:     bridgedesc.ConcreteMessage[testpb.Scalars](),
				bodyPath:    "*",
				queryParams: url.Values{"bool_value": []string{"false"}, "string_value": []string{"another"}},
			},
			data: `{"bool_value":true, "int32Value": 12387, "string_value": "hello"}`,
			wantMsg: &testpb.Scalars{
				BoolValue:   true,
				Int32Value:  12387,
				StringValue: "hello",
			},
		},
		{
			name: "empty body",
			request: requestValues{
				message:     bridgedesc.ConcreteMessage[testpb.Scalars](),
				bodyPath:    "",
				pathParams:  map[string]string{"string_value": "hello123", "int32_value": "-23478"},
				queryParams: url.Values{"int64_value": []string{"123"}, "string_value": []string{"another"}},
			},
			data:    `{"bool_value":true, "int32Value": 12387, "string_value": "hello"}`,
			wantMsg: &testpb.Scalars{StringValue: "hello123", Int32Value: -23478, Int64Value: 123},
		},
		{
			name: "top-level field body",
			request: requestValues{
				message:  bridgedesc.ConcreteMessage[testpb.Scalars](),
				bodyPath: "bool_value",
			},
			data:    `true`,
			wantMsg: &testpb.Scalars{BoolValue: true},
		},
		{
			name: "nested field body",
			request: requestValues{
				message:  bridgedesc.ConcreteMessage[testpb.NonScalars](),
				bodyPath: "child.digits",
			},
			data:    `"ONE"`,
			wantMsg: &testpb.NonScalars{Child: &testpb.NonScalars_Child{Nested: &testpb.NonScalars_Child_Digits{Digits: testpb.Digits_ONE}}},
		},
		{
			name: "nested field body and path",
			request: requestValues{
				message:    bridgedesc.ConcreteMessage[testpb.NonScalars](),
				bodyPath:   "child.extra_scalars.bool_value",
				pathParams: map[string]string{"child.digits": "ONE"},
			},
			data: `true`,
			wantMsg: &testpb.NonScalars{Child: &testpb.NonScalars_Child{
				Nested:       &testpb.NonScalars_Child_Digits{Digits: testpb.Digits_ONE},
				ExtraScalars: &testpb.Scalars{BoolValue: true},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			reqTranscoder, _ := mustBind(t, transcoder, tt.request.prepare())

			// Act
			msg := tt.request.message.New()
			transcodeErr := reqTranscoder.Transcode([]byte(tt.data), msg)

			// Assert
			if transcodeErr != nil {
				t.Fatalf("RequestTranscoder.Transcode() returned non-nil error = %q", transcodeErr)
			}

			if diff := cmp.Diff(tt.wantMsg, msg, protoCmp()); diff != "" {
				t.Errorf("RequestTranscoder.Transcode() message differing from expected(-want+got):\n%s", diff)
			}
		})
	}
}

func Test_StandardTranscoder_Request_Error(t *testing.T) {
	t.Parallel()

	// Set up a shared transcoder for all subtests.
	transcoder := NewStandardTranscoder(StandardTranscoderOpts{})

	tests := []struct {
		name    string
		request requestValues
		data    string
		wantErr string
	}{
		{
			name: "invalid body path",
			request: requestValues{
				message:  bridgedesc.ConcreteMessage[testpb.Scalars](),
				bodyPath: "invalid",
			},
			wantErr: "no field \"invalid\" found in Scalars",
		},
		{
			name: "invalid body",
			request: requestValues{
				message:  bridgedesc.ConcreteMessage[testpb.Scalars](),
				bodyPath: "bool_value",
			},
			data:    `123`,
			wantErr: "unmarshaling request body",
		},
		{
			name: "invalid body field",
			request: requestValues{
				message:  bridgedesc.ConcreteMessage[testpb.NonScalars](),
				bodyPath: "msg_list.digits",
			},
			data:    `"ONE"`,
			wantErr: "is not a message",
		},
		{
			name: "invalid path parameter",
			request: requestValues{
				message:    bridgedesc.ConcreteMessage[testpb.Scalars](),
				bodyPath:   "bool_value",
				pathParams: map[string]string{"int32_value": "strsss"},
			},
			data:    `false`,
			wantErr: "type mismatch, parameter: int32_value",
		},
		{
			name: "invalid query parameter field",
			request: requestValues{
				message:     bridgedesc.ConcreteMessage[testpb.NonScalars](),
				queryParams: url.Values{"msg_list.digits": []string{"ONE"}},
			},
			wantErr: "is not a message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			reqTranscoder, _ := mustBind(t, transcoder, tt.request.prepare())

			// Act
			msg := tt.request.message.New()
			transcodeErr := reqTranscoder.Transcode([]byte(tt.data), msg)

			// Assert
			if transcodeErr == nil {
				t.Fatalf("RequestTranscoder.Transcode() returned nil error, expected non-nil error containing %q", tt.wantErr)
			} else if !strings.Contains(transcodeErr.Error(), tt.wantErr) {
				t.Fatalf("RequestTranscoder.Transcode() returned error = %q, want error containing %q", transcodeErr, tt.wantErr)
			}
		})
	}
}

// Test_StandardTranscoder_Response_Ok tests that the transcoding of outgoing responses works as expected.
func Test_StandardTranscoder_Response_Ok(t *testing.T) {
	t.Parallel()

	// Set up a shared transcoder for all subtests.
	transcoder := NewStandardTranscoder(StandardTranscoderOpts{})

	tests := []struct {
		name     string
		response responseValues
		msg      proto.Message
		wantData any
	}{
		{
			name:     "empty path",
			response: responseValues{responsePath: ""},
			msg:      &testpb.Scalars{BoolValue: true, Sfixed32Value: -1238494},
			wantData: map[string]any{
				"boolValue":     true,
				"int32Value":    0.0,
				"int64Value":    "0",
				"uint32Value":   0.0,
				"uint64Value":   "0",
				"sint32Value":   0.0,
				"sint64Value":   "0",
				"fixed32Value":  0.0,
				"fixed64Value":  "0",
				"sfixed32Value": -1238494.0,
				"sfixed64Value": "0",
				"floatValue":    0.0,
				"doubleValue":   0.0,
				"stringValue":   "",
				"bytesValue":    "",
			},
		},
		{
			name:     "wildcard path",
			response: responseValues{responsePath: "*"},
			msg:      &testpb.Scalars{StringValue: "hello", Int32Value: 123},
			wantData: map[string]any{
				"boolValue":     false,
				"int32Value":    123.0,
				"int64Value":    "0",
				"uint32Value":   0.0,
				"uint64Value":   "0",
				"sint32Value":   0.0,
				"sint64Value":   "0",
				"fixed32Value":  0.0,
				"fixed64Value":  "0",
				"sfixed32Value": 0.0,
				"sfixed64Value": "0",
				"floatValue":    0.0,
				"doubleValue":   0.0,
				"stringValue":   "hello",
				"bytesValue":    "",
			},
		},
		{
			name:     "top-level path",
			response: responseValues{responsePath: "string_value"},
			msg:      &testpb.Scalars{StringValue: "hello", Int32Value: 123},
			wantData: "hello",
		},
		{
			name:     "nested path",
			response: responseValues{responsePath: "child.digits"},
			msg:      &testpb.NonScalars{Child: &testpb.NonScalars_Child{Nested: &testpb.NonScalars_Child_Digits{Digits: testpb.Digits_ONE}}},
			wantData: "ONE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			_, respTranscoder := mustBind(t, transcoder, tt.response.prepare())

			// Act
			rawData, transcodeErr := respTranscoder.Transcode(tt.msg)

			if transcodeErr != nil {
				t.Fatalf("ResponseTranscoder.Transcode() returned non-nil error = %q", transcodeErr)
			}

			var data any
			if err := json.Unmarshal(rawData, &data); err != nil {
				t.Fatalf("ResponseTranscoder.Transcode() returned invalid data, failed to unmarshal: %q", err)
			}

			// Assert
			if diff := cmp.Diff(tt.wantData, data); diff != "" {
				t.Errorf("ResponseTranscoder.Transcode() data differing from expected (-want+got):\n%s", diff)
			}
		})
	}
}

// Test_StandardTranscoder_RequestStream tests the streaming version of request transcoding.
func Test_StandardTranscoder_RequestStream(t *testing.T) {
	t.Parallel()

	// Arrange
	request := requestValues{
		message:     bridgedesc.ConcreteMessage[testpb.NonScalars](),
		bodyPath:    "str_list",
		pathParams:  map[string]string{"root_digits": "ONE", "duration": "5m"},
		queryParams: url.Values{"str2str_map[key]": []string{"value"}},
	}

	data := strings.Join([]string{
		`["str1", "str2"]`,
		`[]`,
		`["single"]`,
	}, "\n")

	wantMessages := []proto.Message{
		&testpb.NonScalars{StrList: []string{"str1", "str2"}, RootDigits: testpb.Digits_ONE, Duration: durationpb.New(time.Minute * 5), Str2StrMap: map[string]string{"key": "value"}},
		&testpb.NonScalars{StrList: []string{}, RootDigits: testpb.Digits_ONE, Duration: durationpb.New(time.Minute * 5), Str2StrMap: map[string]string{"key": "value"}},
		&testpb.NonScalars{StrList: []string{"single"}, RootDigits: testpb.Digits_ONE, Duration: durationpb.New(time.Minute * 5), Str2StrMap: map[string]string{"key": "value"}},
	}

	transcoder := NewStandardTranscoder(StandardTranscoderOpts{})
	reqTranscoder, _ := mustBind(t, transcoder, request.prepare())
	stream := reqTranscoder.(RequestStreamTranscoder).Stream(strings.NewReader(data))

	// Act
	messages := make([]proto.Message, 0, len(wantMessages))

	for i := 0; i < 5; i++ {
		msg := request.message.New()

		if err := stream.Transcode(msg); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatalf("RequestStream.Transcode() returned non-nil error = %q", err)
		}

		messages = append(messages, msg)
	}

	// Assert
	if diff := cmp.Diff(wantMessages, messages, protoCmp()); diff != "" {
		t.Errorf("RequestStream.Transcode() messages differing from expected (-want+got):\n%s", diff)
	}
}

// Test_StandardTranscoder_ResponseStream tests the streaming version of response transcoding.
func Test_StandardTranscoder_ResponseStream(t *testing.T) {
	t.Parallel()

	// Arrange
	response := responseValues{responsePath: "str_list"}

	messages := []proto.Message{
		&testpb.NonScalars{StrList: []string{"str1", "str2"}, Str2StrMap: map[string]string{"key": "value"}},
		&testpb.NonScalars{StrList: []string{}, RootDigits: testpb.Digits_ONE},
		&testpb.NonScalars{StrList: []string{"single"}, Duration: durationpb.New(time.Minute * 5)},
	}

	wantData := []any{
		[]any{"str1", "str2"},
		[]any{},
		[]any{"single"},
	}

	buf := bytes.NewBuffer(nil)

	transcoder := NewStandardTranscoder(StandardTranscoderOpts{})
	_, respTranscoder := mustBind(t, transcoder, response.prepare())
	stream := respTranscoder.(ResponseStreamTranscoder).Stream(buf)

	// Act
	for _, msg := range messages {
		if err := stream.Transcode(msg); err != nil {
			t.Fatalf("ResponseStream.Transcode() returned non-nil error = %q", err)
		}
	}

	data := make([]any, 0, len(wantData))
	decoder := json.NewDecoder(buf)

	for {
		var d any
		if err := decoder.Decode(&d); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			t.Fatalf("ResponseStream.Transcode() returned invalid data, failed to unmarshal: %q", err)
		}

		data = append(data, d)
	}

	// Assert
	if diff := cmp.Diff(wantData, data); diff != "" {
		t.Errorf("ResponseStream.Transcode() returned data differing from expected (-want+got):\n%s", diff)
	}
}

// Test_StandardTranscoder_PickMarshalerFail tests that StandardMarshaler properly handles requests with unsupported content types.
func Test_StandardTranscoder_PickMarshalerFail(t *testing.T) {
	t.Parallel()

	// Arrange
	transcoder := NewStandardTranscoder(StandardTranscoderOpts{})

	// Act
	_, _, err := transcoder.Bind(HTTPRequest{RawRequest: &http.Request{
		Header: http.Header{"Content-Type": []string{"application\x00", "image/png"}},
	}})

	// Assert
	if err == nil {
		t.Errorf("Bind() returned nil error for request with invalid and unsupported content type")
	} else if cmpErr := bridgetest.StatusCodeIs(err, codes.InvalidArgument); cmpErr != nil {
		t.Errorf("Bind() returned error with unexpected gRPC status: %q", err)
	} else if httpStatus, ok := err.(interface{ HTTPStatus() int }); !ok {
		t.Errorf("Bind() returned error not implementing HTTPStatus()")
	} else if gotStatus := httpStatus.HTTPStatus(); gotStatus != http.StatusUnsupportedMediaType {
		t.Errorf("Bind() returned error with HTTP status = %q, want %q", gotStatus, http.StatusUnsupportedMediaType)
	}
}

type fakeMarshaler struct {
	JSONMarshaler
	ct []string
}

func (m *fakeMarshaler) ContentTypes() []string {
	return m.ct
}

// Test_StandardTranscoder_CustomResponseMarshaler tests that StandardTranscoder supports returning different marshalers for the request and response.
func Test_StandardTranscoder_CustomResponseMarshaler(t *testing.T) {
	t.Parallel()

	const contentTypePNG = "image/png"

	// Arrange
	transcoder := NewStandardTranscoder(StandardTranscoderOpts{
		Marshalers: []Marshaler{DefaultJSONMarshaler, &fakeMarshaler{ct: []string{contentTypePNG}}},
	})

	// Act
	reqTranscoder, respTranscoder, bindErr := transcoder.Bind(HTTPRequest{RawRequest: &http.Request{
		Header: http.Header{
			"Content-Type": []string{"application/grpc", contentTypeJSON},
			"Accept":       []string{"text/html", contentTypePNG},
		},
	}})

	// Assert
	if bindErr != nil {
		t.Fatalf("Bind() returned error = %q, expected Bind to succeed and return transcoders", bindErr)
	}

	if reqTranscoder.ContentType() != contentTypeJSON {
		t.Fatalf("RequestTranscoder.ContentType() = %q, want %q", reqTranscoder.ContentType(), contentTypeJSON)
	}

	if respTranscoder.ContentType() != contentTypePNG {
		t.Fatalf("ResponseTranscoder.ContentType() = %q, want %q", respTranscoder.ContentType(), contentTypePNG)
	}
}
