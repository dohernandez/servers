package servers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// ErrRESTStart is returned when an error occurs the REST server.
var ErrRESTStart = errors.New("start REST server")

// REST is a listening HTTP server instance.
type REST struct {
	*Server

	httpServer *http.Server
}

// NewREST constructs a new rest Server.
func NewREST(config *Config, handler http.Handler, opts ...Option) *REST {
	srv := REST{}

	srv.Server = NewServer(config, opts...)

	srv.httpServer = &http.Server{
		Handler:     handler,
		ReadTimeout: 400 * time.Millisecond,
	}

	return &srv
}

// Start starts serving the REST server.
func (srv *REST) Start() error {
	if err := srv.Server.Start(); err != nil {
		return err
	}

	if err := srv.serve(srv.httpServer, srv.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("%w: %v addr %v", ErrRESTStart, err, srv.listener.Addr().String()) //nolint:errorlint
	}

	return nil
}

// Stop gracefully shuts down the REST server.
func (srv *REST) Stop() {
	if err := srv.httpServer.Shutdown(context.Background()); err != nil {
		srv.httpServer.Close() //nolint:errcheck,gosec
	}

	srv.Server.Stop()
}
