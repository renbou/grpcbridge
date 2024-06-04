package transcoding

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"slices"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/grpc-ecosystem/grpc-gateway/v2/utilities"
	"github.com/renbou/grpcbridge/internal/gwquery"
	"github.com/renbou/grpcbridge/internal/httperr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var (
	acceptHeader      = http.CanonicalHeaderKey("Accept")
	contentTypeHeader = http.CanonicalHeaderKey("Content-Type")
)

const (
	wildcardFieldPath = "*"
	fieldPathSep      = "."
)

var (
	_ HTTPTranscoder           = (*StandardTranscoder)(nil)
	_ HTTPRequestTranscoder    = (*standardRequestTranscoder)(nil)
	_ RequestStreamTranscoder  = (*standardRequestStreamTranscoder)(nil)
	_ HTTPResponseTranscoder   = (*standardResponseTranscoder)(nil)
	_ ResponseStreamTranscoder = (*standardResponseStreamTranscoder)(nil)
)

// StandardTranscoderOpts define all the optional settings which can be set for [StandardTranscoder].
type StandardTranscoderOpts struct {
	// Marshalers is the list of marshalers to use for transcoding.
	// If not set, the default marshaler list contains a [DefaultJSONMarshaler].
	Marshalers []Marshaler

	// Marshaler to use if the request does not specify a Content-Type header.
	// If not set, the default marshaler will be [DefaultJSONMarshaler].
	DefaultMarshaler Marshaler
}

func (o StandardTranscoderOpts) withDefaults() StandardTranscoderOpts {
	if o.Marshalers == nil {
		o.Marshalers = []Marshaler{DefaultJSONMarshaler}
	}

	if o.DefaultMarshaler == nil {
		o.DefaultMarshaler = DefaultJSONMarshaler
	}

	return o
}

// StandardTranscoder is the standard [HTTPTranscoder] used by grpcbridge,
// implemented according to the standard HTTP-to-gRPC transcoding rules, specified in [google/api/http.proto].
// Additionally to the specification, it supports binding request and response bodies to deeply-nested
// fields, not just the top-level ones.
//
// It picks the marshaler to use for transcoding in [StandardTranscoder.Bind] using the Content-Type and Accept headers
// (for more information, see the documentation for Bind), which is how gRPC-Gateway functions, too.
//
// If the picked marshaler supports streaming via [StreamMarshaler], the returned transcoders will also support streaming.
// However, unlike gRPC-Gateway, the streaming transcoders will also support binding the original request's path and query parameters,
// which allows them to be used for cases like WebSockets without having to specify all the information in each message's body.
//
// [google/api/http.proto]: https://github.com/googleapis/googleapis/blob/master/google/api/http.proto
type StandardTranscoder struct {
	defaultMarshaler Marshaler
	mimeMarshalers   map[string]Marshaler
}

// NewStandardTranscoder initializes a new [StandardTranscoder] with the specified options, which be used by it during Bind.
func NewStandardTranscoder(opts StandardTranscoderOpts) *StandardTranscoder {
	opts = opts.withDefaults()

	mimeMarshalers := make(map[string]Marshaler)
	for _, marshaler := range opts.Marshalers {
		mimeType, _ := marshaler.ContentType()
		mimeMarshalers[mimeType] = marshaler
	}

	return &StandardTranscoder{
		defaultMarshaler: opts.DefaultMarshaler,
		mimeMarshalers:   mimeMarshalers,
	}
}

