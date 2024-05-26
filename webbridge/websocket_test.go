package webbridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gorilla/websocket"
	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/internal/bridgetest/testpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
)

func mustTranscodedWebSocketBridge(t *testing.T) (*testpb.TestService, *TranscodedWebSocketBridge) {
	testsvc, router, transcoder := mustTranscodedTestSvc(t)
	bridge := NewTranscodedWebSocketBridge(router, TranscodedWebSocketBridgeOpts{
		Transcoder: transcoder,
		Forwarder: grpcadapter.NewProxyForwarder(grpcadapter.ProxyForwarderOpts{
			Filter: grpcadapter.NewProxyMDFilter(grpcadapter.ProxyMDFilterOpts{
				AllowRequestMD: []string{testpb.FlowMetadataKey},
			}),
		}),
	})

	return testsvc, bridge
}

func wsTestURL(baseURL string, path string) string {
	return strings.ReplaceAll(baseURL, "http", "ws") + path
}

type wsFlow interface {
	isWSFlow()
}

type wsFlow_ExpectMessage struct {
	wsFlow
	expectMessage *testpb.FlowMessage
}

type wsFlow_ExpectClose struct {
	wsFlow
	code   int
	reason string
}

type wsFlow_SendMessage struct {
	wsFlow
	code    int
	message []byte
}

type wsFlow_Sleep struct {
	wsFlow
	duration time.Duration
}

type wsFlow_Close struct {
	wsFlow
	code int
}

func dialWebSocket(wsURLStr string, values url.Values) (*websocket.Conn, *http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	wsURL, err := url.Parse(wsURLStr)
	if err != nil {
		return nil, nil, fmt.Errorf("url.Parse(%s) returned non-nil error = %q", wsURLStr, err)
	}

	query := wsURL.Query()
	for k, v := range values {
		query[k] = v
	}
	wsURL.RawQuery = query.Encode()

	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, wsURL.String(), nil)
	if err != nil {
		return nil, resp, fmt.Errorf("websocket.DialContext(%s) returned non-nil error = %q", wsURL.String(), err)
	}

	return conn, resp, nil
}

func webSocketFlowTest(t *testing.T, wsURLStr string, clientFlow []wsFlow, serverFlowID string) {
	t.Helper()

	// Arrange
	// Set up flow on the test server, initiate websocket upgrade.
	// This should land us in the actual Forward() call, and we can start testing the flow execution.
	query := make(url.Values)
	query.Set(fmt.Sprintf("%s[%s]", defaultMetadataParam, testpb.FlowMetadataKey), serverFlowID)

	conn, _, err := dialWebSocket(wsURLStr, query)
	if err != nil {
		t.Fatalf("dialWebSocket(%s) returned non-nil error = %q", wsURLStr, err)
	}

	defer conn.Close()

	// Act && Assert
	for i, action := range clientFlow {
		ii := i + 1

		switch v := action.(type) {
		case *wsFlow_ExpectMessage:
			{
				msgType, msgData, err := conn.ReadMessage()
				if err != nil {
					t.Fatalf("ExpectMessage (%d/%d): conn.ReadMessage() returned non-nil error = %q", ii, len(clientFlow), err)
				} else if msgType != websocket.TextMessage {
					t.Fatalf("ExpectMessage (%d/%d): received message of type = %d, which isn't a valid JSON text message type", ii, len(clientFlow), msgType)
				}

				msg := new(testpb.FlowMessage)

				if err := protojson.Unmarshal(msgData, msg); err != nil {
					t.Fatalf("ExpectMessage (%d/%d): invalid message data, protojson.Unmarshal() returned non-nil error = %q", ii, len(clientFlow), err)
				}

				if diff := cmp.Diff(v.expectMessage, msg, protocmp.Transform()); diff != "" {
					t.Fatalf("ExpectMessage (%d/%d): received message differs from expected (-want +got):\n%s", ii, len(clientFlow), diff)
				}
			}
		case *wsFlow_ExpectClose:
			{
				_, _, err := conn.ReadMessage()
				if err == nil {
					t.Fatalf("ExpectClose (%d/%d): conn.ReadMessage() returned nil error, expected CloseError", ii, len(clientFlow))
				}

				var closeErr *websocket.CloseError
				if !errors.As(err, &closeErr) {
					t.Fatalf("ExpectClose (%d/%d): received error %q of type = %T, expected CloseError", ii, len(clientFlow), err, err)
				} else if closeErr.Code != v.code {
					t.Fatalf("ExpectClose (%d/%d): received close code = %d (reason = %q), want %d", ii, len(clientFlow), closeErr.Code, closeErr.Text, v.code)
				} else if !strings.Contains(closeErr.Text, v.reason) {
					t.Fatalf("ExpectClose (%d/%d): received close reason = %q, want %q", ii, len(clientFlow), closeErr.Text, v.reason)
				}
			}
		case *wsFlow_SendMessage:
			{
				if err := conn.WriteMessage(v.code, v.message); err != nil {
					t.Fatalf("SendMessage (%d/%d): conn.WriteMessage() returned non-nil error = %q", ii, len(clientFlow), err)
				}
			}
		case *wsFlow_Sleep:
			time.Sleep(v.duration)
		case *wsFlow_Close:
			// don't forcefully close the underlying network connection to test that the websocket handler properly handles such cases
			conn.WriteControl(websocket.CloseMessage, []byte(""), time.Time{})
		default:
			t.Fatalf("Client flow action %d/%d is of unrecognized type %T", ii, len(clientFlow), action)
		}
	}
}

