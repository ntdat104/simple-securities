package constant

import (
	"simple-securities/pkg/registry"
	"time"
)

var (
	UserProfile = registry.NewDefaultCache("user:v1:profile")
	UserCash    = registry.NewDefaultCache("user:v1:cash")
	UserHistory = registry.NewCache("user:v1:history", 2*time.Minute, 4*time.Minute)
)
