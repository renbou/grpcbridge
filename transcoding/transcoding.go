// Package transcoding implements standard methods for transcoding between non-gRPC and gRPC request and response messages.
// At this time, standard HTTP-to-gRPC transcoding specified in [google/api/http.proto] is implemented via [StandardTranscoder]
// with support for additional improvements, such as binding request and response bodies to deeply-nested fields,
// as well as a few different marshaler implementations: [JSONMarshaler].
//
// This package's interfaces act as adapters for easier development of standard web-based gRPC bridging handlers,
// which receive an incoming HTTP request with various path and query parameters, and then receive one or more
// messages, either over the HTTP body stream, or through other means, such as WebSocket messages after an upgrade.
//
// [google/api/http.proto]: https://github.com/googleapis/googleapis/blob/master/google/api/http.proto
package transcoding

import (
	"io"
	"net/http"

	"github.com/renbou/grpcbridge/bridgedesc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Marshaler defines a converter interface for arbitrary protoreflect messages and their fields.
// Marshalers are used by the default [HTTPTranscoder] implementation, [StandardTranscoder].
//
// At the minimum, a Marshaler should support converting single messages,
// but it can additionally support encoding and decoding to/from a stream by implementing [StreamMarshaler],
// in which case it will be usable for message streams over protocols such as HTTP.
//
// A Marshaler's methods must be concurrency-safe,
// since Marshal and Unmarshal on a single marshaler instance can be used for multiple concurrent requests.
type Marshaler interface {
	Marshal(bridgedesc.TypeResolver, protoreflect.Message, protoreflect.FieldDescriptor) ([]byte, error)
	Unmarshal(bridgedesc.TypeResolver, []byte, protoreflect.Message, protoreflect.FieldDescriptor) error
	// ContentType should return the MIME content type for which this marshaler can be used,
	// and whether messages of such type are valid UTF-8 strings, or some other binary data.
	// This distinction between UTF-8/non-UTF-8 is important for protocols such as WebSocket.
	ContentType() (mime string, binary bool)
}

// StreamMarshaler defines additional methods for a [Marshaler], which, if implemented,
// will allow encoding and decoding messages to/from a stream.
// [StandardTranscoder.Bind] checks if a Marshaler implements this interface,
// after which it can decide whether to return transcoders supporting only single-message encoding,
// or the streaming versions, [RequestStreamTranscoder] and [ResponseStreamTranscoder].
type StreamMarshaler interface {
	NewEncoder(bridgedesc.TypeResolver, io.Writer) Encoder
	NewDecoder(bridgedesc.TypeResolver, io.Reader) Decoder
}

// Encoder is derived from a [StreamMarshaler] to be used for encoding messages to a stream.
// The encoder should place any additional structural information in the stream, such as a delimiter,
// so that the decoder of the same marshaler on the other side kind is able to decode the messages from the stream.
//
// An Encoder shouldn't be expected to be concurrency-safe, because it is meant to be attached to a single stream/request.
type Encoder interface {
	Encode(protoreflect.Message, protoreflect.FieldDescriptor) error
}

// Decoder is derived from a [StreamMarshaler] to be used for decoding messages from a stream.
// The decoder should handle any additional structural information in the stream placed by the encoder.
//
// A Decoder shouldn't be expected to be concurrency-safe, because it is meant to be attached to a single stream/response.
type Decoder interface {
	Decode(protoreflect.Message, protoreflect.FieldDescriptor) error
}

// HTTPRequest contains all the information an [HTTPTranscoder] could need to perform transcoding
// of the request and response messages associated with this HTTP request.
// No fields here should be nil, a transcoder might expect them all to be filled in.
type HTTPRequest struct {
	Target  *bridgedesc.Target
	Service *bridgedesc.Service
	Method  *bridgedesc.Method
	Binding *bridgedesc.Binding

	RawRequest *http.Request
	PathParams map[string]string
}

// HTTPTranscoder is the interface required to be implemented by a transcoder suitable for transcoding requests originating from HTTP.
// The only method, Bind, should return two transcoders for transcoding the request and response messages, respectively,
// or an error, if no suitable transcoders for the request are available (e.g. the request specifies an unsupported Content-Type).
// Errors returned by Bind should preferrably be gRPC status errors, but they can additionally implement
// interface { HTTPStatus() int } to return a custom HTTP status code, such as 415 (UnsupportedMediaType).
type HTTPTranscoder interface {
	Bind(HTTPRequest) (HTTPRequestTranscoder, HTTPResponseTranscoder, error)
}

// HTTPRequestTranscoder is responsible for transcoding request messages bound to a specific HTTP request.
//
// Errors returned by Transcode should be gRPC status errors to differentiate between internal errors and invalid requests
// (by default errors should be interpreted as InvalidArgument),
// and they can additionally implement interface { HTTPStatus() int } to return a custom HTTP status code.
//
// Contrary to [HTTPResponseTranscoder], ContentType doesn't accept any parameters as the it
// can be determined when a request is bound, from its headers and other metadata.
type HTTPRequestTranscoder interface {
	Transcode([]byte, proto.Message) error
	ContentType() (mime string, binary bool)
}

// HTTPResponseTranscoder is responsible for transcoding response messages bound to a specific HTTP request.
//
// Transcode should support not only the response message specified in the HTTPRequest from which this transcoder was bound,
// but also gRPC [*google.golang.org/genproto/googleapis/rpc/status.Status] messages,
// which are marshaled to return complete gRPC status codes along with the message and details.
//
// Errors returned by Transcode can be gRPC status errors to set custom status info (by default errors should be interpreted as Internal),
// and they can additionally implement interface { HTTPStatus() int } to return a custom HTTP status code.
//
// ContentType accepts a message since some responses might have dynamic content type mapping, such as [google/api/httpbody.proto],
// which can only be determined when the actual response has arrived.
// Currently, however, the default [StandardTranscoder] doesn't support this, and always returns a static content type.
//
// [google/api/httpbody.proto]: https://github.com/googleapis/googleapis/blob/master/google/api/httpbody.proto
type HTTPResponseTranscoder interface {
	Transcode(proto.Message) ([]byte, error)
	ContentType(proto.Message) (mime string, binary bool)
}

// RequestStreamTranscoder should be implemented by bound transcoders supporting receiving request messages over a binary stream,
// such as an HTTP request body.
type RequestStreamTranscoder interface {
	Stream(io.Reader) TranscodedStream
}

// ResponseStreamTranscoder should be implemented by bound transcoders supporting sending response messages over a binary stream,
// such as an HTTP response body.
type ResponseStreamTranscoder interface {
	Stream(io.Writer) TranscodedStream
}

// TranscodedStream is a stream of transcoded messages, which are being read or written to/from a stream.
// The same remarks apply to errors returned by the streaming Transcode as for [HTTPRequestTranscoder]/[HTTPResponseTranscoder],
// but, additionally, a TranscodedStream wrapping an io.Reader should return an error satisifying errors.Is(err, io.EOF)
// when the stream has ended, unless a partial read occurs and the stream ends abruptly.
// In other words, a returned io.EOF indicates successful end of stream, and other errors should be used when that is not the case.
type TranscodedStream interface {
	Transcode(proto.Message) error
}
