# Simple Securities Exchange

A Binance-style **spot and USDⓈ-M perpetual futures crypto exchange** built as Go microservices with event-driven design:
transactional **outbox / inbox**, Kafka, gRPC **and** HTTP between services, an in-memory
price-time-priority matching engine, real-time WebSocket market data and trading, and a
Tailwind web UI.

```
                ┌──────────── web/index.html (Tailwind UI) ────────────┐
                │        REST /api/v1          WebSocket /ws           │
                └───────────────────────┬──────────────────────────────┘
                                        ▼
                                 ┌────────────┐  Redis (rate limit)
                                 │  gateway   │◀──── tail: market.*, matching.events,
                                 └─────┬──────┘       order.updates, wallet.balance.events
                     gRPC or HTTP      │  (UPSTREAM_TRANSPORT=grpc|http)
       ┌───────────────┬───────────────┼────────────────┬──────────────────┐
       ▼               ▼               ▼                ▼                  │
  ┌─────────┐    ┌──────────┐    ┌──────────┐    ┌────────────┐            │
  │identity │    │  wallet  │◀───│  order   │    │ marketdata │            │
  └────┬────┘    └────┬─────┘gRPC└────┬─────┘    └─────▲──────┘            │
       │outbox        │outbox/inbox   │outbox/inbox    │inbox              │
       ▼              ▼               ▼                │                   │
  ════════════════════════════ Kafka ══════════════════════════════════════╛
                                      │ order.commands (key=symbol)
                                      ▼
                               ┌────────────┐  matching.events (trades, order status)
                               │  matching  │──▶ market.depth (100ms snapshots)
                               └────────────┘
```

Each service owns its own Postgres database (database-per-service). Data access uses **GORM**, and
schemas are created/updated at startup with **GORM AutoMigrate** (models in each service, serialised
across replicas with a Postgres advisory lock).

## Services

| Service    | Ports (gRPC/HTTP) | Responsibility |
|------------|-------------------|----------------|
| gateway    | –/8080            | Public REST API, WebSocket streams + WS trading API, JWT, rate limiting, UI |
| identity   | 9101/8101         | Register/login, JWT access tokens, rotating refresh tokens |
| wallet     | 9102/8102         | Balances (free/locked), order locks, ledger, trade settlement, fees |
| order      | 9103/8103         | Order validation & lifecycle, user trade history |
| matching   | –/8104            | Order books, matching (LIMIT GTC/IOC/FOK, MARKET, POST-ONLY), STP |
| marketdata | 9105/8105         | Klines, 24h tickers, public trades, depth |
| futures    | 9106/8106         | USDⓈ-M perpetuals: margin account, positions, leverage, mark/index price, funding, liquidation |
| matching-futures | –/8107      | Same matching binary with `MARKET=futures` (futures order books) |
| marketdata-futures | 9108/8108 | Same marketdata binary with `MARKET=futures` |
| mmbot      | –                 | Market-maker bot that provides spot and futures liquidity (follows Binance prices) |

## Quick start

```bash
make up                 # build & start everything (docker compose)
open http://localhost:8080
```

If ports 8080/5432/6379 are already in use: `GATEWAY_PORT=18080 PG_PORT=15432 REDIS_PORT=16379 make up`.

Run a service from source against the dockerised infra:

```bash
make infra              # postgres, kafka (localhost:9094), redis
make run-identity       # go run ./cmd/identity (env defaults target localhost)
```

## Key design decisions

- **Outbox**: state change + event row in one DB transaction. After commit, a separate goroutine is
  woken and publishes to Kafka right away. A 1s poll covers crashes. One leader per service
  (Postgres advisory lock) keeps the event order.
- **Inbox**: consumers claim `event_id` in the same transaction as their side effects. Kafka offsets
  are committed after that, which gives effectively-once processing. Poison messages go to `<topic>.dlq`.
- **Synchronous risk check, async matching**: `order` locks funds in `wallet` over gRPC (idempotent
  per order id) before the order reaches the engine. A recovery job reconciles orders stuck in `PENDING_NEW`.
- **Single writer per symbol**: `order.commands` is keyed by symbol, so one matching instance owns
  each book. The book state is persisted in the same transaction as the emitted events.
- **Both transports**: every service serves gRPC and grpc-gateway HTTP from the same proto. Callers
  pick one with `UPSTREAM_TRANSPORT` / `WALLET_TRANSPORT`.

Full design, event contract, API and WebSocket protocol: [docs/architecture.md](docs/architecture.md).
Futures (margin model, funding, liquidation, `/fapi/v1` API): [docs/futures.md](docs/futures.md).

## Development

```bash
make proto   # buf generate (protoc-gen-go, -go-grpc, -grpc-gateway in $GOPATH/bin)
make build
make lint
```

Layout: `cmd/<svc>` (wiring) · `internal/<svc>` (domain, store, app, grpcapi, consumer) ·
`pkg/` (database (GORM), outbox, inbox, kafka, grpcx, httpx, events, symbols, auth…) · `proto/` → `gen/` ·
`web/index.html` · `deploy/`.

## License

MIT, see [LICENSE](LICENSE).
