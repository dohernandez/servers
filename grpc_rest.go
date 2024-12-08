package servers

import (
	"context"
	"google.golang.org/protobuf/proto"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

type grpcRestOptions struct {
	withDocEndpoint     bool
	withVersionEndpoint bool

	muxOpts  []runtime.ServeMuxOption
	register []func(mux *runtime.ServeMux) error
}

// WithServerMuxOption sets the options for the mux server.
// Apply to GRPCRest server instances.
func WithServerMuxOption(opts ...runtime.ServeMuxOption) Option {
	return func(srv any) {
		s, ok := srv.(*GRPCRest)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		s.options.muxOpts = append(s.options.muxOpts, opts...)
	}
}

// WithDocEndpoint sets the options for the mux server to serve API docs.
func WithDocEndpoint(serviceName, basePath, filepath string, json []byte) Option {
	return func(srv any) {
		s, ok := srv.(*GRPCRest)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		s.options.withDocEndpoint = true

		WithHandlers(NewRestAPIDocsHandlers(
			serviceName,
			basePath,
			filepath,
			json,
		))(s)
	}
}

// WithVersionEndpoint sets the options for the mux server to serve version.
func WithVersionEndpoint() Option {
	return func(srv any) {
		s, ok := srv.(*GRPCRest)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		s.options.withVersionEndpoint = true

		WithHandlers(map[string]http.Handler{
			"/version": NewRestVersionHandler(),
		})(s)
	}
}

// WithHandlers sets the options for custom path handlers to the mux server.
// Apply to GRPCRest server instances.
func WithHandlers(handlers map[string]http.Handler) Option {
	return func(srv any) {
		s, ok := srv.(*GRPCRest)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		for p, handler := range handlers {
			s.options.register = append(s.options.register, func(mux *runtime.ServeMux) error {
				err := mux.HandlePath(http.MethodGet, p, func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
					handler.ServeHTTP(w, r)
				})
				if err != nil {
					return err
				}

				return nil
			})
		}
	}
}

// WithResponseModifier sets the options for custom response modifier to the mux server.
func WithResponseModifier(modifier ...func(ctx context.Context, w http.ResponseWriter, _ proto.Message) error) Option {
	return func(srv any) {
		s, ok := srv.(*GRPCRest)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		opts := make([]runtime.ServeMuxOption, 0, len(modifier))

		for _, m := range modifier {
			opts = append(opts, runtime.WithForwardResponseOption(m))
		}

		WithServerMuxOption(opts...)(s)
	}
}

// GRPCRestRegisterService is an interface for a grpc rest service that provides registration.
type GRPCRestRegisterService interface {
	RegisterServiceHandler(mux *runtime.ServeMux) error
}

// WithRegisterServiceHandler registers a service handler to the mux server.
// Apply to GRPCRes server instances.
func WithRegisterServiceHandler(r GRPCRestRegisterService) Option {
	return func(srv any) {
		s, ok := srv.(*GRPCRest)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		s.options.register = append(s.options.register, r.RegisterServiceHandler)
	}
}

// GRPCRest is a listening grpc rest server instance.
type GRPCRest struct {
	*REST

	options grpcRestOptions
}

// NewGRPCRest initiates a new wrapped grpc rest server.
func NewGRPCRest(config Config, opts ...Option) (*GRPCRest, error) {
	srv := &GRPCRest{}

	for _, o := range opts {
		o(srv)
	}

	var links []any

	if srv.options.withDocEndpoint {
		links = append(links, "API Docs", "/docs")
	}

	if srv.options.withVersionEndpoint {
		links = append(links, "Version", "/version")
	}

	WithHandlers(map[string]http.Handler{
		"/": NewRestRootHandler(config.Name, links...),
	})(srv)

	WithResponseModifier(
		NewXhttpCodeResponseModifier(),
		CleanGrpcMetadataResponseModifier(),
	)(srv)

	// Init REST Server.
	mux := runtime.NewServeMux(srv.options.muxOpts...)

	for _, register := range srv.options.register {
		err := register(mux)
		if err != nil {
			return nil, err
		}
	}

	srv.REST = NewREST(config, mux, opts...)

	return srv, nil
}
