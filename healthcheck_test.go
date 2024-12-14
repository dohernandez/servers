package servers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/dohernandez/servers"
	"github.com/hellofresh/health-go/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthcheckService_root(t *testing.T) {
	srv := servers.NewHealthCheck(
		servers.Config{
			Name: "Health test service",
			Host: "localhost",
			Port: 0,
		},
		servers.WithHealthCheck(
			health.Config{
				Name: "health-check",
				Check: func(ctx context.Context) error {
					return nil
				},
			}),
		servers.WithAddrAssigned(),
	)

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
		t.Fatalf("failed to start REST: %s", err)
	// using srv.AddrAssigned to confirm that grpc server is up and running
	case addr = <-srv.AddrAssigned:
	}

	res, err := http.Get(fmt.Sprintf("http://%s/", addr))
	require.NoError(t, err)

	_ = res.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusOK, res.StatusCode, "/ returned wrong status code: got %s, want %s", res.StatusCode, http.StatusOK)

	res, err = http.Get(fmt.Sprintf("http://%s/health", addr))
	require.NoError(t, err)

	content, err := resBodyContent(res)
	require.NoError(t, err)

	body := make(map[string]interface{})
	err = json.NewDecoder(bytes.NewReader(content)).Decode(&body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, res.StatusCode, "/health returned wrong status code: got %s, want %s", res.StatusCode, http.StatusOK)

	assert.Equal(t, string(health.StatusOK), body["status"], "wrong status: got %s, want %s", body["status"], string(health.StatusOK))
}

func resBodyContent(res *http.Response) ([]byte, error) {
	defer res.Body.Close() //nolint:errcheck

	return io.ReadAll(res.Body)
}

func TestHealthcheckService_root_grpc_rest(t *testing.T) {
	grpcSrv, grpcAddr, err := startGRPCService(nil, nil, nil)
	require.NoErrorf(t, err, "start GRPC: %v", err)

	defer grpcSrv.Stop()

	grpcRestSrv, _, err := startGRPCRestService(grpcAddr, nil, nil)
	require.NoErrorf(t, err, "start GRPC Rest: %v", err)

	defer grpcRestSrv.Stop()

	srv := servers.NewHealthCheck(
		servers.Config{
			Name: "Health test service",
			Host: "localhost",
			Port: 0,
		},
		servers.WithHealthCheck(
			health.Config{
				Name: "health-check",
				Check: func(ctx context.Context) error {
					return nil
				},
			}),
		servers.WithGRPCRest(grpcRestSrv),
		servers.WithAddrAssigned(),
	)

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
		t.Fatalf("failed to start REST: %s", err)
	// using srv.AddrAssigned to confirm that grpc server is up and running
	case addr = <-srv.AddrAssigned:
	}

	res, err := http.Get(fmt.Sprintf("http://%s/", addr))
	require.NoError(t, err)

	_ = res.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusOK, res.StatusCode, "/ returned wrong status code: got %s, want %s", res.StatusCode, http.StatusOK)
}

func TestHealthcheckService_health(t *testing.T) {
	grpcSrv, grpcAddr, err := startGRPCService(
		nil,
		nil,
		nil,
		servers.WithGrpcHealthCheck(),
	)
	require.NoErrorf(t, err, "start GRPC: %v", err)

	defer grpcSrv.Stop()

	grpcRestSrv, _, err := startGRPCRestService(grpcAddr, nil, nil)
	require.NoErrorf(t, err, "start GRPC Rest: %v", err)

	defer grpcRestSrv.Stop()

	srv := servers.NewHealthCheck(
		servers.Config{
			Name: "Health test service",
			Host: "localhost",
			Port: 0,
		},
		servers.WithHealthCheck(
			health.Config{
				Name: "health-check",
				Check: func(ctx context.Context) error {
					return nil
				},
			}),
		servers.WithGRPC(grpcSrv),
		servers.WithGRPCRest(grpcRestSrv),
		servers.WithAddrAssigned(),
	)

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
		t.Fatalf("failed to start REST: %s", err)
	// using srv.AddrAssigned to confirm that grpc server is up and running
	case addr = <-srv.AddrAssigned:
	}

	res, err := http.Get(fmt.Sprintf("http://%s/health", addr))
	require.NoError(t, err)

	_ = res.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusOK, res.StatusCode, "/ returned wrong status code: got %s, want %s", res.StatusCode, http.StatusOK)
}
