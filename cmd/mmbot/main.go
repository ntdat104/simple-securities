// Command mmbot is a market maker / liquidity simulator that trades through the public gateway.
package main

import (
	"strings"
	"time"

	"simple-securities/internal/mmbot"
	"simple-securities/pkg/app"
	"simple-securities/pkg/config"
	"simple-securities/pkg/logger"
)

func main() {
	log := logger.New("mmbot", config.String("APP_ENV", "dev"), config.String("LOG_LEVEL", "info"))
	defer func() { _ = log.Sync() }()

	ctx, stop := app.Context()
	defer stop()

	cfg := mmbot.Config{
		GatewayURL:   config.String("GATEWAY_URL", "http://localhost:8080"),
		EmailPrefix:  config.String("MMBOT_EMAIL_PREFIX", "mm-bot"),
		Password:     config.String("MMBOT_PASSWORD", "bot-password-123"),
		BinanceRef:   strings.EqualFold(config.String("MMBOT_REFERENCE", "binance"), "binance"),
		OrdersPerSec: float64(config.Int("MMBOT_ORDERS_PER_SEC", 4)),
		OrderTTL:     config.Duration("MMBOT_ORDER_TTL", 20*time.Second),
		Futures:      config.Bool("MMBOT_FUTURES", true),
	}
	_ = app.Run(ctx, log, mmbot.New(cfg, log).Run)
}
