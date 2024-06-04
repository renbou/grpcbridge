package transcoding

import (
	"net/url"
	"testing"

	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/internal/bridgetest/testpb"
)

var keyCases = []string{
	"str2str_map",
	"child.extra_scalars.bool_value",
	"a.b.c.",
	"str_list[0]",
	"duration",
	"*",
}

// FuzzTranscoder_PathKey fuzzes the transcoder with various path parameter key values.
func FuzzTranscoder_PathKey(f *testing.F) {
	for _, cc := range keyCases {
		f.Add(cc)
	}

	f.Fuzz(func(t *testing.T, path string) {
		transcoder := NewStandardTranscoder(StandardTranscoderOpts{})
		request := requestValues{
			message:     bridgedesc.ConcreteMessage[testpb.NonScalars](),
			bodyPath:    "*",
			pathParams:  map[string]string{path: "value"},
			queryParams: url.Values{},
		}

		reqTranscoder, _ := mustBind(t, transcoder, request.prepare())
		_ = reqTranscoder.Transcode(nil, request.message.New())
	})
}

// FuzzTranscoder_QueryKey fuzzes the transcoder with various query parameter key values.
func FuzzTranscoder_QueryKey(f *testing.F) {
	for _, cc := range keyCases {
		f.Add(cc)
	}

	f.Fuzz(func(t *testing.T, query string) {
		transcoder := NewStandardTranscoder(StandardTranscoderOpts{})
		request := requestValues{
			message:     bridgedesc.ConcreteMessage[testpb.NonScalars](),
			bodyPath:    "*",
			pathParams:  map[string]string{},
			queryParams: url.Values{query: []string{"value"}},
		}

		reqTranscoder, _ := mustBind(t, transcoder, request.prepare())
		_ = reqTranscoder.Transcode(nil, request.message.New())
	})
}

// FuzzTranscoder_BodyPath fuzzes the transcoder with various body path values.
func FuzzTranscoder_BodyPath(f *testing.F) {
	for _, cc := range keyCases {
		f.Add(cc)
	}

	f.Fuzz(func(t *testing.T, bodyPath string) {
		transcoder := NewStandardTranscoder(StandardTranscoderOpts{})
		request := requestValues{
			message:     bridgedesc.ConcreteMessage[testpb.NonScalars](),
			bodyPath:    bodyPath,
			pathParams:  map[string]string{},
			queryParams: url.Values{},
		}

		reqTranscoder, _ := mustBind(t, transcoder, request.prepare())
		_ = reqTranscoder.Transcode([]byte(`{"value": "value"}`), request.message.New())
	})
}

// Test_TranscoderFuzzCases runs various cases found by fuzzing which have previously caused the transcoder to panic.
func Test_TranscoderFuzzCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		values requestValues
		data   []byte
	}{
		{
			// traverseFieldPath contained a bug due to which the child field would be selected as the message with this invalid path,
			// and then msg.Get() would panic in the protobuf lib since "child" does not have the field "child".
			name: "invalid nested body path",
			values: requestValues{
				message:     bridgedesc.ConcreteMessage[testpb.NonScalars](),
				bodyPath:    "child..",
				pathParams:  map[string]string{},
				queryParams: url.Values{},
			},
			data: []byte(`{"value": "value"}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			transcoder := NewStandardTranscoder(StandardTranscoderOpts{})
			reqTranscoder, _ := mustBind(t, transcoder, tt.values.prepare())
			_ = reqTranscoder.Transcode(tt.data, tt.values.message.New())
		})
	}
}
