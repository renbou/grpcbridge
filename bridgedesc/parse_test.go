package bridgedesc

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/renbou/grpcbridge/internal/bridgetest/testpb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var testSvcDesc = &Target{
	Name:         "testpb",
	FileRegistry: testpb.TestServiceFileRegistry,
	TypeRegistry: testpb.TestServiceTypesRegistry,
	Services: []Service{
		{
			Name: protoreflect.FullName(testpb.TestService_ServiceDesc.ServiceName),
			Methods: []Method{
				{
					RPCName:         testpb.TestService_UnaryUnbound_FullMethodName,
					Input:           ConcreteMessage[testpb.Scalars](),
					Output:          ConcreteMessage[testpb.Scalars](),
					ClientStreaming: false,
					ServerStreaming: false,
				},
				{
					RPCName:         testpb.TestService_UnaryCombined_FullMethodName,
					Input:           ConcreteMessage[testpb.Combined](),
					Output:          ConcreteMessage[testpb.Combined](),
					ClientStreaming: false,
					ServerStreaming: false,
					Bindings: []Binding{{
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

func bridgedescOpts() cmp.Option {
	return cmp.Options{
		cmp.Transformer("MessagesToNames", func(message Message) protoreflect.FullName {
			return message.New().ProtoReflect().Descriptor().FullName()
		}),
		cmpopts.IgnoreInterfaces(struct{ FileRegistry }{}),
		cmpopts.IgnoreInterfaces(struct{ TypeRegistry }{}),
	}
}

// Test_ParseFileDescriptors_Ok tests that ParseFileDescriptors properly parses various sets of services and descriptors.
func Test_ParseFileDescriptors_Ok(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		services   []string
		wantTarget *Target
	}{
		{
			name:       "all test services",
			services:   []string{testpb.TestService_ServiceDesc.ServiceName},
			wantTarget: testSvcDesc,
		},
		{
			name:       "unknown service missing",
			services:   []string{testpb.TestService_ServiceDesc.ServiceName, "unknown"},
			wantTarget: &Target{Name: "testpb", Services: append(slices.Clone(testSvcDesc.Services), Service{Name: "unknown"})},
		},
		{
			name:       "unrequested service ignored",
			services:   []string{},
			wantTarget: &Target{Name: "testpb", Services: make([]Service, 0)},
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
			result := ParseTarget("testpb", testpb.TestServiceFileRegistry, testpb.TestServiceTypesRegistry, serviceNames)

			// Assert
			if diff := cmp.Diff(tt.wantTarget, result, bridgedescOpts()); diff != "" {
				t.Errorf("ParseTarget(%s, all descriptors) returned diff (-want +got):\n%s", serviceNames, diff)
			}
		})
	}
}
