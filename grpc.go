package servers

import (
	"context"
	"errors"
	"fmt"

	"github.com/bool64/ctxd"
	"github.com/bool64/zapctxd"
	grpcLogging "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// ErrGRPCStart is returned when an error occurs the GRPC server start.
var ErrGRPCStart = errors.New("start grpc server")

// GRPCRegisterService is an interface for a grpc service that provides registration.
type GRPCRegisterService interface {
	RegisterService(s grpc.ServiceRegistrar)
}

// GRPCRegisterServiceFunc is the function to register service to a grpc.
type GRPCRegisterServiceFunc func(s grpc.ServiceRegistrar)

// WithRegisterService registers a service.
// Apply to GRPC server instances.
func WithRegisterService(r GRPCRegisterService) Option {
	return func(srv any) {
		s, ok := srv.(*GRPC)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		s.options.registerServices = append(s.options.registerServices, r.RegisterService)
	}
}

// WithReflection sets service to implement reflection. Mainly used in dev.
// Apply to GRPC server instances.
func WithReflection() Option {
	return func(srv any) {
		s, ok := srv.(*GRPC)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		s.options.reflection = true
	}
}

// WithChainUnaryInterceptor sets the server interceptors for unary.
// Apply to GRPC server instances.
func WithChainUnaryInterceptor(interceptors ...grpc.UnaryServerInterceptor) Option {
	return WithServerOption(grpc.ChainUnaryInterceptor(interceptors...))
}

// WithChainStreamInterceptor sets the server interceptors for stream.
// Apply to GRPC server instances.
func WithChainStreamInterceptor(interceptors ...grpc.StreamServerInterceptor) Option {
	return WithServerOption(grpc.ChainStreamInterceptor(interceptors...))
}

// WithServerOption sets the options for the grpc server.
// Apply to GRPC server instances.
func WithServerOption(opts ...grpc.ServerOption) Option {
	return func(srv any) {
		s, ok := srv.(*GRPC)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		s.options.serverOpts = append(s.options.serverOpts, opts...)
	}
}

// GRPCObserver interface implemented by anything wants to append observer in the server thro UnaryServerInterceptor and StreamServerInterceptor.
type GRPCObserver interface {
	UnaryServerInterceptor() func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error)
	StreamServerInterceptor() func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error
}

// WithGRPCObserver sets the GRPCObserver collector manager, used to append UnaryServerInterceptor and StreamServerInterceptor
// Apply to GRPC server instances.
func WithGRPCObserver(observer GRPCObserver) Option {
	return func(srv any) {
		s, ok := srv.(*GRPC)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		if s.options.observer != nil {
			return
		}

		s.options.observer = observer

		s.options.serverOpts = append(
			s.options.serverOpts,
			grpc.ChainUnaryInterceptor(observer.UnaryServerInterceptor()),
			grpc.ChainStreamInterceptor(observer.StreamServerInterceptor()),
		)
	}
}

// WithGrpcHealthCheck sets service to enable health check.
// Apply to GRPC server instances.
func WithGrpcHealthCheck() Option {
	return func(srv any) {
		s, ok := srv.(*GRPC)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		s.options.healthCheck = true
	}
}

// WithLogger sets service to use logger.
// Apply to GRPC server instances.
func WithLogger(logger ctxd.Logger) Option {
	return func(srv any) {
		s, ok := srv.(*GRPC)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		s.options.logger = logger

		s.options.serverOpts = append(
			s.options.serverOpts,
			grpc.ChainUnaryInterceptor(grpcLogging.UnaryServerInterceptor(grpcInterceptorLogger(s.options.logger))),
			grpc.ChainStreamInterceptor(grpcLogging.StreamServerInterceptor(grpcInterceptorLogger(s.options.logger))),
		)
	}
}

// grpcInterceptorLogger adapts zapctxd logger to interceptor logger.
func grpcInterceptorLogger(l ctxd.Logger) grpcLogging.Logger {
	return grpcLogging.LoggerFunc(func(ctx context.Context, lvl grpcLogging.Level, msg string, fields ...any) {
		ctx = ctxd.AddFields(ctx, fields...)

		if sl, ok := l.(interface{ SkipCaller() *zapctxd.Logger }); ok {
			l = sl.SkipCaller()
		}

		switch lvl {
		case grpcLogging.LevelDebug:
			l.Debug(ctx, msg)
		case grpcLogging.LevelInfo:
			l.Info(ctx, msg)
		case grpcLogging.LevelWarn:
			l.Warn(ctx, msg)
		case grpcLogging.LevelError:
			l.Error(ctx, msg)
		default:
			panic(fmt.Sprintf("unknown level %v", lvl))
		}
	})
}

// NewGRPC initiates a new wrapped grpc server.
func NewGRPC(config Config, opts ...Option) *GRPC {
	srv := &GRPC{}

	srv.Server = NewServer(config, opts...)

	for _, o := range opts {
		o(srv)
	}

	// Init GRPC Server.
	grpcSrv := grpc.NewServer(srv.options.serverOpts...)

	for _, register := range srv.options.registerServices {
		register(grpcSrv)
	}

	// Make the service reflective so that APIs can be discovered.
	if srv.options.reflection {
		reflection.Register(grpcSrv)
	}

	// Init GRPC Health Server.
	if srv.options.healthCheck {
		grpcHealthServer := health.NewServer()
		grpcHealthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
		grpcHealthServer.SetServingStatus(config.Name, healthpb.HealthCheckResponse_SERVING)
		healthpb.RegisterHealthServer(grpcSrv, grpcHealthServer)

		srv.grpcHealthServer = grpcHealthServer
	}

	srv.grpcServer = grpcSrv

	return srv
}

type grpcOptions struct {
	registerServices []GRPCRegisterServiceFunc
	serverOpts       []grpc.ServerOption

	reflection bool

	observer GRPCObserver
	logger   ctxd.Logger

	healthCheck bool
}

// GRPC is a listening grpc server instance.
type GRPC struct {
	*Server

	options grpcOptions

	grpcServer       *grpc.Server
	grpcHealthServer *health.Server
}

// Start starts serving the GRPC server.
func (srv *GRPC) Start() error {
	if err := srv.Server.Start(); err != nil {
		return err
	}

	if err := srv.serve(srv.grpcServer, srv.listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return fmt.Errorf("%w: %v addr %s", ErrGRPCStart, err, srv.listener.Addr().String()) //nolint:errorlint
	}

	return nil
}

// Stop gracefully shuts down the GRPC server.
func (srv *GRPC) Stop() {
	srv.grpcServer.GracefulStop()

	srv.Server.Stop()
}
