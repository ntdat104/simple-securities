package di

import (
	"simple-securities/internal/user/scheduler"

	"github.com/google/wire"
)

var SchedulerSet = wire.NewSet(
	scheduler.NewUserCronScheduler,
)
