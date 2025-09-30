package middleware

import (
	"bytes"
	"io"
	"time"

	"simple-securities/config"
	"simple-securities/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b) // capture response body
	return w.ResponseWriter.Write(b)
}

func ZapLoggerWithBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Clone the request body
		var reqBody []byte
		if c.Request.Body != nil {
			reqBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(reqBody)) // restore
		}

		// Wrap the response writer to capture response body
		blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = blw

		// Process request
		c.Next()

		end := time.Now()
		duration := end.Sub(start)
		formattedStart := start.Format("2006-01-02 15:04:05.000")
		formattedEnd := end.Format("2006-01-02 15:04:05.000")

		fields := []zap.Field{
			zap.String("request_id", c.GetHeader(RequestIDHeader)),
			zap.String("app_name", config.GlobalConfig.App.Name),
			zap.String("app_version", config.GlobalConfig.App.Version),
			zap.String("start_time", formattedStart),
			zap.String("end_time", formattedEnd),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("x_api_key", c.GetHeader("X-Api-Key")),
			zap.String("x_api_secret", c.GetHeader("X-Api-Secret")),
			zap.String("access_token", c.GetHeader("Authorization")),
			zap.String("signature", c.GetHeader("Signature")),
			zap.Int("status", c.Writer.Status()),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.String("latency", duration.String()),
			zap.ByteString("request_body", reqBody),
			zap.ByteString("response_body", blw.body.Bytes()),
		}

		// Write structured log
		logger.Logger.Info("HTTP request", fields...)
	}
}
