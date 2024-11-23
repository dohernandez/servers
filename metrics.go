package servers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"

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

// NewMetrics initiates a new wrapped prom server collector.
func NewMetrics(config Config, opts ...Option) *Metrics {
	srv := &Metrics{
		registered: make(map[string]bool),
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

	httpServer *http.Server

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

	srv.httpServer.Handler = promhttp.HandlerFor(
		reg,
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		},
	)

	if err := srv.Server.Start(); err != nil {
		return err
	}

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
