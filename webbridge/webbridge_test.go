package webbridge

import (
	"testing"

	"github.com/renbou/grpcbridge/internal/bridgetest"
	"github.com/renbou/grpcbridge/internal/bridgetest/testpb"
	"github.com/renbou/grpcbridge/routing"
	"github.com/renbou/grpcbridge/transcoding"
	"go.uber.org/goleak"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

const ctJson = "application/json"

func mustTranscodedTestSvc(t *testing.T) (*testpb.TestService, *routing.PatternRouter, *transcoding.StandardTranscoder) {
	testsvc := testpb.NewTestService()

	server, pool, _ := bridgetest.MustGRPCServer(t, func(s *grpc.Server) {
		testpb.RegisterTestServiceServer(s, testsvc)
	})
	t.Cleanup(server.Stop)

	transcoder := transcoding.NewStandardTranscoder(transcoding.StandardTranscoderOpts{})
	router := routing.NewPatternRouter(pool, routing.PatternRouterOpts{})

	routerWatcher, err := router.Watch(bridgetest.TestTargetName)
	if err != nil {
		t.Fatalf("failed to create router watcher to write state: %s", err)
	}

	routerWatcher.UpdateDesc(testpb.TestServiceDesc)
	t.Cleanup(routerWatcher.Close)

	return testsvc, router, transcoder
}

func unmarshalJSON(b []byte, msg proto.Message) error {
	return transcoding.DefaultJSONMarshaler.Unmarshal(testpb.TestServiceTypesResolver, b, msg.ProtoReflect(), nil)
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