// Bind constructs request and response message transcoders bound to a single specific [HTTPRequest],
// and returns a status error of StatusUnsupportedMediaType if no marshalers were found for the specified Content-Type and Accept headers.
// If the request specified a Content-Type header, it will also be used as a default for the
// response message transcoder, unless a different MIME type is specified in the Accept header.
// However, if the request specified no Content-type, a default marshaler specified in [StandardTranscoderOpts] will be used.
// The returned transcoders' ContentType methods will return the value with which they have been matched.
//
// If a chosen marshaler supports streaming via [StreamMarshaler], the according transcoder will also
// implement the [RequestStreamTranscoder] and [ResponseStreamTranscoder], which can be used to establish
// a transcoding stream if the request or response messages are to be streamed according to the method specification.
//
// Bind expects *most* of the fields of HTTPRequest to be non-nil, for example, using the type resolver in HTTPRequest.Target
// for transcoding complex message types.
func (t *StandardTranscoder) Bind(req HTTPRequest) (HTTPRequestTranscoder, HTTPResponseTranscoder, error) {
	requestMarshaler, err := t.pickRequestMarshaler(&req)
	if err != nil {
		return nil, nil, err
	}

	var isSSE bool

	responseMarshaler, ok := t.pickResponseMarshaler(&req)
	if !ok {
		responseMarshaler = requestMarshaler

		if slices.Contains(req.RawRequest.Header[acceptHeader], "text/event-stream") {
			isSSE = true
		}
	}

	if isSSE && req.Method.ClientStreaming {
		return nil, nil, status.Errorf(codes.InvalidArgument, "SSE cannot be used with client streaming methods")
	} else if isSSE && !req.Method.ServerStreaming {
		return nil, nil, status.Errorf(codes.InvalidArgument, "SSE needs to be used with server streaming methods")
	}

	bt := &boundTranscoder{req: req}
	in := newRequestTranscoder(bt, requestMarshaler)
	out := newResponseTranscoder(bt, responseMarshaler, isSSE)

	return in, out, nil
}

func (t *StandardTranscoder) pickRequestMarshaler(req *HTTPRequest) (Marshaler, error) {
	ctHeader := req.RawRequest.Header[contentTypeHeader]

	if len(ctHeader) == 0 {
		return t.defaultMarshaler, nil
	}

	for _, ct := range ctHeader {
		mt, _, err := mime.ParseMediaType(ct)
		if err != nil {
			continue
		}

		if marshaler, ok := t.mimeMarshalers[mt]; ok {
			return marshaler, nil
		}
	}

	return nil, httperr.Status(http.StatusUnsupportedMediaType, status.Errorf(codes.InvalidArgument, http.StatusText(http.StatusUnsupportedMediaType)))
}

func (t *StandardTranscoder) pickResponseMarshaler(req *HTTPRequest) (Marshaler, bool) {
	for _, mt := range req.RawRequest.Header[acceptHeader] {
		// No need to parse the Accept header, it should be in type/subtype format anyway.
		if marshaler, ok := t.mimeMarshalers[mt]; ok {
			return marshaler, true
		}
	}

	return nil, false
}

// boundTranscoder holds shared parameters for both of the bound transcoders.
type boundTranscoder struct {
	req HTTPRequest
}

// newRequestTranscoder creates a new incoming transcoder with additional streaming capabilities if the marshaler supports it.
func newRequestTranscoder(bt *boundTranscoder, marshaler Marshaler) HTTPRequestTranscoder {
	t := &standardRequestTranscoder{boundTranscoder: bt, marshaler: marshaler}

	if sm, ok := marshaler.(StreamMarshaler); ok {
		return &standardRequestStreamTranscoder{standardRequestTranscoder: t, streamer: sm}
	}

	return t
}

// standardRequestTranscoder is an incoming transcoder bound using [StandardTranscoder.Bind].
type standardRequestTranscoder struct {
	*boundTranscoder
	marshaler   Marshaler
	queryFilter *utilities.DoubleArray
}

// Transcode transcodes a new request according to the rules specified in http.proto,
// https://github.com/googleapis/googleapis/blob/bbcce1d481a148676634603794c6e697ae3b58c7/google/api/http.proto#L208.
func (t *standardRequestTranscoder) Transcode(b []byte, protomsg proto.Message) error {
	return t.transcodeFunc(false, protomsg, func(msg protoreflect.Message, fd protoreflect.FieldDescriptor) error {
		// Treat completely empty bodies as valid ones.
		if len(b) == 0 {
			return nil
		}

		return t.marshaler.Unmarshal(t.req.Target.TypeResolver, b, msg, fd)
	})
}

