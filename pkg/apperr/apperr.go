// Package apperr defines typed application errors that map to gRPC status codes
// (and, through the gateway, to HTTP status codes) with a stable machine-readable reason.
package apperr

import (
	"errors"
	"fmt"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Error is a domain error.
type Error struct {
	Code   codes.Code
	Reason string // e.g. INSUFFICIENT_BALANCE
	Msg    string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Reason, e.Msg) }

// GRPCStatus lets grpc-go convert the error automatically.
func (e *Error) GRPCStatus() *status.Status {
	st := status.New(e.Code, e.Msg)
	if ds, err := st.WithDetails(&errdetails.ErrorInfo{Reason: e.Reason, Domain: "simple-securities"}); err == nil {
		return ds
	}
	return st
}

func newErr(c codes.Code, reason, format string, args ...any) *Error {
	return &Error{Code: c, Reason: reason, Msg: fmt.Sprintf(format, args...)}
}

func Invalid(reason, format string, args ...any) *Error {
	return newErr(codes.InvalidArgument, reason, format, args...)
}
func NotFound(reason, format string, args ...any) *Error {
	return newErr(codes.NotFound, reason, format, args...)
}
func Conflict(reason, format string, args ...any) *Error {
	return newErr(codes.AlreadyExists, reason, format, args...)
}
func Precondition(reason, format string, args ...any) *Error {
	return newErr(codes.FailedPrecondition, reason, format, args...)
}
func Unauthenticated(reason, format string, args ...any) *Error {
	return newErr(codes.Unauthenticated, reason, format, args...)
}
func Forbidden(reason, format string, args ...any) *Error {
	return newErr(codes.PermissionDenied, reason, format, args...)
}

// Reason extracts the reason from an apperr or from a gRPC status carrying ErrorInfo.
func Reason(err error) string {
	var ae *Error
	if errors.As(err, &ae) {
		return ae.Reason
	}
	if st, ok := status.FromError(err); ok {
		for _, d := range st.Details() {
			if info, ok := d.(*errdetails.ErrorInfo); ok {
				return info.Reason
			}
		}
		return st.Code().String()
	}
	return "INTERNAL"
}
