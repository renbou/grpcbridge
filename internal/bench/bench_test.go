package bench

import (
	context "context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	gwruntime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/routing"
	"github.com/renbou/grpcbridge/webbridge"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// Benchmark_GRPC runs a benchmark calling the NOOP Echo method via a normal grpc-go client.
func Benchmark_GRPC(b *testing.B) {
	buf := bufconn.Listen(1 << 20)

	server := grpc.NewServer()
	RegisterBenchServer(server, &BenchService{})
	go func() {
		_ = server.Serve(buf)
	}()

	client, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return buf.DialContext(ctx)
		}),
	)
	if err != nil {
		b.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	benchClient := NewBenchClient(client)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(p *testing.PB) {
		for {
			if !p.Next() {
				break
			}

			_, err := benchClient.Echo(context.Background(), &EchoMessage{A: "a", B: 1337, C: []string{"c", "d", "e"}})
			if err != nil {
				panic(err)
			}
		}
	})
}

// Benchmark_GRPCGateway runs a benchmark calling the NOOP Echo method via an HTTP client using gRPC-Gateway for relaying requests to the grpc-go server.
func Benchmark_GRPCGateway(b *testing.B) {
	buf := bufconn.Listen(1 << 20)

	mux := gwruntime.NewServeMux()
	_ = RegisterBenchHandlerServer(context.TODO(), mux, &BenchService{})

	server := &http.Server{
		Handler: mux,
	}
	go func() {
		_ = server.Serve(buf)
	}()

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return buf.DialContext(ctx)
		},
	}
	client := &http.Client{
		Transport: transport,
	}

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(p *testing.PB) {
		for {
			if !p.Next() {
				break
			}

			resp, err := client.Post("http://bufconn/echo/a?b=1337", "application/json", strings.NewReader(`["c", "d", "e"]`))
			if err != nil {
				panic(err)
			}

			// Reuse HTTP connection by properly reading the whole HTTP/1.1 body
			_, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
		}
	})
}

// Benchmark_GRPCBridge runs a benchmark calling the NOOP Echo method via an HTTP client using gRPCbridge for relaying requests to the grpc-go server.
func Benchmark_GRPCBridge(b *testing.B) {
	buf := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	RegisterBenchServer(server, &BenchService{})
	go func() {
		_ = server.Serve(buf)
	}()

	pool := grpcadapter.NewAdaptedClientPool(grpcadapter.AdaptedClientPoolOpts{
		DefaultOpts: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return buf.DialContext(ctx)
			}),
		},
	})
	_, _ = pool.New("gotestapi", "passthrough:///bufconn")

	router := routing.NewPatternRouter(pool, routing.PatternRouterOpts{})
	watcher, _ := router.Watch("gotestapi")
	watcher.UpdateDesc(bridgedesc.ParseTarget("gotestapi", protoregistry.GlobalFiles, protoregistry.GlobalTypes, []protoreflect.FullName{"bench.Bench"}))

	bridge := webbridge.NewTranscodedHTTPBridge(router, webbridge.TranscodedHTTPBridgeOpts{})

	buf2 := bufconn.Listen(1 << 20)
	httpServer := &http.Server{Handler: bridge}
	go func() {
		_ = httpServer.Serve(buf2)
	}()

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return buf2.DialContext(ctx)
		},
	}
	client := &http.Client{
		Transport: transport,
	}
	time.Sleep(time.Second)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(p *testing.PB) {
		for {
			if !p.Next() {
				break
			}

			resp, err := client.Post("http://bufconn/echo/a?b=1337", "application/json", strings.NewReader(`["c", "d", "e"]`))
			if err != nil {
				panic(err)
			}

			// Reuse HTTP connection by properly reading the whole HTTP/1.1 body
			_, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
		}
	})
}
