package scheduler

import (
	"simple-securities/internal/user/domain/repo"
	"simple-securities/pkg/scheduler"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// UserCronScheduler holds all cron entries for the user service.
// It implements io.Closer so it can be registered with server.AddShutdownHook.
type UserCronScheduler struct {
	c   *cron.Cron
	log *zap.Logger
}

func NewUserCronScheduler(userRepo repo.IUserRepo, log *zap.Logger) *UserCronScheduler {
	c := scheduler.New(log)

	c.AddFunc("@every 0h0m5s", func() { log.Info("Every second") })

	c.AddJob("0 0 * * * *", newCleanExpiredTokensJob(userRepo, log)) // every hour

	return &UserCronScheduler{c: c, log: log}
}

func (s *UserCronScheduler) Start() {
	s.c.Start()
	s.log.Info("user cron scheduler started")
}

// Close gracefully waits for running jobs to finish before stopping.
func (s *UserCronScheduler) Close() error {
	ctx := s.c.Stop()
	<-ctx.Done()
	s.log.Info("user cron scheduler stopped")
	return nil
}
