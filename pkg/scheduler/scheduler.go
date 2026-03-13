package scheduler

import (
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// zapAdapter adapts *zap.Logger to the cron.Logger interface.
type zapAdapter struct {
	log *zap.Logger
}

func (z *zapAdapter) Info(msg string, keysAndValues ...any) {
	z.log.Info(msg, toZapFields(keysAndValues)...)
}

func (z *zapAdapter) Error(err error, msg string, keysAndValues ...any) {
	fields := append([]zap.Field{zap.Error(err)}, toZapFields(keysAndValues)...)
	z.log.Error(msg, fields...)
}

func toZapFields(kv []any) []zap.Field {
	fields := make([]zap.Field, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		k, _ := kv[i].(string)
		fields = append(fields, zap.Any(k, kv[i+1]))
	}
	return fields
}

// New returns a *cron.Cron configured with:
// - seconds precision (6-field spec)
// - zap-based logging
// - per-entry panic recovery
func New(log *zap.Logger) *cron.Cron {
	adapter := &zapAdapter{log: log}
	return cron.New(
		cron.WithSeconds(),
		cron.WithLogger(adapter),
		cron.WithChain(cron.Recover(adapter)),
	)
}
