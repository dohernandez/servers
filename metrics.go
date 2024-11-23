package servers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// ErrMetricsStart is returned when an error occurs the Metrics server.
	ErrMetricsStart = errors.New("start Metrics server")
	// ErrCollectorAppended is returned when attempting to append a collector that already exists.
	ErrCollectorAppended = errors.New("collector already appended")
)

// WithCollector add collectors to the metrics.
func WithCollector(collectors ...prometheus.Collector) Option {
	return func(srv interface{}) {
		s, ok := srv.(*Metrics)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		if s.collectors == nil {
			s.collectors = make([]prometheus.Collector, 0)
		}

		for _, collector := range collectors {
			st := reflect.TypeOf(collector).String()

			if _, ok := s.registered[st]; ok {
				panic(fmt.Errorf("%w: type %s", ErrCollectorAppended, st))
			}

			s.collectors = append(s.collectors, collector)
			s.registered[st] = true
		}
	}
}

// WithGRPCServer sets the GRPC server.
func WithGRPCServer(grpcSrv *GRPC) Option {
	return func(srv any) {
		s, ok := srv.(*Metrics)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		s.grpcServer = grpcSrv
	}
}

// WithExposeAt sets the path to expose the metrics.
func WithExposeAt(path string) Option {
	return func(srv any) {
		s, ok := srv.(*Metrics)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		s.exposeAt = path
	}
}

// NewMetrics initiates a new wrapped prom server collector.
func NewMetrics(config Config, opts ...Option) *Metrics {
	srv := &Metrics{
		registered: make(map[string]bool),
		exposeAt:   "/metrics",
	}

	srv.Server = NewServer(config, opts...)

	for _, o := range opts {
		o(srv)
	}

	srv.httpServer = &http.Server{} //nolint:gosec

	return srv
}

// Metrics is a listening HTTP collector server instance.
type Metrics struct {
	*Server

	grpcServer *GRPC

	httpServer *http.Server
	exposeAt   string

	registered map[string]bool
	collectors []prometheus.Collector
}

// Start starts serving the Metrics server.
func (srv *Metrics) Start() error {
	// Create a collector registry.
	reg := prometheus.NewRegistry()

	reg.MustRegister(srv.collectors...)

	reg.MustRegister(collectors.NewBuildInfoCollector())
	reg.MustRegister(collectors.NewGoCollector())

	// Create a new ServeMux for routing.
	mux := http.NewServeMux()

	// Register the /metrics endpoint handler.
	mux.Handle(srv.exposeAt, promhttp.HandlerFor(
		reg,
		promhttp.HandlerOpts{
			EnableOpenMetrics: true, // Enable OpenMetrics format.
		},
	))

	if srv.grpcServer != nil {
		// Register gRPC metrics with the custom registry.
		grpcMetrics := grpc_prometheus.DefaultServerMetrics
		reg.MustRegister(grpcMetrics)

		// Initialize gRPC server metrics.
		grpcMetrics.InitializeMetrics(srv.grpcServer.grpcServer)
	}

	if err := srv.Server.Start(); err != nil {
		return err
	}

	// Use the custom mux with the /metrics handler.
	srv.httpServer.Handler = mux

	if err := srv.serve(srv.httpServer, srv.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("%w: %w addr %s", err, ErrMetricsStart, srv.listener.Addr().String())
	}

	return nil
}

// Stop gracefully shuts down the Metrics server.
func (srv *Metrics) Stop() {
	if err := srv.httpServer.Shutdown(context.Background()); err != nil {
		_ = srv.httpServer.Close() //nolint:errcheck
	}

	srv.Server.Stop()
}
