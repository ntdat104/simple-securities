package middleware

import (
	"simple-securities/pkg/uuid"

	"github.com/gin-gonic/gin"
)

const (
	// RequestIDHeader is the header key for request ID
	RequestIDHeader = "X-Request-ID"
)

// RequestID is a middleware that injects a request ID into the context of each request
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get request ID from header or generate a new one
		requestID := c.GetHeader(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.NewGoogleUUID()
		}

		// Set request ID to header
		c.Writer.Header().Set(RequestIDHeader, requestID)
		c.Set(RequestIDHeader, requestID)

		c.Next()
	}
}
