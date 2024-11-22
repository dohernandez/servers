package servers_test

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/dohernandez/servers"
	"github.com/stretchr/testify/assert"
)

func startRESTService(mux *http.ServeMux, host string, port uint, shutdownDoneCh, shutdownCh chan struct{}) (*servers.REST, string, error) {
	srv := servers.NewREST(
		&servers.Config{
			Name: "Test service",
			Host: host,
			Port: port,
		},
		mux,
		servers.WithAddrAssigned(),
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

func TestREST_StartServing(t *testing.T) {
	resBody := "Hello world!\n"

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, resBody) //nolint:errcheck,gosec
	})

	srv, addr, err := startRESTService(mux, "localhost", 0, nil, nil)
	if err != nil {
		t.Errorf("failed to start REST: %s", err)
	}

	defer srv.Stop()

	res, err := http.Get(fmt.Sprintf("http://%s/", addr))
	if err != nil {
		t.Errorf("failed to call REST server: %s", err)
	}

	defer res.Body.Close() //nolint:errcheck

	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Errorf("failed to read response body: %s", err)
	}

	assert.Equalf(t, resBody, string(data), "unexpected response body: got %s, expected %s", string(data), resBody)
}

func TestREST_Stop(t *testing.T) {
	shutdownDoneCh := make(chan struct{})
	shutdownCh := make(chan struct{})

	_, _, err := startRESTService(nil, "localhost", 0, shutdownDoneCh, shutdownCh)
	if err != nil {
		t.Errorf("failed to start REST: %s", err)
	}

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
