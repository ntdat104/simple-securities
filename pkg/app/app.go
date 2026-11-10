// Package app runs a service's long-lived components and shuts them down together.
package app

import (
	"context"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// Component blocks until ctx is cancelled (or fails).
type Component func(ctx context.Context) error

// Context returns a context cancelled on SIGINT/SIGTERM.
func Context() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}

// Run starts all components; the first failure cancels the others.
func Run(ctx context.Context, log *zap.Logger, comps ...Component) error {
	g, gctx := errgroup.WithContext(ctx)
	for _, c := range comps {
		g.Go(func() error { return c(gctx) })
	}
	err := g.Wait()
	if err != nil {
		log.Error("service stopped with error", zap.Error(err))
	} else {
		log.Info("service stopped gracefully")
	}
	return err
}
