package servers

import (
	"errors"
	"fmt"
	"net"
	"sync"
)

// ErrListenerFailedStart is returned when an error occurs starting listener for the REST server.
var ErrListenerFailedStart = errors.New("failed to start listener")

// Option sets up a server.
type Option func(srv any)

// WithAddrAssigned sets service to ask for listener assigned address. Mainly used when the port to the listener is assigned dynamically.
// Apply to all server instances.
func WithAddrAssigned() Option {
	return func(srv any) {
		s, ok := srv.(*Server)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		s.AddrAssigned = make(chan string, 1)
	}
}

// WithListener sets the listener. Server does not need to start a new one.
func WithListener(l net.Listener, shouldCloseListener bool) Option {
	return func(srv any) {
		s, ok := srv.(*Server)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		s.listener = l
		s.config.shouldCloseListener = shouldCloseListener
		s.addr = l.Addr().String()
	}
}

// Config contains configuration options for a Server.
type Config struct {
	Name string `envconfig:"NAME" required:"true"`
	Host string `envconfig:"HOST"`
	Port uint   `envconfig:"PORT" required:"true"`

	shouldCloseListener bool
}

// Server is a listening server instance.
type Server struct {
	config Config

	AddrAssigned chan string

	listener net.Listener
	addr     string

	shutdownSignal <-chan struct{}
	shutdownDone   chan<- struct{}

	sm sync.Mutex

	started bool
	stopped bool
}

// NewServer constructs a new Server.
func NewServer(config Config, opts ...Option) *Server {
	srv := Server{
		config: config,
	}

	srv.config.shouldCloseListener = true

	for _, o := range opts {
		o(&srv)
	}

	return &srv
}

// WithShutdownSignal adds channels to wait for shutdown and to report shutdown finished.
func (srv *Server) WithShutdownSignal(shutdown <-chan struct{}, done chan<- struct{}) {
	srv.shutdownSignal = shutdown
	srv.shutdownDone = done
}

// Name Service name.
func (srv *Server) Name() string {
	if srv.config.Name != "" {
		return srv.config.Name
	}

	return "server"
}

// Addr service address.
func (srv *Server) Addr() string {
	return srv.addr
}

// handleShutdown will wait and handle shutdown signal that comes to the server
// and gracefully shutdown the server.
func (srv *Server) handleShutdown() {
	if srv.shutdownSignal == nil {
		return
	}

	<-srv.shutdownSignal

	srv.Stop()

	close(srv.shutdownDone)
}

// Start starts serving the server.
func (srv *Server) Start() error {
	srv.sm.Lock()
	defer srv.sm.Unlock()

	if srv.started {
		return nil
	}

	srv.started = true

	if srv.listener == nil {
		lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", srv.config.Host, srv.config.Port))
		if err != nil {
			return fmt.Errorf("%w: %v", ErrListenerFailedStart, err) //nolint:errorlint
		}

		srv.listener = lis
		srv.addr = lis.Addr().String()
	}

	// the server is being asked for the dynamical address assigned.
	if srv.AddrAssigned != nil {
		srv.AddrAssigned <- srv.addr
	}

	go srv.handleShutdown()

	return nil
}

// Stop gracefully shuts down the grpc server.
func (srv *Server) Stop() {
	srv.sm.Lock()
	srv.stopped = true
	srv.sm.Unlock()

	if srv.listener != nil && srv.config.shouldCloseListener {
		srv.listener.Close() //nolint:errcheck,gosec
	}
}

type serve interface {
	Serve(lis net.Listener) error
}

func (srv *Server) serve(s serve, lis net.Listener) error {
	err := s.Serve(lis)
	if err == nil {
		return nil
	}

	srv.sm.Lock()
	defer srv.sm.Unlock()

	if srv.stopped {
		return nil
	}

	return err
}
