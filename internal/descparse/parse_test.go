package descparse

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/internal/bridgetest/testpb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func bridgedescOpts() cmp.Option {
	return cmp.Options{
		cmp.Transformer("MessagesToNames", func(message bridgedesc.Message) protoreflect.FullName {
			return message.New().ProtoReflect().Descriptor().FullName()
		}),
	}
}

// Test_ParseFileDescriptors_Ok tests that ParseFileDescriptors properly parses various sets of services and descriptors.
func Test_ParseFileDescriptors_Ok(t *testing.T) {
	t.Parallel()

	wantDesc := &bridgedesc.Target{
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
				},
			},
		},
	}

	tests := []struct {
		name        string
		services    []string
		wantResults ParseResults
	}{
		{
			name:        "all test services",
			services:    []string{testpb.TestService_ServiceDesc.ServiceName},
			wantResults: ParseResults{Desc: wantDesc},
		},
		{
			name:     "unknown service missing",
			services: []string{testpb.TestService_ServiceDesc.ServiceName, "unknown"},
			wantResults: ParseResults{
				Desc:            &bridgedesc.Target{Services: append(slices.Clone(wantDesc.Services), bridgedesc.Service{Name: "unknown"})},
				MissingServices: []protoreflect.FullName{"unknown"},
			},
		},
		{
			name:        "unrequested service ignored",
			services:    []string{},
			wantResults: ParseResults{Desc: &bridgedesc.Target{Services: make([]bridgedesc.Service, 0)}},
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
			result, gotErr := ParseFileDescriptors(serviceNames, testpb.TestServiceFDs)

			// Assert
			if gotErr != nil {
				t.Errorf("ParseFileDescriptors(%s, all descriptors) returned non-nil error = %q", serviceNames, gotErr)
			}

			if diff := cmp.Diff(tt.wantResults, result, bridgedescOpts()); diff != "" {
				t.Errorf("ParseFileDescriptors(%s, all descriptors) returned diff (-want +got):\n%s", serviceNames, diff)
			}
		})
	}
}
