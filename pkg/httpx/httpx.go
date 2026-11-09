// Package httpx provides the HTTP side of every service: the grpc-gateway mux (so each service is
// reachable over HTTP/JSON as well as gRPC), health/readiness/metrics endpoints, and a protojson
// client used for service-to-service calls over HTTP.
package httpx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var (
	marshal   = protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}
	unmarshal = protojson.UnmarshalOptions{DiscardUnknown: true}
)

// NewGatewayMux returns a grpc-gateway mux with snake_case JSON.
func NewGatewayMux() *runtime.ServeMux {
	return runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard,
		&runtime.JSONPb{MarshalOptions: marshal, UnmarshalOptions: unmarshal}))
}

// ReadyFunc reports readiness (e.g. DB ping).
type ReadyFunc func(ctx context.Context) error

// ServiceHandler mounts the gateway mux plus /healthz, /readyz and /metrics.
func ServiceHandler(api http.Handler, ready ReadyFunc) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if ready != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := ready(ctx); err != nil {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("/metrics", promhttp.Handler())
	if api != nil {
		mux.Handle("/", api)
	}
	return mux
}

// Serve runs an HTTP server until ctx is cancelled and shuts it down gracefully.
func Serve(ctx context.Context, addr string, h http.Handler, log *zap.Logger) error {
	srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 120 * time.Second}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.Info("HTTP server listening", zap.String("addr", addr))
	select {
	case <-ctx.Done():
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(sctx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// Client calls another service's grpc-gateway endpoints with protojson.
// Errors are converted back into gRPC status errors so callers handle both transports the same way.
type Client struct {
	base string
	hc   *http.Client
}

// NewClient returns a client for base (e.g. "http://wallet:8102").
func NewClient(base string) *Client {
	return &Client{base: base, hc: &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{
		MaxIdleConns: 200, MaxIdleConnsPerHost: 100, IdleConnTimeout: 90 * time.Second,
	}}}
}

// Do performs method path?query with an optional protobuf body and decodes into out.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body, out proto.Message) error {
	u := c.base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var rd io.Reader
	if body != nil {
		b, err := marshal.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return grpcstatus.Error(codes.Unavailable, err.Error())
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return grpcstatus.Error(codes.Unavailable, err.Error())
	}
	if resp.StatusCode >= 300 {
		var st status.Status
		if uerr := unmarshal.Unmarshal(raw, &st); uerr == nil && st.Code != 0 {
			return grpcstatus.ErrorProto(&st)
		}
		return grpcstatus.Error(codes.Unknown, fmt.Sprintf("http %d: %s", resp.StatusCode, raw))
	}
	if out != nil {
		return unmarshal.Unmarshal(raw, out)
	}
	return nil
}