// Test_TranscodedWebSocketBridge_ServerStream runs various execution flows for a server streaming handler.
func Test_TranscodedWebSocketBridge_ServerStream(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		endpoint   string
		serverFlow []*testpb.FlowAction
		clientFlow []wsFlow
	}{
		{
			name:     "no request message",
			endpoint: "/flow/server",
			serverFlow: []*testpb.FlowAction{
				{Action: &testpb.FlowAction_SendMessage{SendMessage: &testpb.FlowMessage{Message: "Hello"}}},
				{Action: &testpb.FlowAction_SendMessage{SendMessage: &testpb.FlowMessage{Message: "World"}}},
				{Action: &testpb.FlowAction_SendMessage{SendMessage: &testpb.FlowMessage{Message: "!"}}},
			},
			clientFlow: []wsFlow{
				&wsFlow_ExpectMessage{expectMessage: &testpb.FlowMessage{Message: "Hello"}},
				&wsFlow_ExpectMessage{expectMessage: &testpb.FlowMessage{Message: "World"}},
				&wsFlow_ExpectMessage{expectMessage: &testpb.FlowMessage{Message: "!"}},
				&wsFlow_ExpectClose{code: websocket.CloseNormalClosure, reason: ""},
			},
		},
		{
			name:     "request message without body",
			endpoint: "/flow/server?message=client-message",
			serverFlow: []*testpb.FlowAction{
				{Action: &testpb.FlowAction_SendMessage{SendMessage: &testpb.FlowMessage{Message: "Hello"}}},
				{Action: &testpb.FlowAction_ExpectMessage{ExpectMessage: &testpb.FlowMessage{Message: "client-message"}}},
			},
			clientFlow: []wsFlow{
				&wsFlow_ExpectMessage{expectMessage: &testpb.FlowMessage{Message: "Hello"}},
				&wsFlow_ExpectClose{code: websocket.CloseNormalClosure, reason: ""},
			},
		},
		{
			name:     "ignored body",
			endpoint: "/flow/server?message=original-message",
			serverFlow: []*testpb.FlowAction{
				{Action: &testpb.FlowAction_Sleep{Sleep: durationpb.New(time.Millisecond * 100)}}, // sleep to wait for any erroneous sends
				{Action: &testpb.FlowAction_ExpectMessage{ExpectMessage: &testpb.FlowMessage{Message: "original-message"}}},
			},
			clientFlow: []wsFlow{
				&wsFlow_SendMessage{code: websocket.TextMessage, message: []byte(`"another-message"`)},
				&wsFlow_ExpectClose{code: websocket.CloseNormalClosure, reason: ""},
			},
		},
		{
			name:     "request message with body",
			endpoint: "/flow/server:ws",
			serverFlow: []*testpb.FlowAction{
				{Action: &testpb.FlowAction_ExpectMessage{ExpectMessage: &testpb.FlowMessage{Message: "client-message"}}},
				{Action: &testpb.FlowAction_SendMessage{SendMessage: &testpb.FlowMessage{Message: "Hello"}}},
				{Action: &testpb.FlowAction_SendMessage{SendMessage: &testpb.FlowMessage{Message: "World"}}},
			},
			clientFlow: []wsFlow{
				&wsFlow_SendMessage{code: websocket.TextMessage, message: []byte(`"client-message"`)},
				&wsFlow_ExpectMessage{expectMessage: &testpb.FlowMessage{Message: "Hello"}},
				&wsFlow_ExpectMessage{expectMessage: &testpb.FlowMessage{Message: "World"}},
				&wsFlow_ExpectClose{code: websocket.CloseNormalClosure, reason: ""},
			},
		},
		{
			name:       "invalid body message type",
			endpoint:   "/flow/server:ws",
			serverFlow: []*testpb.FlowAction{},
			clientFlow: []wsFlow{
				&wsFlow_SendMessage{code: websocket.BinaryMessage, message: []byte(`"test"`)},
				&wsFlow_ExpectClose{code: websocket.CloseUnsupportedData, reason: "code InvalidArgument: received binary message instead of text"},
			},
		},
		{
			name:       "invalid body message",
			endpoint:   "/flow/server:ws",
			serverFlow: []*testpb.FlowAction{},
			clientFlow: []wsFlow{
				&wsFlow_SendMessage{code: websocket.TextMessage, message: []byte("\x00\x01\x02\x03")},
				&wsFlow_ExpectClose{code: websocket.CloseGoingAway, reason: "code InvalidArgument: unmarshaling request body:"},
			},
		},
		{
			name:     "immediate server error",
			endpoint: "/flow/server?message=message",
			serverFlow: []*testpb.FlowAction{
				{Action: &testpb.FlowAction_SendStatus{SendStatus: status.New(codes.Internal, "server error").Proto()}},
			},
			clientFlow: []wsFlow{
				&wsFlow_ExpectClose{code: websocket.CloseGoingAway, reason: "code Internal: server error"},
			},
		},
		{
			name:     "server error after messages",
			endpoint: "/flow/server?message=message",
			serverFlow: []*testpb.FlowAction{
				{Action: &testpb.FlowAction_SendMessage{SendMessage: &testpb.FlowMessage{Message: "Hello"}}},
				{Action: &testpb.FlowAction_SendStatus{SendStatus: status.New(codes.Internal, "server error").Proto()}},
			},
			clientFlow: []wsFlow{
				&wsFlow_ExpectMessage{expectMessage: &testpb.FlowMessage{Message: "Hello"}},
				&wsFlow_ExpectClose{code: websocket.CloseGoingAway, reason: "code Internal: server error"},
			},
		},
		{
			name:     "disconnect by client",
			endpoint: "/flow/server?message=message",
			serverFlow: []*testpb.FlowAction{
				// server will not acknowledge the close, but forwarder must detect that the client is no longer connected, i.e. context was canceled
				{Action: &testpb.FlowAction_Sleep{Sleep: durationpb.New(time.Second * 10)}},
			},
			clientFlow: []wsFlow{
				&wsFlow_Close{code: websocket.CloseNormalClosure},
				// give the server time to detect the client-side closure.
				// this doesn't weaken the test, but allows any goroutines to exit before the leak check.
				&wsFlow_Sleep{duration: time.Millisecond * 100},
			},
		},
	}

	// Arrange
	testsvc, bridge := mustTranscodedWebSocketBridge(t)
	server := httptest.NewServer(bridge)
	t.Cleanup(server.Close)

	// Run in non-parallel subtest so that server.Stop() runs AFTER all the subtests.
	t.Run("cases", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				webSocketFlowTest(t, wsTestURL(server.URL, tt.endpoint), tt.clientFlow, testsvc.AddFlow(tt.serverFlow))
			})
		}
	})
}

