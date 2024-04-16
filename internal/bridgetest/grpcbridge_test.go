package bridgetest

import (
	"context"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/renbou/grpcbridge"
	"github.com/renbou/grpcbridge/internal/bridgetest/testpb"
	"github.com/renbou/grpcbridge/transcoding"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
)

var testEchoMessage = &testpb.Combined{
	Scalars: &testpb.Scalars{
		BoolValue:     true,
		Int32Value:    -12379,
		Uint32Value:   378,
		Uint64Value:   math.MaxUint64,
		Int64Value:    math.MinInt64,
		Sfixed64Value: 1,
		DoubleValue:   math.Inf(-1),
		StringValue:   "sttrrr",
	},
	NonScalars: &testpb.NonScalars{
		Str2StrMap: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		Str2Int32Map: map[string]int32{
			"one": 1,
			"two": 2,
		},
		Duration: durationpb.New(time.Millisecond * 123),
		Child: &testpb.NonScalars_Child{
			Nested: &testpb.NonScalars_Child_Digits{Digits: testpb.Digits_TWO},
		},
	},
}

// Test_GRPCBridge performs a basic integration test of all the [grpcbridge] functionality.
func Test_GRPCBridge(t *testing.T) {
	t.Parallel()

	// Arrange
	// Set up a single complete grpcbridge instance for all tests to test parallelism.
	targetListener, targetServer := mustRawServer(t, nil, []func(*grpc.Server){func(s *grpc.Server) {
		testpb.RegisterTestServiceServer(s, new(testpb.TestService))
		reflection.Register(s)
	}})

	defer targetServer.Stop()

	router := grpcbridge.NewReflectionRouter(
		grpcbridge.WithLogger(Logger(t)),
		grpcbridge.WithDialOpts(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
				return targetListener.Dial()
			}),
		),
		grpcbridge.WithConnFunc(grpc.NewClient),
		grpcbridge.WithDisabledReflectionPolling(),
		grpcbridge.WithReflectionPollInterval(time.Second),
	)

	if ok, err := router.Add(TestTargetName, TestDialTarget); err != nil {
		t.Fatalf("ReflectionRouter.Add(%q) returned non-nil error = %q", TestTargetName, err)
	} else if !ok {
		t.Fatalf("ReflectionRouter.Add(%q) returned false, expected it to successfully add test target", TestTargetName)
	}

	t.Cleanup(func() {
		if ok := router.Remove(TestTargetName); !ok {
			t.Fatalf("ReflectionRouter.Remove(%q) returned false, expected it to successfully remove test target", TestTargetName)
		}
	})

	// Wait for the router to update.
	// TODO(renbou): perhaps introduce a blocking mode to the reflection resolver,
	// which would allow waiting for the first resolution to succeed, or return an error?
	time.Sleep(time.Millisecond * 50)

	proxy := grpcbridge.NewGRPCProxy(router, grpcbridge.WithLogger(Logger(t)))
	bridge := grpcbridge.NewWebBridge(router,
		grpcbridge.WithLogger(Logger(t)),
		grpcbridge.WithDefaultMarshaler(transcoding.DefaultJSONMarshaler),
		grpcbridge.WithMarshalers([]transcoding.Marshaler{transcoding.DefaultJSONMarshaler}),
	)

	// Set up servers which simulate grpcbridge itself.
	bridgeListener, grpcServer := mustRawServer(t, []grpc.ServerOption{proxy.AsServerOption()}, nil)
	defer grpcServer.Stop()

	httpServer := httptest.NewServer(bridge)
	defer httpServer.Close()

	grpcClient, err := grpc.NewClient(TestDialTarget,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return bridgeListener.Dial()
		}),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient(grpcbridge) returned non-nil error = %q", err)
	}

	httpClient := httpServer.Client()

	// Run in non-parallel subtest so that server.Stop() runs AFTER all the subtests.
	t.Run("cases", func(t *testing.T) {
		t.Run("proxy", func(t *testing.T) {
			test_GRPCBridge_Proxy(t, grpcClient)
		})

		t.Run("bridge", func(t *testing.T) {
			test_GRPCBridge_Bridge(t, httpClient, httpServer)
		})
	})
}

func test_GRPCBridge_Proxy(t *testing.T, client *grpc.ClientConn) {
	t.Parallel()

	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	testClient := testpb.NewTestServiceClient(client)

	// Act
	resp, gotErr := testClient.Echo(ctx, testEchoMessage)

	// Assert
	if gotErr != nil {
		t.Errorf("TestService.Echo() returned non-nil error = %q", gotErr)
	}

	if diff := cmp.Diff(testEchoMessage, resp, protocmp.Transform()); diff != "" {
		t.Fatalf("TestService.Echo() returned response differing from request (-want+got):\n%s", diff)
	}
}

func test_GRPCBridge_Bridge(t *testing.T, client *http.Client, server *httptest.Server) {
	t.Parallel()

	// Arrange
	testBody := `{
		"scalars": {
			"boolValue": true,
			"int32Value": -12379,
			"uint32Value": 378,
			"uint64Value": "18446744073709551615",
			"int64Value": "-9223372036854775808",
			"sfixed64Value": 1,
			"doubleValue": "-Infinity",
			"stringValue": "sttrrr"
		},
		"nonScalars": {
			"str2strMap": {
				"key1": "value1",
				"key2": "value2"
			},
			"str2int32Map": {
				"one": 1,
				"two": 2
			},
			"duration": "0.123s",
			"child": {
				"digits": "TWO"
			}
		}
	}`

	// Act
	resp, gotErr := client.Post(server.URL+"/service/echo", "application/json", strings.NewReader(testBody))

	// Assert
	if gotErr != nil {
		t.Fatalf("POST(/service/echo) returned non-nil error = %q", gotErr)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST(/service/echo) returned non-OK response = %s (%s)", resp.Status, string(body))
	}

	respMsg := new(testpb.Combined)
	if err := transcoding.DefaultJSONMarshaler.Unmarshal(testpb.TestServiceTypesResolver, []byte(testBody), respMsg.ProtoReflect(), nil); err != nil {
		t.Fatalf("POST(/service/echo) returned invalid body, failed to unmarshal with error = %q", err)
	}

	if diff := cmp.Diff(testEchoMessage, respMsg, protocmp.Transform()); diff != "" {
		t.Fatalf("POST(/service/echo) returned response differing from request (-want+got):\n%s", diff)
	}
}
