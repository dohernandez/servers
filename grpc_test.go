package servers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"contrib.go.opencensus.io/exporter/prometheus"
	"github.com/bool64/ctxd"
	"github.com/bool64/zapctxd"
	"github.com/dohernandez/servers"
	"github.com/dohernandez/servers/testdata"
	grpcPrometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type gRPCTestServer struct {
	// UnimplementedGreeterServer must be embedded to have forward compatible implementations.
	testdata.UnimplementedGreeterServer

	logger ctxd.Logger
}

func newGRPCTestServer(logger ctxd.Logger) *gRPCTestServer {
	return &gRPCTestServer{
		logger: logger,
	}
}

func (srv *gRPCTestServer) SayHello(ctx context.Context, req *testdata.HelloRequest) (*testdata.HelloReply, error) {
	srv.logger.Debug(ctx, fmt.Sprintf("Received: %v", req.GetName()))

	return &testdata.HelloReply{Message: "Hello " + req.GetName()}, nil
}

// RegisterService registers the service implementation to grpc service.
func (srv *gRPCTestServer) RegisterService(sr grpc.ServiceRegistrar) {
	testdata.RegisterGreeterServer(sr, srv)
}

func startGRPCService(logger ctxd.Logger, shutdownDoneCh, shutdownCh chan struct{}, ops ...servers.Option) (*servers.GRPC, string, error) {
	if logger == nil {
		logger = ctxd.NoOpLogger{}
	}

	srvGRPC := newGRPCTestServer(logger)

	ops = append(
		ops,
		servers.WithAddrAssigned(),
		servers.WithRegisterService(srvGRPC),
		servers.WithLogger(logger),
	)

	srv := servers.NewGRPC(
		servers.Config{
			Name: "Test service",
		},
		ops...,
	)

	srv.WithShutdownSignal(shutdownCh, shutdownDoneCh)

	// creating channel to return the error returned by servicing.Start.
	result := make(chan error, 1)

	// starting the server
	go func() {
		err := srv.Start()

		result <- err
	}()

	var addr string

	select {
	case err := <-result:
		return nil, "", err
	// using srv.AddrAssigned to confirm that grpc server is up and running
	case addr = <-srv.AddrAssigned:
	}

	return srv, addr, nil
}

func TestGRPC_StartServing(t *testing.T) {
	srv, addr, err := startGRPCService(nil, nil, nil)
	require.NoErrorf(t, err, "start GRPC: %v", err)

	defer srv.Stop()

	// Set up a connection to the server.
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "grpc connection: %v", err)

	defer conn.Close() //nolint:errcheck

	c := testdata.NewGreeterClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, err := c.SayHello(ctx, &testdata.HelloRequest{Name: "test"})
	require.NoErrorf(t, err, "SayHello request: %v", err)

	assert.Equal(t, "Hello test", r.GetMessage(), "response message are different: got %q, want %q", r.GetMessage(), "Hello test")
}

func TestGRPC_Stop(t *testing.T) {
	shutdownDoneCh := make(chan struct{})
	shutdownCh := make(chan struct{})

	_, _, err := startGRPCService(nil, shutdownDoneCh, shutdownCh)
	require.NoErrorf(t, err, "start GRPC: %v", err)

	// closing the server gracefully
	close(shutdownCh)

	deadline := time.After(10 * time.Second)

	select {
	case <-shutdownDoneCh:
		assert.True(t, true)
	case <-deadline:
		t.Errorf("failed shutdown deadline exceeded while waiting for server to shutdown")
	}
}

func TestGRPC_StartServing_WithHealthCheck(t *testing.T) {
	srv, addr, err := startGRPCService(
		nil,
		nil,
		nil,
		servers.WithHealthCheck(testdata.Greeter_ServiceDesc.ServiceName),
	)
	require.NoErrorf(t, err, "start GRPC: %v", err)

	defer srv.Stop()

	// Set up a connection to the server.
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "grpc connection: %v", err)

	defer conn.Close() //nolint:errcheck

	healthClient := grpc_health_v1.NewHealthClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	res, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{
		Service: testdata.Greeter_ServiceDesc.ServiceName,
	})
	require.NoError(t, err, "health call: %v", err)

	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, res.GetStatus(), "reported as non-serving")
}

