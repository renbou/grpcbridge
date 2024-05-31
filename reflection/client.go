package reflection

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/renbou/grpcbridge/grpcadapter"
	"google.golang.org/grpc/codes"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type client struct {
	// Untyped stream instead of the ServerReflection_ServerReflectionInfoClient
	// because both v1 and v1alpha are exactly the same, so messages for them can be interchanged.
	stream  grpcadapter.ClientStream
	timeout time.Duration
}

func connectClient(timeout time.Duration, conn grpcadapter.ClientConn, method string) (*client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	stream, err := conn.Stream(ctx, method)
	if err != nil {
		return nil, fmt.Errorf("establishing reflection stream with method %q: %w", method, err)
	}

	return &client{stream: stream, timeout: timeout}, nil
}

func (c *client) close() {
	c.stream.CloseSend()

	// Close the reflection stream gracefully when possible, to avoid spurious errors on target servers.
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	_ = c.stream.Recv(ctx, new(reflectionpb.ServerReflectionResponse))

	c.stream.Close()
}

// listServiceNames executes the ListServices reflection request using a single timeout for both request and response.
// it expects that the response is of type ListServiceResponse, so it should be used once at the start and not
// reused alongside other requests.
// listServices doesn't deduplicate any received service names, if any, so it should be done by the caller if necessary,
// even though this isn't allowed by the protocol, but who knows what some external server might return.
// The returned names aren't validated to actually be complete service names, this needs to happen on the caller's side.
func (c *client) listServiceNames() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// NB: if this returns an error, the stream is successfully closed.
	err := c.stream.Send(ctx, &reflectionpb.ServerReflectionRequest{
		MessageRequest: &reflectionpb.ServerReflectionRequest_ListServices{},
	})
	isEOF := errors.Is(err, io.EOF)

	if err != nil && !isEOF {
		return nil, fmt.Errorf("sending ListServices request: %w", err)
	}

	resp := new(reflectionpb.ServerReflectionResponse)
	if err := c.recv(ctx, resp); err != nil {
		return nil, fmt.Errorf("receiving response to ListServices request: %w", err)
	} else if isEOF {
		return nil, errors.New("misbehaving ClientStream, Recv returned nil error after getting io.EOF from Send")
	}

	// sanity check to ensure that the response is valid
	if _, ok := resp.MessageResponse.(*reflectionpb.ServerReflectionResponse_ListServicesResponse); !ok {
		return nil, fmt.Errorf("received response to different request instead of ListServices (parallel call to reflection client?): %v", resp.MessageResponse)
	}

	// the client doesn't do any processing, so just return the names as is.
	services := resp.GetListServicesResponse().GetService()
	serviceNames := make([]string, len(services))

	for i, s := range services {
		serviceNames[i] = s.GetName()
	}

	return serviceNames, nil
}

// fileDescriptorsBySymbols retrieves the file descriptors for all the given symbols using the FileContainingSymbol request.
func (c *client) fileDescriptorsBySymbols(serviceNames []protoreflect.FullName) ([][]byte, error) {
	requests := make([]*reflectionpb.ServerReflectionRequest, len(serviceNames))

	for i, name := range serviceNames {
		requests[i] = &reflectionpb.ServerReflectionRequest{
			MessageRequest: &reflectionpb.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: string(name),
			},
		}
	}

	return c.execFileDescriptorRequests(requests, "FileContainingSymbol")
}

// fileDescriptorsByFilenames retrieves the file descriptors for all the given symbols using the FileByFilename request.
func (c *client) fileDescriptorsByFilenames(fileNames []string) ([][]byte, error) {
	requests := make([]*reflectionpb.ServerReflectionRequest, len(fileNames))

	for i, name := range fileNames {
		requests[i] = &reflectionpb.ServerReflectionRequest{
			MessageRequest: &reflectionpb.ServerReflectionRequest_FileByFilename{
				FileByFilename: name,
			},
		}
	}

	return c.execFileDescriptorRequests(requests, "FileByFilename")
}