// Test_TranscodedWebSocketBridge_404 tests how TranscodedWebSocketBridge.ServeHTTP handles WebSocket upgrades on non-existent routes.
func Test_TranscodedWebSocketBridge_404(t *testing.T) {
	t.Parallel()

	// Arrange
	_, bridge := mustTranscodedWebSocketBridge(t)
	server := httptest.NewServer(bridge)
	t.Cleanup(server.Close)

	// Act
	conn, resp, err := dialWebSocket(wsTestURL(server.URL, "/notaroute"), url.Values{})

	// Assert
	if err == nil {
		conn.Close()
		t.Fatalf("dialWebSocket() got nil error for non-existent route")
	}

	if resp == nil {
		t.Fatalf("dialWebSocket() got nil response on non-existent route")
	} else if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("dialWebSocket() got response with status code = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func Test_TranscodedWebSocketBridge_UpgradeError(t *testing.T) {
	t.Parallel()

	// Arrange
	// Real server needed because recorder doesn't support hijacking.
	_, bridge := mustTranscodedWebSocketBridge(t)
	server := httptest.NewServer(bridge)
	t.Cleanup(server.Close)

	// Act
	req, err := http.NewRequest("GET", server.URL+"/flow/server", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() returned non-nil error = %q", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /flow/server returned non-nil error = %q", err)
	}

	t.Cleanup(func() {
		io.ReadAll(resp.Body)
		resp.Body.Close()
	})

	// Assert
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("TranscodedWebSocketBridge.ServeHTTP() returned status code = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
