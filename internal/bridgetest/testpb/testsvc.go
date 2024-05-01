package testpb

import (
	context "context"
	"encoding/hex"
	"math/rand"
	sync "sync"
	"time"

	"github.com/google/go-cmp/cmp"
	codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	status "google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

const FlowMetadataKey = "flow-id"

func PrepareResponse[T proto.Message](response T, err error) *PreparedResponse[T] {
	return &PreparedResponse[T]{Response: response, Error: err}
}

type PreparedResponse[T proto.Message] struct {
	Response T
	Error    error
}

// TestService implements the test gRPC service with handlers which simply record the requests and return predetermined responses.
type TestService struct {
	UnimplementedTestServiceServer
	UnaryBoundRequest  *Scalars
	UnaryBoundResponse *PreparedResponse[*Combined]

	rndmu sync.Mutex
	rnd   *rand.Rand

	flowmu sync.Mutex
	flows  map[string][]*FlowAction
}

func NewTestService() *TestService {
	return &TestService{
		flows: make(map[string][]*FlowAction),
		rnd:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *TestService) UnaryBound(ctx context.Context, req *Scalars) (*Combined, error) {
	s.UnaryBoundRequest = req
	return s.UnaryBoundResponse.Response, s.UnaryBoundResponse.Error
}

func (s *TestService) BadResponsePath(context.Context, *Scalars) (*Combined, error) {
	return new(Combined), nil
}

func (s *TestService) Echo(ctx context.Context, req *Combined) (*Combined, error) {
	return req, nil
}

// AddFlow adds a flow and returns its ID to be sent in the metadata.
func (s *TestService) AddFlow(flow []*FlowAction) string {
	s.rndmu.Lock()
	idBytes := make([]byte, 8)
	_, _ = s.rnd.Read(idBytes)
	defer s.rndmu.Unlock()

	id := hex.EncodeToString(idBytes)

	s.flowmu.Lock()
	s.flows[id] = flow
	s.flowmu.Unlock()

	return id
}

func (s *TestService) flowFromMetadata(ctx context.Context) ([]*FlowAction, error) {
	flowID := metadata.ValueFromIncomingContext(ctx, FlowMetadataKey)

	if len(flowID) < 1 {
		return nil, status.Error(codes.InvalidArgument, "flow-id not found in metadata")
	}

	s.flowmu.Lock()
	defer s.flowmu.Unlock()

	flow, ok := s.flows[flowID[0]]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "flow with id %s not found", flowID[0])
	}

	return flow, nil
}

// ServerFlow simulates a pre-defined flow for a server-side stream.
func (s *TestService) ServerFlow(msg *FlowMessage, stream TestService_ServerFlowServer) error {
	flow, err := s.flowFromMetadata(stream.Context())
	if err != nil {
		return err
	}

	var received bool

	for i, action := range flow {
		ii := i + 1

		switch v := action.Action.(type) {
		case *FlowAction_ExpectMessage:
			if received {
				return status.Errorf(codes.FailedPrecondition, "ExpectMessage (%d/%d): multiple ExpectMessage for ServerFlow", ii, len(flow))
			}

			if diff := cmp.Diff(v.ExpectMessage, msg, protocmp.Transform()); diff != "" {
				return status.Errorf(codes.FailedPrecondition, "ExpectMessage (%d/%d): received message differs from expected (-want +got):\n%s", ii, len(flow), diff)
			}

			received = true
		case *FlowAction_ExpectClose:
			return status.Errorf(codes.FailedPrecondition, "ExpectClose (%d/%d): ExpectClose doesn't make sense for ServerFlow", ii, len(flow))
		case *FlowAction_SendMessage:
			if err := stream.Send(v.SendMessage); err != nil {
				return status.Errorf(codes.Internal, "SendMessage (%d/%d): stream.Send() returned non-nil error: %v", ii, len(flow), err)
			}
		case *FlowAction_SendStatus:
			return status.FromProto(v.SendStatus).Err()
		case *FlowAction_Sleep:
			select {
			case <-stream.Context().Done():
				return status.Errorf(codes.Canceled, "Sleep (%d/%d): context was canceled before sleep duration elapsed", ii, len(flow))
			case <-time.After(v.Sleep.AsDuration()):
			}
		default:
			return status.Errorf(codes.Internal, "(%d/%d): unrecognized server flow action type %T", ii, len(flow), action)
		}
	}

	return nil
}
