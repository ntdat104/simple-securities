package middleware

import (
	"simple-securities/api/http/app_context"
	"simple-securities/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func AppContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		appCtx := &app_context.AppContext{
			Ctx:    c.Request.Context(),
			Logger: logger.Logger.With(zap.String("request_id", c.GetString("X-Request-ID"))),
		}
		defer appCtx.Cleanup()
		c.Set("app_context", appCtx)
		c.Next()
	}
}