// transcodeFunc is a helper to support transcoding using both Unmarshal and a Decoder.
func (t *standardRequestTranscoder) transcodeFunc(supportsEOF bool, reqMsg proto.Message, f func(protoreflect.Message, protoreflect.FieldDescriptor) error) error {
	// Initially fill the request body using the specified path, if any.
	if t.req.Binding.RequestBodyPath != "" {
		msg, fd, err := traverseFieldPath(reqMsg.ProtoReflect(), t.req.Binding.RequestBodyPath)
		if err != nil {
			return status.Errorf(codes.Internal, "request body path %q: %v", t.req.Binding.RequestBodyPath, err)
		}

		if err := f(msg, fd); errors.Is(err, io.EOF) && supportsEOF {
			// EOF should only be returned during streaming, not during a single unmarshal.
			return fmt.Errorf("unmarshaling request body: %w", err)
		} else if err != nil {
			return status.Errorf(codes.InvalidArgument, "unmarshaling request body: %s", err)
		}
	}

	// Next, overwrite values using the path parameters, since they take priority over everything.
	for k, v := range t.req.PathParams {
		if err := gwquery.PopulateFieldFromPath(reqMsg, k, v); err != nil {
			return status.Errorf(codes.InvalidArgument, "type mismatch, parameter: %s, error: %s", k, err)
		}
	}

	// Finally, use the query parameters, if any should be used, to populate the rest.
	if !t.shouldParseQuery() {
		return nil
	}

	if err := runtime.PopulateQueryParameters(reqMsg, t.req.RawRequest.URL.Query(), t.queryParamFilter()); err != nil {
		return status.Errorf(codes.InvalidArgument, "parsing query parameters: %s", err)
	}

	return nil
}

func (t *standardRequestTranscoder) shouldParseQuery() bool {
	return t.req.Binding.RequestBodyPath != wildcardFieldPath
}

func (t *standardRequestTranscoder) queryParamFilter() *utilities.DoubleArray {
	if t.queryFilter != nil {
		return t.queryFilter
	}

	seqsLen := len(t.req.PathParams)
	if t.req.Binding.RequestBodyPath != "" {
		seqsLen++
	}

	seqs := make([][]string, 0, seqsLen)
	if t.req.Binding.RequestBodyPath != "" {
		seqs = append(seqs, strings.Split(t.req.Binding.RequestBodyPath, fieldPathSep))
	}

	for k := range t.req.PathParams {
		seqs = append(seqs, strings.Split(k, fieldPathSep))
	}

	t.queryFilter = utilities.NewDoubleArray(seqs)

	return t.queryFilter
}

func (t *standardRequestTranscoder) ContentType() (mime string, binary bool) {
	return t.marshaler.ContentType()
}

// newResponseTranscoder creates a new outgoing transcoder with additional streaming capabilities if the marshaler supports it.
func newResponseTranscoder(bt *boundTranscoder, marshaler Marshaler, isSSE bool) HTTPResponseTranscoder {
	t := &standardResponseTranscoder{boundTranscoder: bt, marshaler: marshaler, isSSE: isSSE}

	if sm, ok := marshaler.(StreamMarshaler); ok {
		return &standardResponseStreamTranscoder{standardResponseTranscoder: t, streamer: sm}
	}

	return t
}

// standardResponseTranscoder is an outgoing transcoder bound using [StandardTranscoder.Bind].
type standardResponseTranscoder struct {
	*boundTranscoder
	marshaler Marshaler
	isSSE     bool
}

// Transcode transcodes a new request according to the rules specified in http.proto,
// https://github.com/googleapis/googleapis/blob/bbcce1d481a148676634603794c6e697ae3b58c7/google/api/http.proto#L208.
// In contrast to the request transcoding procedure, this simply takes one of the response's fields and marshals it.
func (t *standardResponseTranscoder) Transcode(protomsg proto.Message) ([]byte, error) {
	var b []byte

	err := t.transcodeFunc(protomsg, func(m protoreflect.Message, fd protoreflect.FieldDescriptor) error {
		var err error
		b, err = t.marshaler.Marshal(t.req.Target.TypeResolver, m, fd)
		return err
	})

	return b, err
}

func (t *standardResponseTranscoder) transcodeFunc(protomsg proto.Message, f func(protoreflect.Message, protoreflect.FieldDescriptor) error) error {
	if protomsg.ProtoReflect().Descriptor().FullName() == "google.rpc.Status" {
		if err := f(protomsg.ProtoReflect(), nil); err != nil {
			return status.Errorf(codes.Internal, "marshaling response status: %s", err)
		}

		return nil
	}

	msg, fd, err := traverseFieldPath(protomsg.ProtoReflect(), t.req.Binding.ResponseBodyPath)
	if err != nil {
		return status.Errorf(codes.Internal, "response body path %q: %s", t.req.Binding.ResponseBodyPath, err)
	}

	if err := f(msg, fd); err != nil {
		return status.Errorf(codes.Internal, "marshaling response body: %s", err)
	}

	return nil
}

