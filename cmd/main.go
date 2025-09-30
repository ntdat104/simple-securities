package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"simple-securities/api/middleware"
	"simple-securities/config"
	"simple-securities/infra/repository"
	"simple-securities/pkg/logger"

	"go.uber.org/zap"

	http2 "simple-securities/api/http"
)

const (
	DefaultMetricsAddr = ":9090"
)

func main() {
	// Initialize configuration
	config.Init("./config", "config")

	// Initialize logging
	logger.Init()
	logger.Logger.Info("Application starting",
		zap.String("service", config.GlobalConfig.App.Name),
		zap.String("env", string(config.GlobalConfig.Env)))

	// Initialize metrics collection system
	middleware.InitializeMetrics()
	logger.Logger.Info("Metrics collection system initialized")

	// Initializing mysql
	db, err := repository.NewSqliteConn()
	if err != nil {
		logger.Logger.Fatal("Failed to initialize mysql",
			zap.Error(err))
	}
	logger.Logger.Info("Mysql initialized successfully")

	// Initializing redis
	rdb, err := repository.NewRedisConn()
	if err != nil {
		logger.Logger.Fatal("Failed to initialize redis",
			zap.Error(err))
	}
	logger.Logger.Info("Redis initialized successfully")

	// Start metrics server in a separate goroutine if enabled
	if config.GlobalConfig.MetricsServer != nil && config.GlobalConfig.MetricsServer.Enabled {
		metricsAddr := config.GlobalConfig.MetricsServer.Addr
		if metricsAddr == "" {
			metricsAddr = DefaultMetricsAddr
		}
		go func() {
			if err := middleware.StartMetricsServer(metricsAddr); err != nil {
				logger.Logger.Error("Failed to start metrics server", zap.Error(err))
			}
		}()
		logger.Logger.Info("Metrics server started", zap.String("address", metricsAddr))
	} else {
		logger.Logger.Info("Metrics server is disabled")
	}

	// Create context and cancel function
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	router := http2.NewServerRoute(db, rdb)

	srv := &http.Server{
		Addr:    config.GlobalConfig.HTTPServer.Addr,
		Handler: router,
	}

	// Run server in a goroutine
	go func() {
		log.Printf("%v started on http://%v%v", config.GlobalConfig.App.Name, "localhost", config.GlobalConfig.HTTPServer.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf(config.GlobalConfig.App.Name+" failed to start: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exiting")
}
