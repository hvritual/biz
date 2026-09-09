package enforcement

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
	"sync"
)

type outcomeKey struct{}
type outcome struct {
	mu      sync.Mutex
	id      string
	failure *Failure
}

func ensureOutcome(ctx context.Context) context.Context {
	if _, ok := ctx.Value(outcomeKey{}).(*outcome); ok {
		return ctx
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("commercial: correlation entropy unavailable")
	}
	return context.WithValue(ctx, outcomeKey{}, &outcome{id: hex.EncodeToString(b[:])})
}
func correlation(ctx context.Context) string {
	if o, ok := ctx.Value(outcomeKey{}).(*outcome); ok {
		return o.id
	}
	return ""
}
func remember(ctx context.Context, f *Failure) {
	if o, ok := ctx.Value(outcomeKey{}).(*outcome); ok {
		o.mu.Lock()
		defer o.mu.Unlock()
		if o.failure == nil {
			o.failure = f
		}
	}
}
func failed(ctx context.Context) *Failure {
	if o, ok := ctx.Value(outcomeKey{}).(*outcome); ok {
		o.mu.Lock()
		defer o.mu.Unlock()
		return o.failure
	}
	return nil
}

// Error projection only: does not authenticate, bind requests or route around
// generated transports. Native IAM errors remain untouched.
func HTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := ensureOutcome(r.Context())
		next.ServeHTTP(&decisionWriter{ResponseWriter: w, ctx: ctx}, r.WithContext(ctx))
	})
}

type decisionWriter struct {
	http.ResponseWriter
	ctx               context.Context
	written, suppress bool
}

func (w *decisionWriter) WriteHeader(code int) {
	if w.written {
		return
	}
	w.written = true
	if f := failed(w.ctx); f != nil {
		w.suppress = true
		code = http.StatusForbidden
		if f.Unavailable {
			code = http.StatusServiceUnavailable
		}
		w.Header().Del("Content-Length")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.ResponseWriter.WriteHeader(code)
		_ = json.NewEncoder(w.ResponseWriter).Encode(f)
		return
	}
	w.ResponseWriter.WriteHeader(code)
}
func (w *decisionWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	if w.suppress {
		return len(b), nil
	}
	return w.ResponseWriter.Write(b)
}
func RPC() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		ctx = ensureOutcome(ctx)
		response, err := next(ctx, req)
		if f := failed(ctx); f != nil {
			code := codes.PermissionDenied
			if f.Unavailable {
				code = codes.Unavailable
			}
			s := status.New(code, f.Code)
			if detailed, e := s.WithDetails(&errdetails.ErrorInfo{Reason: f.Code, Domain: "biz.commercial", Metadata: map[string]string{"operation": f.Operation, "correlation_id": f.CorrelationID}}); e == nil {
				s = detailed
			}
			return nil, s.Err()
		}
		return response, err
	}
}