// Sends/Recvs are made in parallel to minimize delays which would occur if done sequentially for all the symbols.
// NB: the responses aren't deduplicated by any means, so the file descriptors need to be properly parsed and de-duped by name.
// This is valid behaviour as specified in https://github.com/grpc/grpc/blob/aa67587bac54458464d38126c92d3a586a7c7a21/src/proto/grpc/reflection/v1/reflection.proto#L94.
func (c *client) execFileDescriptorRequests(requests []*reflectionpb.ServerReflectionRequest, name string) ([][]byte, error) {
	// avoid all this logic when it's not needed because why not?
	if len(requests) == 0 {
		return [][]byte{}, nil
	}

	semaphore := make(chan struct{}, len(requests))
	sendErr := make(chan error, 1)
	recvErr := make(chan error, 1)

	// wait for goroutines to exit to avoid any leaks
	var wg sync.WaitGroup
	wg.Add(2)
	defer wg.Wait()

	// single base context to cancel both goroutines when one fails
	// this is done specifically after the wg.Wait defer for cancel() to run before waiting
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		defer wg.Done()
		sendErr <- c.fileDescriptorsRequester(ctx, semaphore, requests, name)
	}()

	var res [][]byte
	go func() {
		defer wg.Done()
		recvd, err := c.fileDescriptorsReceiver(ctx, semaphore, requests, name)
		res = recvd // before channel send, which is a synchronizing operation
		recvErr <- err
	}()

	// If one of the goroutines fails prematurely, immediately return and cancel the context to stop the other one.
	// Otherwise wait for both of the signals to arrive and return the result.
	for range 2 {
		var err error

		select {
		case err = <-sendErr:
		case err = <-recvErr:
		}

		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

func (c *client) fileDescriptorsRequester(ctx context.Context, semaphore chan struct{}, requests []*reflectionpb.ServerReflectionRequest, name string) error {
	for i, req := range requests {
		if err := c.sendTimeout(ctx, req); err != nil {
			return fmt.Errorf("sending %s request %d/%d (%v): %w", name, i, len(requests), req, err)
		}

		semaphore <- struct{}{} // buffered channel
	}

	return nil
}

func (c *client) fileDescriptorsReceiver(ctx context.Context, semaphore chan struct{}, requests []*reflectionpb.ServerReflectionRequest, name string) ([][]byte, error) {
	// this preallocation is just a base prediction of the number of files,
	// based on the assumption that each symbol will return a different file.
	// in reality there can be more (files of dependencies) or less (multiple service in a file).
	fileDescriptors := make([][]byte, 0, len(requests))
	resp := new(reflectionpb.ServerReflectionResponse)

	// no validation of received files is performed here because any potential errors will be covered anyway during proto parsing and registry construction.
	for i := range requests {
		// wait for barrier #i to be released by the sender, otherwise we would start waiting for a response before the request is sent.
		select {
		case <-semaphore:
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		if err := c.recvTimeout(ctx, resp); err != nil {
			return nil, fmt.Errorf("receiving response %d/%d to %s request: %w", i, len(requests), name, err)
		}

		// sanity check to ensure that the response is valid
		if _, ok := resp.MessageResponse.(*reflectionpb.ServerReflectionResponse_FileDescriptorResponse); !ok {
			return nil, fmt.Errorf("received response to different request instead of %s (parallel call to reflection client?): %v", name, resp.MessageResponse)
		}

		fileDescriptors = append(fileDescriptors, resp.GetFileDescriptorResponse().GetFileDescriptorProto()...)
	}

	return fileDescriptors, nil
}

func (c *client) sendTimeout(ctx context.Context, req *reflectionpb.ServerReflectionRequest) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	if err := c.stream.Send(ctx, req); err != nil {
		// wrapped by the caller
		return err
	}

	return nil
}

func (c *client) recvTimeout(ctx context.Context, resp *reflectionpb.ServerReflectionResponse) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.recv(ctx, resp)
}

func (c *client) recv(ctx context.Context, resp *reflectionpb.ServerReflectionResponse) error {
	if err := c.stream.Recv(ctx, resp); err != nil {
		// wrapped by the caller
		return err
	} else if errRespWrapper, ok := resp.MessageResponse.(*reflectionpb.ServerReflectionResponse_ErrorResponse); ok {
		errResp := errRespWrapper.ErrorResponse
		return fmt.Errorf("ErrorResponse with status %w", status.Error(codes.Code(errResp.GetErrorCode()), errResp.GetErrorMessage()))
	}

	return nil
}
