package grpcadapter

import (
	"context"
	"errors"
	"io"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/renbou/grpcbridge/internal/rpcutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ProxyForwarderOpts define all the optional settings which can be set for [ProxyForwarder].
type ProxyForwarderOpts struct {
	// Filter specifies the metadata filter to be used for all operations.
	// If not set, the default [ProxyMDFilter] is created via [NewProxyMDFilter] and used.
	Filter MetadataFilter
}

func (o ProxyForwarderOpts) withDefaults() ProxyForwarderOpts {
	if o.Filter == nil {
		o.Filter = NewProxyMDFilter(ProxyMDFilterOpts{})
	}
	return o
}

// ProxyForwarder is the default implementation of [Forwarder] used by grpcbridge,
// suitable for use in a proxy scenario where request/response metadata must be filtered
// and various timeouts must be set to ensure the proxy continues working with misbehaving clients.
//
// It doesn't modify the messages received from the incoming and outgoing streams, proxying them as-is.
// Metadata, on the other hand, is filtered using the specified [MetadataFilter].
// If the filtered metadata returned by the filter's FilterRequestMD method contains a "grpc-timeout" field,
// ProxyForwarder attempts to parse it as specified in gRPC's [PROTOCOL-HTTP2] spec
// and use it as an additional timeout for the whole streaming call.
//
// [PROTOCOL-HTTP2]: https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md
type ProxyForwarder struct {
	filter MetadataFilter
}

// NewProxyForwarder creates a new [ProxyForwarder] using the specified options.
func NewProxyForwarder(opts ProxyForwarderOpts) *ProxyForwarder {
	opts = opts.withDefaults()

	return &ProxyForwarder{
		filter: opts.Filter,
	}
}

// forwardError is a utility struct to distinguish errors received from server.Recv/Send and client.Recv/Send calls.
// This is mostly needed for properly handling io.EOF errors which might come from all sorts of calls.
type forwardError struct {
	incomingErr error
	outgoingErr error
}

// Forward forwards an incoming [ServerStream] to an outgoing [ClientConn] after initiating a [ClientStream] using the specified params.
// Metadata is retrieved using [metadata.FromIncomingContext] and filtered using the [MetadataFilter] using which this forwarder was configured.
// It either returns an error when the forwarding process fails, or nil when the outgoing stream returns an io.EOF on Recv.
//
// Forward waits for its forwarding goroutines to exit, which means that both the ServerStream and the created ClientStream
// MUST support context for their Recv/Send operations, otherwise there exist execution flows which might never end
// if a Send/Recv on the incoming or outgoing stream don't return properly.
// Due to such cases existing where Forward will block indefinitely, it waits for its goroutines to exit in ALL cases.
func (pf *ProxyForwarder) Forward(ctx context.Context, params ForwardParams) error {
	// Request metadata to be forwarded in the initial call.
	md, _ := metadata.FromIncomingContext(ctx)
	md = pf.filter.FilterRequestMD(md)
	ctx = metadata.NewOutgoingContext(ctx, md)

	// wg.Wait deferred before cancel() and Close() to ensure that all potential resources have
	// been notified to stop and return, so that wg.Wait can end.
	var wg sync.WaitGroup
	defer wg.Wait()

	// This context is used as the base context for the whole forwarding process, including the forwarding goroutines.
	// However, we don't wait for them to actually exit, because ServerStream operations can still block until the server handler returns.
	// TODO(renbou): support timeout here for unary calls.
	ctx, cancel := pf.baseContext(ctx, md)
	defer cancel()

	i2oErrCh := make(chan forwardError, 1)
	o2iErrCh := make(chan forwardError, 1)

	var streamErr error
	var outgoing ClientStream

	// Launch the client-to-server forwarder only when the method expects a client stream,
	// otherwise its better to actually read the incoming request before interacting with the server,
	// to make sure we aren't wasting any resources on something that is guaranteed to fail.
	if params.Method.ClientStreaming {
		outgoing, streamErr = pf.stream(ctx, &params)
		if streamErr != nil {
			return streamErr
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			i2oErrCh <- pf.forwardIncomingToOutgoing(ctx, &params, outgoing)
		}()
	} else {
		outgoing, streamErr = pf.forwardUnaryRequest(ctx, &params)
		if streamErr != nil {
			// outgoing guaranteed to be nil and closed by sendUnaryRequest()
			return streamErr
		}
	}

	defer outgoing.Close()

	// Server-to-client forwarder must always be run, because the server might respond at any time,
	// not only after the client stream has finished, e.g. close the stream due to an authentication error.
	wg.Add(1)
	go func() {
		defer wg.Done()
		o2iErrCh <- pf.forwardOutgoingToIncoming(ctx, &params, outgoing)
	}()

	// Handle proxying errors and return them when needed.
	// Nothing here will deadlock thanks to the buffer of the channels,
	// and the outgoing channel is guaranteed to receive an error due to the context cancelation.
	for {
		select {
		case <-ctx.Done():
			// forcefully stop everything once we get a deadline or the initial client request context is canceled (i.e. client closed the request/connection).
			return rpcutil.ContextError(ctx.Err())
		case perr := <-i2oErrCh:
			if errors.Is(perr.incomingErr, io.EOF) || errors.Is(perr.outgoingErr, io.EOF) {
				// io.EOF can arrive either from incoming.Recv or outgoing.Send,
				// and in both cases we need to receive further responses via outgoing.Recv until it returns a proper error.
				// For this to work properly, forward a CloseSend to outgoing, even though io.EOF from outgoing.Send might've already done so.
				// Calling it here is safe since outgoing.Send isn't going to get called anymore anyway.
				outgoing.CloseSend()
			} else if perr.incomingErr != nil {
				return perr.incomingErr
			} else {
				// perr.outgoingErr is guaranteed to be non-nil by forwardIncomingToOutgoing.
				return perr.outgoingErr
			}
		case perr := <-o2iErrCh:
			if perr.incomingErr != nil {
				return perr.incomingErr
			}

			// At this point, perr.outgoingErr is guaranteed by to contain the correct error.
			return perr.outgoingErr
		}
	}
}

func (pf *ProxyForwarder) baseContext(ctx context.Context, md metadata.MD) (context.Context, context.CancelFunc) {
	if v := md.Get("grpc-timeout"); len(v) > 0 {
		if d, ok := decodeTimeout(v[0]); ok {
			ctx, cancel := context.WithTimeout(ctx, d)
			return ctx, cancel
		}
	}

	return context.WithCancel(ctx)
}

func (pf *ProxyForwarder) stream(ctx context.Context, params *ForwardParams) (ClientStream, error) {
	// TODO(renbou): support stream initiation timeout.
	outgoing, err := params.Outgoing.Stream(ctx, params.Method.RPCName)
	if err != nil {
		return nil, err
	}

	return outgoing, nil
}

// forwardUnaryRequest is separated from forwardIncomingToOutgoing to be able to optimize stream initiation
// for it to occur only after receiving the request for unary calls.
func (pf *ProxyForwarder) forwardUnaryRequest(ctx context.Context, params *ForwardParams) (s ClientStream, e error) {
	msg := params.Method.Input.New()

	if err := params.Incoming.Recv(ctx, msg); errors.Is(err, io.EOF) {
		// io.EOF from a client must be treated as an error in the case of a unary request.
		return nil, status.Errorf(codes.Unavailable, "grpcbridge: unexpected EOF from client for unary request: %s", err)
	} else if err != nil {
		return nil, err
	}

	// NB: Must be closed by forwardUnaryRequest() if an error occurs.
	outgoing, err := pf.stream(ctx, params)
	if err != nil {
		return nil, err
	}

	// io.EOF skipped to continue the process in ForwardServerToClient,
	// the actual status will be returned by forwardOutgoingToIncoming on Recv().
	if err := outgoing.Send(ctx, msg); err != nil && !errors.Is(err, io.EOF) {
		outgoing.Close()
		return nil, err // we only care about the case where err == io.EOF, nil and other errors are handled normally.
	}

	outgoing.CloseSend() // just in case specify that no more messages will come.
	return outgoing, nil
}

func (pf *ProxyForwarder) forwardIncomingToOutgoing(ctx context.Context, params *ForwardParams, outgoing ClientStream) forwardError {
	for {
		// It is not safe to reuse a Proto message after Outgoing.Send().
		msg := params.Method.Input.New()

		if err := params.Incoming.Recv(ctx, msg); err != nil {
			return forwardError{incomingErr: err}
		}

		if err := outgoing.Send(ctx, msg); err != nil {
			return forwardError{outgoingErr: err}
		}
	}
}

func (pf *ProxyForwarder) forwardOutgoingToIncoming(ctx context.Context, params *ForwardParams, outgoing ClientStream) forwardError {
	if !params.Method.ServerStreaming {
		return pf.forwardUnaryResponse(ctx, params, outgoing)
	}

	first := true

	for {
		// It is not safe to reuse a Proto message after Incoming.Send().
		msg := params.Method.Output.New()

		// Error is handled after the headers are set, since can be returned even when an error occurs (non-OK status returned).
		err := outgoing.Recv(ctx, msg)

		// Additionally set the header once we receive the first response from the server.
		if first {
			first = false
			params.Incoming.SetHeader(pf.filter.FilterResponseMD(outgoing.Header()))
		}

		if err != nil {
			// Trailers must be available - outgoing.Recv has returned a non-nil error, possibly io.EOF.
			params.Incoming.SetTrailer(pf.filter.FilterTrailerMD(outgoing.Trailer()))

			if errors.Is(err, io.EOF) {
				return forwardError{outgoingErr: nil} // successful end of stream
			}
			return forwardError{outgoingErr: err}
		}

		if err := params.Incoming.Send(ctx, msg); err != nil {
			return forwardError{incomingErr: err}
		}
	}
}

// forwardUnaryResponse is separated from forwardOutgoingToIncoming since Recv must be called twice for unary responses
// to guarantee that the status has arrived - otherwise, gRPC might return the response successfully without a status,
// and the status will then be unable to get written to the client in cases like HTTP where the status is part of the headers sent BEFORE the response.
func (pf *ProxyForwarder) forwardUnaryResponse(ctx context.Context, params *ForwardParams, outgoing ClientStream) forwardError {
	msg := params.Method.Output.New()

	receivedResponse := false
	receivedTrailers := true // by default expect everything to go well

	err := outgoing.Recv(ctx, msg)
	if err == nil {
		receivedResponse = true

		// Response message received without errors, lets receive the status now.
		err = outgoing.Recv(ctx, params.Method.Output.New()) // avoid overwriting msg
		if err == nil {
			// Misbehaving server returned second message for supposedly unary request...
			// Ok, lets pretend that everything is fine, and simply not set any trailers.
			// Headers are fine though, since they come with the first response.
			receivedTrailers = false
			// Simulate a valid EOF, since the client is expecting a unary response, not a whole stream.
			err = io.EOF
		}
	}

	// Headers are guaranteed to have arrived.
	params.Incoming.SetHeader(pf.filter.FilterResponseMD(outgoing.Header()))

	// Set trailers if everything went fine (received io.EOF or a non-nil error on first/second Recv).
	if receivedTrailers {
		params.Incoming.SetTrailer(pf.filter.FilterTrailerMD(outgoing.Trailer()))
	}

	// err guaranteed to be non-nil, but we specifically handle io.EOF since it signifies a successful response.
	if !errors.Is(err, io.EOF) {
		return forwardError{outgoingErr: err}
	} else if !receivedResponse {
		// io.EOF from the first Recv(), meaning no response has arrived, which is an error for unary responses.
		return forwardError{outgoingErr: status.Errorf(codes.Unavailable, "grpcbridge: unexpected EOF from server for unary response: %s", err)}
	}

	if err := params.Incoming.Send(ctx, msg); err != nil {
		return forwardError{incomingErr: err}
	}

	return forwardError{outgoingErr: nil}
}

// https://github.com/grpc/grpc-go/blob/1e8b9b7fc655b756544cf10fded8f0d2fa3226e8/internal/transport/http_util.go#L185
// https://github.com/grpc-ecosystem/grpc-gateway/blob/17afee400c35e928b08998db1cd398af0b7ff371/runtime/context.go#L289
func decodeTimeout(s string) (time.Duration, bool) {
	size := len(s)

	if size < 2 || size > 9 {
		return 0, false
	}

	d, ok := timeoutUnitToDuration(s[size-1])
	if !ok {
		return 0, false
	}

	t, err := strconv.ParseInt(s[:size-1], 10, 64)
	if err != nil {
		return 0, false
	}

	const maxHours = math.MaxInt64 / int64(time.Hour)
	if d == time.Hour && t > maxHours {
		return time.Duration(math.MaxInt64), true
	}

	return d * time.Duration(t), true
}

func timeoutUnitToDuration(u uint8) (d time.Duration, ok bool) {
	switch u {
	case 'H':
		return time.Hour, true
	case 'M':
		return time.Minute, true
	case 'S':
		return time.Second, true
	case 'm':
		return time.Millisecond, true
	case 'u':
		return time.Microsecond, true
	case 'n':
		return time.Nanosecond, true
	default:
	}
	return
}
