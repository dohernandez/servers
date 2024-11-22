package servers_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/dohernandez/servers"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startMetricsService(host string, port uint, shutdownDoneCh, shutdownCh chan struct{}, collectors ...prometheus.Collector) (*servers.Metrics, string, error) {
	srv := servers.NewMetrics(
		&servers.Config{
			Name: "Test service",
			Host: host,
			Port: port,
		},
		servers.WithAddrAssigned(),
		servers.WithCollector(collectors...),
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

func TestMetrics_StartServing(t *testing.T) {
	srv, addr, err := startMetricsService("localhost", 0, nil, nil)
	require.NoErrorf(t, err, "failed to start metrics: %s", err)

	resp, err := http.Get(fmt.Sprintf("http://%s/metrics", addr))
	require.NoError(t, err)

	defer func() {
		resp.Body.Close() //nolint:errcheck,gosec

		srv.Stop()
	}()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestMetrics_Stop(t *testing.T) {
	shutdownDoneCh := make(chan struct{})
	shutdownCh := make(chan struct{})

	_, _, err := startMetricsService("localhost", 0, shutdownDoneCh, shutdownCh)
	require.NoErrorf(t, err, "failed to start metrics: %s", err)

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
