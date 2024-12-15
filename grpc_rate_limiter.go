package servers

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

// getClientID extracts the client ID from the context metadata.
func getClientID(ctx context.Context) string {
	// Check if metadata is present in the context
	md, ok := metadata.FromIncomingContext(ctx)

	const unknown = "unknown-client"

	if !ok {
		return unknown // Default if no metadata is found
	}

	// Look for a specific key in the metadata (e.g., "client-id")
	if clientIDs, exists := md["client-id"]; exists && len(clientIDs) > 0 {
		return clientIDs[0] // Return the first client ID if available
	}

	return unknown // Default if "client-id" is not found
}

// WithRateLimiter sets the rate limiter for the gRPC server.
func WithRateLimiter(observer GRPCObserver) Option {
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

// PerClientRateLimiter is a gRPC interceptor that limits the number of requests per client.
type PerClientRateLimiter struct {
	clients map[string]*rate.Limiter
	mu      sync.Mutex
	rps     float64
	burst   int
}

// NewPerClientRateLimiter creates a new PerClientRateLimiter with the given requests per second and burst limit.
func NewPerClientRateLimiter(rps float64, burst int) *PerClientRateLimiter {
	return &PerClientRateLimiter{
		clients: make(map[string]*rate.Limiter),
		rps:     rps,
		burst:   burst,
	}
}

func (r *PerClientRateLimiter) getLimiter(clientID string) *rate.Limiter {
	r.mu.Lock()
	defer r.mu.Unlock()

	if limiter, exists := r.clients[clientID]; exists {
		return limiter
	}

	limiter := rate.NewLimiter(rate.Limit(r.rps), r.burst)
	r.clients[clientID] = limiter

	return limiter
}

// UnaryServerInterceptor returns a new unary server interceptor that limits the number of requests per client.
func (r *PerClientRateLimiter) UnaryServerInterceptor() func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		clientID := getClientID(ctx) // Extract client ID from metadata or IP
		limiter := r.getLimiter(clientID)

		if !limiter.Allow() {
			return nil, Error(codes.ResourceExhausted, "rate limit exceeded for client: %s", map[string]string{"client": clientID})
		}

		return handler(ctx, req)
	}
}

// GRPCRateLimiter interface implemented by anything wants to append rate limiter in the server
// thro UnaryServerInterceptor.
type GRPCRateLimiter interface {
	UnaryServerInterceptor() func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error)
}

// WithGRPCRateLimiter sets the GRPCRateLimiter, used to append UnaryServerInterceptor.
// Apply to GRPC server instances.
func WithGRPCRateLimiter(limiter GRPCRateLimiter) Option {
	return func(srv any) {
		s, ok := srv.(*GRPC)
		if !ok {
			// skip does not apply to this instance.
			return
		}

		if s.options.limiter != nil {
			return
		}

		s.options.limiter = limiter

		s.options.serverOpts = append(
			s.options.serverOpts,
			grpc.ChainUnaryInterceptor(limiter.UnaryServerInterceptor()),
		)
	}
}
