package http

import (
	"net/http"
	"time"

	"simple-securities/api/http/app_context"
	"simple-securities/api/http/validator/custom"
	"simple-securities/application/service"
	"simple-securities/config"
	"simple-securities/infra/repo"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"

	httpMiddleware "simple-securities/api/http/middleware"
	metricsMiddleware "simple-securities/api/middleware"
)

func NewServerRoute(db *sqlx.DB, rdb *redis.Client) *gin.Engine {
	if config.GlobalConfig.Env.IsProd() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Register custom validators
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		custom.RegisterValidators(v)
	}

	// Apply middleware
	router.Use(gin.Recovery())
	router.Use(httpMiddleware.Cors())
	router.Use(httpMiddleware.RequestID())              // Add request ID middleware
	router.Use(httpMiddleware.AppContextMiddleware())   // Add app context middleware
	router.Use(httpMiddleware.RequestLogger())          // Add request logging middleware
	router.Use(httpMiddleware.ErrorHandlerMiddleware()) // Add unified error handling middleware
	// router.Use(httpMiddleware.ZapLoggerWithBody())

	// Add metrics middleware for each handler
	router.Use(func(c *gin.Context) {
		// Use the path as a label for the metrics
		handlerName := c.FullPath()
		if handlerName == "" {
			handlerName = "unknown"
		}

		// Record the start time
		start := time.Now()

		// Process the request
		c.Next()

		// Record metrics after request is processed
		duration := time.Since(start)
		statusCode := c.Writer.Status()

		// Record request metrics
		metricsMiddleware.RecordHTTPMetrics(handlerName, c.Request.Method, statusCode, duration)
	})

	// Health check
	router.GET("/ping", func(c *gin.Context) {
		app_context.Get(c).Logger.Info("Ping request received")
		c.String(http.StatusOK, "pong")
	})

	// Configure CORS
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// example
	exampleRepo := repo.NewExampleRepo(db)
	exampleCacheRepo := repo.NewExampleCacheRepo(rdb)
	exampleService := service.NewExampleService(exampleRepo, exampleCacheRepo)
	NewExampleHandler(router, exampleService)

	// system
	systemService := service.NewSystemService()
	NewSystemHandler(router, systemService)

	return router
}
