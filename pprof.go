package servers

import (
	"net/http"
	"net/http/pprof"

	"github.com/go-chi/chi/v5"
)

// NewPProf creates a new REST service dedicated to serving pprof routes.
// This service is designed as an alternative to the default pprof handler, eliminating the necessity to rely on `http.DefaultServeMux`.
func NewPProf(cfg Config) *REST {
	r := chi.NewRouter()

	r.Method(http.MethodGet, "/", NewRestRootHandler(cfg.Name))

	r.HandleFunc("/debug/pprof/*", pprof.Index)

	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)

	r.Method(http.MethodGet, "/version", NewRestVersionHandler())

	return NewREST(
		Config{
			Name: cfg.Name,
			Host: cfg.Host,
			Port: cfg.Port,
		},
		r,
		WithAddrAssigned(),
	)
}