func TestGRPC_StartServing_WithChainUnaryInterceptor_Logger(t *testing.T) {
	var buff bytes.Buffer

	logger := zapctxd.New(zapctxd.Config{
		FieldNames: ctxd.FieldNames{
			Timestamp: "timestamp",
			Message:   "message",
		},
		Level:  zap.DebugLevel,
		Output: &buff,
	})

	srv, addr, err := startGRPCService(
		logger,
		nil,
		nil,
	)
	require.NoErrorf(t, err, "start GRPC: %v", err)

	defer srv.Stop()

	// Set up a connection to the server.
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "grpc connection: %v", err)

	defer conn.Close() //nolint:errcheck

	c := testdata.NewGreeterClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = c.SayHello(ctx, &testdata.HelloRequest{Name: "test"})
	require.NoErrorf(t, err, "SayHello request: %v", err)

	// check log messages
	blines := bytes.Split(buff.Bytes(), []byte("\n"))

	lines := make([]json.RawMessage, 0, len(blines))

	for _, line := range blines {
		if len(line) == 0 {
			continue
		}

		lines = append(lines, line)
	}

	// check first log message
	out := make(map[string]any)
	err = json.Unmarshal(lines[0], &out)
	require.NoError(t, err, "unmarshal log: %v", err)

	assert.Equal(t, "info", out["level"], "wrong log level first message: got %s want debug", out["level"])
	assert.Equal(t, "started call", out["message"], "wrong log message second message: got %s want grpc: calling server", out["message"])

	// check second log message
	out = make(map[string]any)
	err = json.Unmarshal(lines[1], &out)
	require.NoError(t, err, "unmarshal log: %v", err)

	assert.Equal(t, "debug", out["level"], "wrong log level second message: got %s want debug", out["level"])
	assert.Equal(t, "Received: test", out["message"], "wrong log message first message: got %s want Received: test", out["message"])

	// check third log message
	out = make(map[string]any)
	err = json.Unmarshal(lines[2], &out)
	require.NoError(t, err, "unmarshal log: %v", err)

	assert.Equal(t, "info", out["level"], "wrong log level third message: got %s want info", out["level"])
	assert.Equal(t, "finished call", out["message"], "wrong log message third message: got %s want grpc: called server", out["message"])
}

func TestGRPC_StartServing_WithMetrics(t *testing.T) {
	httpServer := &http.Server{} //nolint:gosec
	promServerMetrics := grpcPrometheus.NewServerMetrics()

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", "localhost", 0))
	require.NoError(t, err)

	reg := prom.NewRegistry()
	reg.MustRegister(promServerMetrics)

	promExporter, err := prometheus.NewExporter(prometheus.Options{
		Registry:  reg,
		Namespace: "",
	})
	require.NoError(t, err)

	httpServer.Handler = promExporter

	go func() {
		_ = httpServer.Serve(lis) //nolint:errcheck
	}()

	defer httpServer.Shutdown(context.Background()) //nolint:errcheck

	srv, addr, err := startGRPCService(
		nil,
		nil,
		nil,
		servers.WithGRPCObserver(promServerMetrics),
	)
	require.NoErrorf(t, err, "start GRPC: %v", err)

	defer srv.Stop()

	// Set up a connection to the server.
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "grpc connection: %v", err)

	defer conn.Close() //nolint:errcheck

	c := testdata.NewGreeterClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, err := c.SayHello(ctx, &testdata.HelloRequest{Name: "test"})
	require.NoErrorf(t, err, "SayHello request: %v", err)

	assert.Equal(t, "Hello test", r.GetMessage(), "response message are different: got %q, want %q", r.GetMessage(), "Hello test")

	resp, err := http.Get(fmt.Sprintf("http://%s/metrics", lis.Addr().String()))
	require.NoError(t, err)

	defer func() {
		resp.Body.Close() //nolint:errcheck,gosec

		srv.Stop()
	}()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Checking metrics grpc_server_handled_total change to value 1
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	arr := strings.Split(string(data), "\n")

	var found string

	for _, s := range arr {
		if strings.Contains(s, "grpc_server_handled_total{grpc_code=\"OK\",grpc_method=\"SayHello\"") {
			found = s

			break
		}
	}

	kv := strings.Split(found, " ")

	n, err := strconv.Atoi(kv[1])
	require.NoError(t, err)

	assert.Equal(t, 1, n)
}
