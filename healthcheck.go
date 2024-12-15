package servers

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hellofresh/health-go/v5"
	healthGRPC "github.com/hellofresh/health-go/v5/checks/grpc"
	healthHttp "github.com/hellofresh/health-go/v5/checks/http"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type healthOptions struct {
	checks []health.Config

	grpcRest *GRPCRest
	grpc     *GRPC
}

// WithHealthCheck sets up health check server.
func WithHealthCheck(checks ...health.Config) Option {
	return func(srv any) {
		s, ok := srv.(*HealthCheck)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		s.options.checks = append(s.options.checks, checks...)
	}
}

// WithGRPC sets up gRPC server options.
func WithGRPC(grpc *GRPC) Option {
	return func(srv any) {
		s, ok := srv.(*HealthCheck)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		s.options.grpc = grpc
	}
}

// WithGRPCRest sets up gRPC REST and with server options.
func WithGRPCRest(grpcRest *GRPCRest) Option {
	return func(srv any) {
		s, ok := srv.(*HealthCheck)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		s.options.grpcRest = grpcRest
	}
}

// HealthCheck is a listening HTTP server instance with the endpoints "/" and "/health" mainly use for
// livenessProbe and readinessProbe on Kubernetes cluster.
type HealthCheck struct {
	*REST

	options healthOptions
}

// NewHealthCheck is a listening HTTP server instance with the endpoints "/" and "/health" mainly use for
// livenessProbe and readinessProbe on Kubernetes cluster.
//
//nolint:funlen
func NewHealthCheck(cfg Config, opts ...Option) *HealthCheck {
	srv := &HealthCheck{}

	for _, o := range opts {
		o(srv)
	}

	h, _ := health.New(health.WithSystemInfo()) //nolint:errcheck

	for _, check := range srv.options.checks {
		_ = h.Register(check) //nolint:errcheck
	}

	var baseRESTURL string

	if srv.options.grpcRest != nil {
		baseRESTURL = strings.Replace(srv.options.grpcRest.Addr(), "[::]", "127.0.0.1", 1)

		_ = h.Register(health.Config{ //nolint:errcheck
			Name:      "grpc-rest-check",
			Timeout:   time.Second * 5,
			SkipOnErr: false,
			Check: healthHttp.New(healthHttp.Config{
				URL: "http://" + baseRESTURL,
			}),
		})
	}

	if srv.options.grpc != nil {
		_ = h.Register(health.Config{ //nolint:errcheck
			Name:      "grpc-check",
			Timeout:   time.Second * 5,
			SkipOnErr: false,
			Check: healthGRPC.New(healthGRPC.Config{
				Target:  srv.options.grpc.Addr(),
				Service: srv.options.grpc.Name(),
				DialOptions: []grpc.DialOption{
					grpc.WithTransportCredentials(insecure.NewCredentials()),
				},
			}),
		})
	}

	r := chi.NewRouter()

	// Check if the rest service is up and running by hitting the root endpoint.
	r.Method(http.MethodGet, "/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Del("Content-Length")
		w.Header().Set("Content-Type", "application/json")

		if baseRESTURL != "" {
			// Check if the rest service is up and running by hitting the root endpoint.
			req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "http://"+baseRESTURL, nil)
			if err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)

				return
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)

				return
			}

			defer resp.Body.Close() //nolint:errcheck

			w.WriteHeader(resp.StatusCode)
		}

		w.WriteHeader(http.StatusOK)
	}))

	// Check if the service dependencies are up and running.
	r.Method(http.MethodGet, "/health", h.Handler())

	srv.REST = NewREST(cfg, r, opts...)

	return srv
}
