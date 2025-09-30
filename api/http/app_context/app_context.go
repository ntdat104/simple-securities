package app_context

import (
	"context"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AppContext struct {
	Ctx    context.Context
	Logger *zap.Logger
}

func (a *AppContext) Cleanup() {
	// Perform any necessary cleanup here
}

func Get(c *gin.Context) *AppContext {
	val, exists := c.Get("app_context")
	if !exists {
		c.JSON(500, gin.H{"error": "app_context not found in gin.Context — did you forget the middleware?"})
		return nil
	}
	value, ok := val.(*AppContext)
	if !ok {
		c.JSON(500, gin.H{"error": "app_context has wrong type in gin.Context"})
		return nil
	}
	return value
}
