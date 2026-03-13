package scheduler

import (
	"context"
	"simple-securities/internal/user/domain/repo"

	"go.uber.org/zap"
)

// cleanExpiredTokensJob removes refresh tokens that have expired.
// Implements cron.Job via the Run() method.
type cleanExpiredTokensJob struct {
	userRepo repo.IUserRepo
	log      *zap.Logger
}

func newCleanExpiredTokensJob(userRepo repo.IUserRepo, log *zap.Logger) *cleanExpiredTokensJob {
	return &cleanExpiredTokensJob{userRepo: userRepo, log: log}
}

func (j *cleanExpiredTokensJob) Run() {
	ctx := context.Background()

	total, err := j.userRepo.CountTotal(ctx)
	if err != nil {
		j.log.Error("cleanExpiredTokensJob: failed to count users", zap.Error(err))
		return
	}

	// TODO: replace with actual expired-token cleanup logic.
	j.log.Info("cleanExpiredTokensJob: completed", zap.Int64("total_users", total))
}
