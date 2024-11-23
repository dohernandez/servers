package servers

import (
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

type grpcRestOptions struct {
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

// WithHandlers sets the options for custom path handlers to the mux server.
// Apply to GRPCRest server instances.
func WithHandlers(handlers ...func(mux *runtime.ServeMux) error) Option {
	return func(srv any) {
		s, ok := srv.(*GRPCRest)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		s.options.register = append(s.options.register, handlers...)
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
