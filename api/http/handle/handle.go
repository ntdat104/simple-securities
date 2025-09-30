package handle

import (
	"net/http"
	"reflect"
	"strconv"
	"time"

	"simple-securities/api/error_code"
	"simple-securities/api/http/middleware"
	"simple-securities/api/http/paginate"
	"simple-securities/pkg/logger"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Ctx *gin.Context
}

type Meta struct {
	RequestID string `json:"request_id"`
	Timestamp int64  `json:"timestamp"`
	Datetime  string `json:"datetime"`
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Token     string `json:"token,omitempty"`
	Total     int    `json:"total,omitempty"`
	Page      int    `json:"page,omitempty"`
	PageSize  int    `json:"page_size,omitempty"`
	DocRef    string `json:"doc_ref,omitempty"`
}

// StandardResponse defines the standard API response structure
type StandardResponse struct {
	Meta   Meta     `json:"meta"`
	Data   any      `json:"data,omitempty"`
	Errors []string `json:"errors,omitempty"`
}

func NewResponse(ctx *gin.Context) *Response {
	return &Response{Ctx: ctx}
}

func (r *Response) ToSuccess() {
	now := time.Now()
	r.Ctx.JSON(http.StatusOK, StandardResponse{
		Meta: Meta{
			RequestID: r.Ctx.GetString(middleware.RequestIDHeader),
			Timestamp: now.UnixMilli(),
			Datetime:  now.Format("2006-01-02 15:04:05"),
			Code:      error_code.SuccessCode,
			Message:   "success",
		},
	})
}

func (r *Response) ToResponse(data interface{}) {
	now := time.Now()
	r.Ctx.JSON(http.StatusOK, StandardResponse{
		Meta: Meta{
			RequestID: r.Ctx.GetString(middleware.RequestIDHeader),
			Timestamp: now.UnixMilli(),
			Datetime:  now.Format("2006-01-02 15:04:05"),
			Code:      error_code.SuccessCode,
			Message:   "success",
		},
		Data: data,
	})
}

func (r *Response) ToResponseList(data interface{}, totalRows int) {
	now := time.Now()
	r.Ctx.JSON(http.StatusOK, StandardResponse{
		Meta: Meta{
			RequestID: r.Ctx.GetString(middleware.RequestIDHeader),
			Timestamp: now.UnixMilli(),
			Datetime:  now.Format("2006-01-02 15:04:05"),
			Code:      error_code.SuccessCode,
			Message:   "success",
			Page:      paginate.GetPage(r.Ctx),
			PageSize:  paginate.GetPageSize(r.Ctx),
			Total:     totalRows,
		},
		Data: data,
	})
}

func (r *Response) ToErrorResponse(err *error_code.Error) {
	now := time.Now()
	r.Ctx.JSON(err.StatusCode(), StandardResponse{
		Meta: Meta{
			RequestID: r.Ctx.GetString(middleware.RequestIDHeader),
			Timestamp: now.UnixMilli(),
			Datetime:  now.Format("2006-01-02 15:04:05"),
			Code:      err.Code,
			Message:   err.Msg,
			DocRef:    err.DocRef,
		},
		Errors: err.Details,
	})
}

// Success returns a success response
func Success(c *gin.Context, data any) {
	now := time.Now()
	c.JSON(http.StatusOK, StandardResponse{
		Meta: Meta{
			RequestID: c.GetString(middleware.RequestIDHeader),
			Timestamp: now.UnixMilli(),
			Datetime:  now.Format("2006-01-02 15:04:05"),
			Code:      error_code.SuccessCode,
			Message:   "success",
		},
		Data: data,
	})
}

// Error unified error handling
func Error(c *gin.Context, err error) {
	now := time.Now()

	// Handle API error codes
	if apiErr, ok := err.(*error_code.Error); ok {
		c.JSON(apiErr.StatusCode(), StandardResponse{
			Meta: Meta{
				RequestID: c.GetString(middleware.RequestIDHeader),
				Timestamp: now.UnixMilli(),
				Datetime:  now.Format("2006-01-02 15:04:05"),
				Code:      apiErr.Code,
				Message:   apiErr.Msg,
				DocRef:    apiErr.DocRef,
			},
			Errors: apiErr.Details,
		})
		return
	}

	// Log unexpected errors
	logger.SugaredLogger.Errorf("Unexpected error: %v", err)

	// Default error response
	c.JSON(http.StatusInternalServerError, StandardResponse{
		Meta: Meta{
			RequestID: c.GetString(middleware.RequestIDHeader),
			Timestamp: now.UnixMilli(),
			Datetime:  now.Format("2006-01-02 15:04:05"),
			Code:      error_code.ServerErrorCode,
			Message:   "Internal server error",
		},
	})
}

// GetQueryInt gets an integer from query parameters with a default value
func GetQueryInt(c *gin.Context, key string, defaultValue int) int {
	value, exists := c.GetQuery(key)
	if !exists {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intValue
}

// GetQueryString gets a string from query parameters with a default value
func GetQueryString(c *gin.Context, key string, defaultValue string) string {
	value, exists := c.GetQuery(key)
	if !exists {
		return defaultValue
	}
	return value
}

// IsNil checks if an interface is nil or its underlying value is nil
func IsNil(i any) bool {
	if i == nil {
		return true
	}
	switch reflect.TypeOf(i).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
		return reflect.ValueOf(i).IsNil()
	}
	return false
}
