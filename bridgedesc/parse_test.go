// Package bridgedesc_test contains tests for the bridgedesc package to avoid loop cycles.
package bridgedesc_test

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/internal/bridgetest/testpb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var testSvcDesc = &bridgedesc.Target{
	Name:         "testpb",
	FileResolver: testpb.TestServiceFileResolver,
	TypeResolver: testpb.TestServiceTypesResolver,
	Services: []bridgedesc.Service{
		{
			Name: protoreflect.FullName(testpb.TestService_ServiceDesc.ServiceName),
			Methods: []bridgedesc.Method{
				{
					RPCName:         testpb.TestService_UnaryUnbound_FullMethodName,
					Input:           bridgedesc.ConcreteMessage[testpb.Scalars](),
					Output:          bridgedesc.ConcreteMessage[testpb.Scalars](),
					ClientStreaming: false,
					ServerStreaming: false,
				},
				{
					RPCName:         testpb.TestService_UnaryBound_FullMethodName,
					Input:           bridgedesc.ConcreteMessage[testpb.Scalars](),
					Output:          bridgedesc.ConcreteMessage[testpb.Combined](),
					ClientStreaming: false,
					ServerStreaming: false,
					Bindings: []bridgedesc.Binding{
						{
							HTTPMethod:       "POST",
							Pattern:          "/service/unary/{string_value}/{fixed64_value}",
							RequestBodyPath:  "bytes_value",
							ResponseBodyPath: "",
						},
					},
				},
				{
					RPCName:         testpb.TestService_UnaryCombined_FullMethodName,
					Input:           bridgedesc.ConcreteMessage[testpb.Combined](),
					Output:          bridgedesc.ConcreteMessage[testpb.Combined](),
					ClientStreaming: false,
					ServerStreaming: false,
					Bindings: []bridgedesc.Binding{{
						HTTPMethod:       "POST",
						Pattern:          "/service/combined/{scalars.bool_value}/{scalars.string_value}",
						RequestBodyPath:  "non_scalars",
						ResponseBodyPath: "non_scalars",
					}},
				},
				{
					RPCName:         testpb.TestService_BadResponsePath_FullMethodName,
					Input:           bridgedesc.ConcreteMessage[testpb.Scalars](),
					Output:          bridgedesc.ConcreteMessage[testpb.Combined](),
					ClientStreaming: false,
					ServerStreaming: false,
					Bindings: []bridgedesc.Binding{{
						HTTPMethod:       "POST",
						Pattern:          "/service/bad-response-path",
						ResponseBodyPath: "not_a_field",
					}},
				},
				{
					RPCName:         testpb.TestService_Echo_FullMethodName,
					Input:           bridgedesc.ConcreteMessage[testpb.Combined](),
					Output:          bridgedesc.ConcreteMessage[testpb.Combined](),
					ClientStreaming: false,
					ServerStreaming: false,
					Bindings: []bridgedesc.Binding{{
						HTTPMethod:       "POST",
						Pattern:          "/service/echo",
						RequestBodyPath:  "*",
						ResponseBodyPath: "",
					}},
				},
				{
					RPCName:         testpb.TestService_UnaryFlow_FullMethodName,
					Input:           bridgedesc.ConcreteMessage[testpb.FlowMessage](),
					Output:          bridgedesc.ConcreteMessage[testpb.FlowMessage](),
					ClientStreaming: false,
					ServerStreaming: false,
				},
				{
					RPCName:         testpb.TestService_ClientFlow_FullMethodName,
					Input:           bridgedesc.ConcreteMessage[testpb.FlowMessage](),
					Output:          bridgedesc.ConcreteMessage[testpb.FlowMessage](),
					ClientStreaming: true,
					ServerStreaming: false,
				},
				{
					RPCName:         testpb.TestService_ServerFlow_FullMethodName,
					Input:           bridgedesc.ConcreteMessage[testpb.FlowMessage](),
					Output:          bridgedesc.ConcreteMessage[testpb.FlowMessage](),
					ClientStreaming: false,
					ServerStreaming: true,
					Bindings: []bridgedesc.Binding{
						{
							HTTPMethod: "GET",
							Pattern:    "/flow/server",
						},
						{
							HTTPMethod:      "GET",
							Pattern:         "/flow/server:ws",
							RequestBodyPath: "message",
						},
					},
				},
				{
					RPCName:         testpb.TestService_BiDiFlow_FullMethodName,
					Input:           bridgedesc.ConcreteMessage[testpb.FlowMessage](),
					Output:          bridgedesc.ConcreteMessage[testpb.FlowMessage](),
					ClientStreaming: true,
					ServerStreaming: true,
				},
			},
		},
	},
}

func bridgedescOpts() cmp.Option {
	return cmp.Options{
		cmp.Transformer("MessagesToNames", func(message bridgedesc.Message) protoreflect.FullName {
			return message.New().ProtoReflect().Descriptor().FullName()
		}),
		cmpopts.IgnoreInterfaces(struct{ bridgedesc.FileResolver }{}),
		cmpopts.IgnoreInterfaces(struct{ bridgedesc.TypeResolver }{}),
	}
}

// Test_ParseFileDescriptors_Ok tests that ParseFileDescriptors properly parses various sets of services and descriptors.
func Test_ParseFileDescriptors_Ok(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		services   []string
		wantTarget *bridgedesc.Target
	}{
		{
			name:       "all test services",
			services:   []string{testpb.TestService_ServiceDesc.ServiceName},
			wantTarget: testSvcDesc,
		},
		{
			name:       "unknown service missing",
			services:   []string{testpb.TestService_ServiceDesc.ServiceName, "unknown"},
			wantTarget: &bridgedesc.Target{Name: "testpb", Services: append(slices.Clone(testSvcDesc.Services), bridgedesc.Service{Name: "unknown"})},
		},
		{
			name:       "unrequested service ignored",
			services:   []string{},
			wantTarget: &bridgedesc.Target{Name: "testpb", Services: make([]bridgedesc.Service, 0)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			serviceNames := make([]protoreflect.FullName, len(tt.services))
			for i, svc := range tt.services {
				serviceNames[i] = protoreflect.FullName(svc)
			}

			// Act
			result := bridgedesc.ParseTarget("testpb", testpb.TestServiceFileResolver, testpb.TestServiceTypesResolver, serviceNames)

			// Assert
			if diff := cmp.Diff(tt.wantTarget, result, bridgedescOpts()); diff != "" {
				t.Errorf("ParseTarget(%s, all descriptors) returned diff (-want +got):\n%s", serviceNames, diff)
			}
		})
	}
}
