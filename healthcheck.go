package servers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// NewHealthCheck is a listening HTTP server instance with the endpoints "/" and "/health" mainly use for
// livenessProbe and readinessProbe on Kubernetes cluster.
func NewHealthCheck(cfg Config, h http.Handler) *REST {
	r := chi.NewRouter()

	r.Method(http.MethodGet, "/", NewRestRootHandler(cfg.Name))

	r.Method(http.MethodGet, "/health", h)

	r.Method(http.MethodGet, "/version", NewRestVersionHandler())

	return NewREST(
		&Config{
			Name: cfg.Name,
			Host: cfg.Host,
			Port: cfg.Port,
		},
		r,
		WithAddrAssigned(),
	)
}
