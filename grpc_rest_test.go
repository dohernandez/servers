package servers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/bool64/ctxd"
	"github.com/bool64/zapctxd"
	"github.com/dohernandez/servers"
	"github.com/dohernandez/servers/testdata"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type gRPCRestTestServer struct {
	grpcEndpoint string
}

func newGRPCRestTestServer(grpcEndpoint string) *gRPCRestTestServer {
	return &gRPCRestTestServer{
		grpcEndpoint: grpcEndpoint,
	}
}

// RegisterServiceHandler registers the service implementation to mux.
func (srv *gRPCRestTestServer) RegisterServiceHandler(mux *runtime.ServeMux) error {
	// register rest service
	return testdata.RegisterGreeterHandlerFromEndpoint(context.Background(), mux, srv.grpcEndpoint, []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())})
}

func startGRPCRestService(grpcSrvAddr string, shutdownDoneCh, shutdownCh chan struct{}, ops ...servers.Option) (*servers.GRPCRest, string, error) {
	srvGRPCRest := newGRPCRestTestServer(grpcSrvAddr)

	ops = append(
		ops,
		servers.WithAddrAssigned(),
		servers.WithRegisterServiceHandler(srvGRPCRest),
	)

	// creating channel to return the error returned by servicing.Start.
	result := make(chan error, 1)

	grpcRestSrv, err := servers.NewGRPCRest(
		servers.Config{
			Name: "Test service",
		},
		append(ops, servers.WithAddrAssigned())...,
	)
	if err != nil {
		return nil, "", err
	}

	grpcRestSrv.WithShutdownSignal(shutdownCh, shutdownDoneCh)

	// starting the rest server
	go func() {
		err := grpcRestSrv.Start()

		result <- err
	}()

	var addr string

	select {
	case err := <-result:
		return nil, "", err
	// using srv.AddrAssigned to confirm that grpc server is up and running
	case addr = <-grpcRestSrv.AddrAssigned:
	}

	return grpcRestSrv, addr, nil
}

func TestGRPCRest_StartServing(t *testing.T) {
	grpcSrv, grpcAddr, err := startGRPCService(nil, nil, nil)
	require.NoErrorf(t, err, "start GRPC: %v", err)

	defer grpcSrv.Stop()

	srv, addr, err := startGRPCRestService(grpcAddr, nil, nil)
	require.NoErrorf(t, err, "start GRPC Rest: %v", err)

	defer srv.Stop()

	res, err := http.Get(fmt.Sprintf("http://%s/say/test", addr))
	if err != nil {
		t.Errorf("failed to call REST server: %s", err)
	}

	defer res.Body.Close() //nolint:errcheck

	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Errorf("failed to read response body: %s", err)
	}

	require.Equal(t, "{\"message\":\"Hello test\"}", string(data))
}

func TestGRPCRest_StartServing_WithChainUnaryInterceptor_Logger(t *testing.T) {
	var buff bytes.Buffer

	logger := zapctxd.New(zapctxd.Config{
		FieldNames: ctxd.FieldNames{
			Timestamp: "timestamp",
			Message:   "message",
		},
		Level:  zap.DebugLevel,
		Output: &buff,
	})

	grpcSrv, grpcAddr, err := startGRPCService(
		logger,
		nil,
		nil,
	)
	require.NoErrorf(t, err, "start GRPC: %v", err)

	defer grpcSrv.Stop()

	srv, addr, err := startGRPCRestService(grpcAddr, nil, nil)
	require.NoErrorf(t, err, "start GRPC Rest: %v", err)

	defer srv.Stop()

	res, err := http.Get(fmt.Sprintf("http://%s/say/test", addr))
	if err != nil {
		t.Errorf("failed to call REST server: %s", err)
	}

	defer res.Body.Close() //nolint:errcheck

	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Errorf("failed to read response body: %s", err)
	}

	require.Equal(t, "{\"message\":\"Hello test\"}", string(data))

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
