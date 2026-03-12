package di

import (
	"simple-securities/internal/user/application/service"

	"github.com/google/wire"
)

var ServiceSet = wire.NewSet(
	service.NewRegisterSvc,
	service.NewLoginSvc,
	service.NewGetUserProfileSvc,
	service.NewRefreshTokenSvc,
)