func (t *standardResponseTranscoder) ContentType(_ proto.Message) (mime string, binary bool) {
	return t.marshaler.ContentType()
}

// standardRequestStreamTranscoder is a wrapper around [standardRequestTranscoder] for marshalers supporting streaming.
type standardRequestStreamTranscoder struct {
	*standardRequestTranscoder
	streamer StreamMarshaler
}

func (st *standardRequestStreamTranscoder) Stream(r io.Reader) TranscodedStream {
	return &standardRequestStream{standardRequestStreamTranscoder: st, decoder: st.streamer.NewDecoder(st.req.Target.TypeResolver, r)}
}

type standardRequestStream struct {
	*standardRequestStreamTranscoder
	decoder Decoder
}

// Transcode is the streaming version of request transcoding which uses a Decoder instead of Unmarshal.
func (rs *standardRequestStream) Transcode(protomsg proto.Message) error {
	return rs.transcodeFunc(true, protomsg, rs.decoder.Decode)
}

// standardResponseStreamTranscoder is a wrapper around [standardResponseTranscoder] for marshalers supporting streaming.
type standardResponseStreamTranscoder struct {
	*standardResponseTranscoder
	streamer StreamMarshaler
}

func (st *standardResponseStreamTranscoder) Stream(w io.Writer) TranscodedStream {
	if st.standardResponseTranscoder.isSSE {
		return &sseResponseStream{standardResponseTranscoder: st.standardResponseTranscoder, w: w}
	}

	return &standardResponseStream{standardResponseStreamTranscoder: st, encoder: st.streamer.NewEncoder(st.req.Target.TypeResolver, w)}
}

type standardResponseStream struct {
	*standardResponseStreamTranscoder
	encoder Encoder
}

// Transcode is the streaming version of response transcoding which uses an Encoder instead of Marshal.
func (rs *standardResponseStream) Transcode(protomsg proto.Message) error {
	return rs.transcodeFunc(protomsg, rs.encoder.Encode)
}

type sseResponseStream struct {
	*standardResponseTranscoder
	w io.Writer
}

// Transcode implements TranscodedStream for an SSE response stream.
func (rs *sseResponseStream) Transcode(protomsg proto.Message) error {
	b, err := rs.standardResponseTranscoder.Transcode(protomsg)
	if err != nil {
		return err
	}

	_, err = rs.w.Write(slices.Concat([]byte("data:"), b, []byte("\n\n")))
	return err
}

// traverseFieldPath is a bit like the path/query parameter parsing implementation in grpc-gateway,
// but different, because we need to extract the path to be able to unmarshal into it.
// https://github.com/grpc-ecosystem/grpc-gateway/blob/8a03634212f599b1c53df57efcac1055ecc202cf/runtime/query.go#L105
func traverseFieldPath(msg protoreflect.Message, path string) (protoreflect.Message, protoreflect.FieldDescriptor, error) {
	if path == "" || path == wildcardFieldPath {
		return msg, nil, nil
	}

	root := msg

	var fd protoreflect.FieldDescriptor

	for elem, rest, foundSep := strings.Cut(path, fieldPathSep); elem != "" || foundSep; elem, rest, foundSep = strings.Cut(rest, fieldPathSep) {
		if foundSep && elem == "" {
			return nil, nil, fmt.Errorf("invalid path: %q contains empty element", path)
		}

		fields := msg.Descriptor().Fields()

		fd = fields.ByName(protoreflect.Name(elem))
		if fd == nil {
			// Don't resolve by JSON, because traverseFieldPath is used only for path & body parameters,
			// which must be named strictly like the proto fields.
			return nil, nil, fmt.Errorf("no field %q found in %s", path, root.Descriptor().Name())
		}

		// Stop on the last element before performing checks needed for next iteration.
		if rest == "" {
			break
		}

		if fd.Message() == nil || fd.Cardinality() == protoreflect.Repeated {
			return nil, nil, fmt.Errorf("invalid path: %q is not a message", elem)
		}

		msg = msg.Mutable(fd).Message()
	}

	return msg, fd, nil
}
