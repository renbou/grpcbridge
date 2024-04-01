package reflection

import (
	"context"
	"fmt"
	"time"

	"github.com/renbou/grpcbridge/grpcadapter"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
)

type client struct {
	// Untyped stream instead of the ServerReflection_ServerReflectionInfoClient
	// because both v1 and v1alpha are exactly the same, so messages for them can be interchanged.
	stream  grpcadapter.BiDiStream
	timeout time.Duration
}

func connectClient(timeout time.Duration, conn grpcadapter.Connection, method string) (*client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	stream, err := conn.BiDiStream(ctx, method)
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

// listServices executes the ListServices reflection request using a single timeout for both request and response.
// it expects that the response is of type ListServiceResponse, so it should be used once at the start and not
// reused alongside other requests.
func (c *client) listServices() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// NB: if this returns an error, the stream is successfully closed.
	if err := c.stream.Send(ctx, &reflectionpb.ServerReflectionRequest{
		MessageRequest: &reflectionpb.ServerReflectionRequest_ListServices{},
	}); err != nil {
		return nil, fmt.Errorf("sending ListServices request: %w", err)
	}

	m := new(reflectionpb.ServerReflectionResponse)
	if err := c.stream.Recv(ctx, m); err != nil {
		return nil, fmt.Errorf("receiving response to ListServices request: %w", err)
	}

	// the client doesn't do any processing, so just return the names as is.
	services := m.GetListServicesResponse().GetService()
	serviceNames := make([]string, len(services))

	for i, s := range services {
		serviceNames[i] = s.GetName()
	}

	return serviceNames, nil
}
