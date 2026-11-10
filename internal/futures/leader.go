package futures

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"simple-securities/pkg/database"
)

const (
	leaderRetry     = 3 * time.Second
	leaderKeepAlive = 5 * time.Second
	riskTick        = time.Second
)

// leaderState is the in-memory state of the leader loops (rebuilt on every election).
type leaderState struct {
	marks    map[string]*markState // mark loop only
	settled  map[string]time.Time  // mark loop only: last settled funding boundary per symbol
	cooldown map[posKey]time.Time  // liquidation loop only: last liquidation order per position
}

func newLeaderState() *leaderState {
	return &leaderState{marks: map[string]*markState{}, settled: map[string]time.Time{},
		cooldown: map[posKey]time.Time{}}
}

// RunLeader runs the leader-only loops (mark price, funding, liquidation) while this instance
// holds a Postgres session advisory lock on a dedicated connection. Standbys retry every few seconds.
func (s *Service) RunLeader(ctx context.Context) error {
	for ctx.Err() == nil {
		if err := s.lead(ctx); err != nil && ctx.Err() == nil {
			s.log.Warn("futures leadership lost", zap.Error(err))
		}
		select {
		case <-ctx.Done():
		case <-time.After(leaderRetry):
		}
	}
	return nil
}

func (s *Service) lead(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	key := database.LockKey("leader:" + ServiceName)
	var ok bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return nil // another instance leads
	}
	defer func() {
		uctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(uctx, `SELECT pg_advisory_unlock($1)`, key)
	}()
	s.log.Info("futures leader elected: running mark price, funding and liquidation loops")

	st := newLeaderState()
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return s.runMarkLoop(gctx, st) })
	g.Go(func() error { return s.runLiquidationLoop(gctx, st) })
	g.Go(func() error {
		t := time.NewTicker(leaderKeepAlive)
		defer t.Stop()
		for {
			select {
			case <-gctx.Done():
				return nil
			case <-t.C:
				if err := conn.PingContext(gctx); err != nil && gctx.Err() == nil {
					return fmt.Errorf("leader connection lost: %w", err)
				}
			}
		}
	})
	return g.Wait()
}

// runMarkLoop computes and publishes mark prices every second and settles funding at the
// interval boundaries (same goroutine, so the premium window needs no locking).
func (s *Service) runMarkLoop(ctx context.Context, st *leaderState) error {
	t := time.NewTicker(riskTick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			now := s.now()
			s.computeMarks(ctx, st, now)
			s.settleDueFunding(ctx, st, now)
		}
	}
}

// runLiquidationLoop scans positions every second.
func (s *Service) runLiquidationLoop(ctx context.Context, st *leaderState) error {
	t := time.NewTicker(riskTick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			if err := s.scanLiquidations(ctx, st); err != nil && ctx.Err() == nil {
				s.log.Warn("liquidation scan failed", zap.Error(err))
			}
		}
	}
}
