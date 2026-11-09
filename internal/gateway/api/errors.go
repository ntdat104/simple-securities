// Package api holds the public contract of the gateway shared by REST and WebSocket: response
// shapes, proto -> JSON mapping, error mapping and the order operations.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"simple-securities/pkg/apperr"
)

// ReasonRateLimited is returned when a rate limit is exceeded.
const ReasonRateLimited = "RATE_LIMITED"

// ErrorBody is the JSON error shape: {"code": "...", "msg": "..."}.
type ErrorBody struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

// RateLimitError signals an exceeded limit (HTTP 429 + Retry-After).
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string { return "too many requests" }

// GRPCStatus maps the error to ResourceExhausted with the RATE_LIMITED reason.
func (e *RateLimitError) GRPCStatus() *status.Status {
	st := status.New(codes.ResourceExhausted, e.Error())
	if ds, err := st.WithDetails(&errdetails.ErrorInfo{Reason: ReasonRateLimited, Domain: "simple-securities"}); err == nil {
		return ds
	}
	return st
}

// RetryAfterSeconds rounds the wait up to whole seconds (at least 1).
func (e *RateLimitError) RetryAfterSeconds() int {
	s := int((e.RetryAfter + time.Second - 1) / time.Second)
	return max(s, 1)
}

// HTTPStatus maps a gRPC code to the HTTP status of the public API.
func HTTPStatus(c codes.Code) int {
	switch c {
	case codes.OK:
		return http.StatusOK
	case codes.InvalidArgument, codes.FailedPrecondition:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// Internal reports whether the error is a server-side failure whose details must not leak.
func Internal(err error) bool {
	switch status.Code(err) {
	case codes.InvalidArgument, codes.FailedPrecondition, codes.Unauthenticated, codes.PermissionDenied,
		codes.NotFound, codes.AlreadyExists, codes.ResourceExhausted, codes.OutOfRange, codes.Aborted:
		return false
	}
	return true
}

// FromError converts any error into its HTTP status and public body. Internal details are hidden.
func FromError(err error) (int, ErrorBody) {
	st, ok := status.FromError(err)
	if !ok {
		return http.StatusInternalServerError, ErrorBody{Code: "INTERNAL", Msg: "internal error"}
	}
	body := ErrorBody{Code: apperr.Reason(err), Msg: st.Message()}
	switch st.Code() {
	case codes.Unavailable:
		body.Msg = "service temporarily unavailable"
	case codes.DeadlineExceeded:
		body.Msg = "upstream timeout"
	default:
		if Internal(err) {
			body.Code, body.Msg = "INTERNAL", "internal error"
		}
	}
	return HTTPStatus(st.Code()), body
}

// WriteJSON writes v with the given status.
func WriteJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes the mapped error (and Retry-After for rate limits).
func WriteError(w http.ResponseWriter, err error) {
	var rl *RateLimitError
	if errors.As(err, &rl) {
		w.Header().Set("Retry-After", strconv.Itoa(rl.RetryAfterSeconds()))
	}
	code, body := FromError(err)
	WriteJSON(w, code, body)
}
