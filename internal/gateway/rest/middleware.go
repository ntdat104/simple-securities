package rest

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"

	"simple-securities/internal/gateway/api"
	"simple-securities/internal/gateway/ratelimit"
)

type ctxKey int

const (
	_ctxRequestID ctxKey = iota + 1
	_ctxLogInfo
)

// logInfo is filled by inner handlers (auth) and read by the access log.
type logInfo struct {
	userID string
}

var _httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "gateway_http_request_duration_seconds",
	Help:    "Gateway HTTP request latency by route and status.",
	Buckets: prometheus.DefBuckets,
}, []string{"method", "route", "code"})

// Middleware wraps a handler.
type Middleware func(http.Handler) http.Handler

// Chain applies mws so that the first one is the outermost.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// RequestIDFrom returns the request id stored in ctx.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(_ctxRequestID).(string)
	return id
}

func setLogUser(ctx context.Context, userID string) {
	if li, ok := ctx.Value(_ctxLogInfo).(*logInfo); ok {
		li.userID = userID
	}
}

// RequestID propagates a sane incoming X-Request-Id or generates one.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if !validRequestID(id) {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), _ctxRequestID, id)))
	})
}

func validRequestID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	return strings.IndexFunc(id, func(r rune) bool {
		return !(r == '-' || r == '_' || r == '.' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z'))
	}) < 0
}

// statusRecorder captures the status code; it keeps Hijack/Flush working for WebSocket and streaming.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijack not supported")
	}
	if s.status == 0 {
		s.status = http.StatusSwitchingProtocols
	}
	return hj.Hijack()
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// AccessLog logs every request and records latency metrics by route pattern.
func AccessLog(log *zap.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			li := &logInfo{}
			r = r.WithContext(context.WithValue(r.Context(), _ctxLogInfo, li))
			next.ServeHTTP(rec, r)
			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			d := time.Since(start)
			route := r.Pattern
			if route == "" {
				route = "unmatched"
			}
			_httpDuration.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Observe(d.Seconds())
			fields := []zap.Field{
				zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.Int("status", rec.status),
				zap.Duration("latency", d), zap.Int("bytes", rec.bytes),
				zap.String("request_id", RequestIDFrom(r.Context())), zap.String("remote", r.RemoteAddr),
			}
			if li.userID != "" {
				fields = append(fields, zap.String("user_id", li.userID))
			}
			switch {
			case rec.status >= 500:
				log.Warn("http", fields...)
			case isAPIPath(r.URL.Path) || r.URL.Path == "/ws":
				log.Info("http", fields...)
			default:
				log.Debug("http", fields...)
			}
		})
	}
}

// Recover turns panics into 500 responses.
func Recover(log *zap.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					if v == http.ErrAbortHandler {
						panic(v)
					}
					log.Error("panic in http handler", zap.Any("panic", v), zap.String("path", r.URL.Path),
						zap.String("request_id", RequestIDFrom(r.Context())), zap.ByteString("stack", debug.Stack()))
					api.WriteJSON(w, http.StatusInternalServerError, api.ErrorBody{Code: "INTERNAL", Msg: "internal error"})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders sets conservative browser security headers.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		if isAPIPath(r.URL.Path) {
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			h.Set("Cache-Control", "no-store")
		}
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// CORS allows the configured origins ("*" = any) and answers preflight requests.
func CORS(origins []string) Middleware {
	allowAll := len(origins) == 0 || slices.Contains(origins, "*")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				h := w.Header()
				switch {
				case allowAll:
					h.Set("Access-Control-Allow-Origin", "*")
				case slices.Contains(origins, origin):
					h.Set("Access-Control-Allow-Origin", origin)
					h.Add("Vary", "Origin")
				}
				h.Set("Access-Control-Expose-Headers", "X-Request-Id, Retry-After")
				if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
					h.Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
					h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-Id")
					h.Set("Access-Control-Max-Age", "600")
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// isAPIPath reports whether path belongs to the JSON APIs (/api spot, /fapi futures).
func isAPIPath(path string) bool {
	return strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/fapi/")
}

// IPRateLimit applies the per-IP limit to /api and /fapi routes.
func IPRateLimit(l api.Limiter, trustProxy bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isAPIPath(r.URL.Path) {
				if d := l.Allow(r.Context(), ratelimit.PerIP, api.ClientIP(r, trustProxy)); !d.Allowed {
					api.WriteError(w, &api.RateLimitError{RetryAfter: d.RetryAfter})
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
