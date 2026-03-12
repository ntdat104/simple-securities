package di

import (
	"simple-securities/internal/user/infras/repo"

	"github.com/google/wire"
)

var RepositorySet = wire.NewSet(
	repo.NewUserRepo,
	repo.NewUserHistoryRepo,
)
