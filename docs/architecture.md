# Simple Securities Exchange: Architecture

A spot crypto exchange (Binance-style) built as Go microservices with event-driven design.

## 1. Services

| Service      | gRPC | HTTP | DB (Postgres) | Owns |
|--------------|------|------|---------------|------|
| gateway      | —    | 8080 | — (Redis)     | Public REST `/api/v1`, WebSocket `/ws`, JWT auth, rate limit, serves `web/index.html` |
| identity     | 9101 | 8101 | `identity`    | Users, bcrypt credentials, JWT access tokens, rotating refresh tokens |
| wallet       | 9102 | 8102 | `wallet`      | Balances (free/locked), order locks, immutable ledger, fee account, settlement |
| order        | 9103 | 8103 | `orders`      | Order lifecycle, validation (symbol filters), user trade history |
| matching     | —    | 8104 | `matching`    | In-memory price-time priority order books, trades, fees, depth |
| marketdata   | 9105 | 8105 | `marketdata`  | Public trades, klines, 24h tickers, depth cache |
| mmbot        | —    | —    | —             | Market maker / liquidity bot (talks to the gateway like a client) |

Every service exposes **both gRPC and HTTP/JSON** (grpc-gateway generated from the same proto, see
`proto/`). Callers select the transport through configuration (`UPSTREAM_TRANSPORT`, `WALLET_TRANSPORT`
= `grpc|http`). Each client package has a gRPC and an HTTP implementation behind one interface.
Every HTTP port also serves `/healthz`, `/readyz` and `/metrics` (Prometheus).

Persistence: GORM (`pkg/database`) on Postgres, one database per service. Each service declares its
tables as GORM models and runs `database.Migrate` (AutoMigrate, serialised with an advisory lock)
at startup; the shared `outbox_events` / `inbox_events` models come from `outbox.Models()`.

## 2. Asynchronous contract

All topics, event types and payload structs live in `pkg/events` (the single source of truth).

```
identity ──user.registered──────────────────────────────▶ wallet (demo airdrop)
order ──order.accepted / order.cancel_requested (key=symbol)──▶ matching
matching ──trade.executed / order.status_changed (key=symbol)──▶ order, wallet, marketdata, gateway
order ──order.updated (key=user)──────────────────────────▶ gateway (WS executionReport)
wallet ──balance.updated (key=user)───────────────────────▶ gateway (WS outboundAccountPosition)
matching ──market.depth (direct, 100ms)───────────────────▶ marketdata, gateway
marketdata ──market.ticker / market.kline (direct)────────▶ marketdata, gateway
```

### Outbox (producer side), `pkg/outbox`
1. The business change and `outbox.Add(...)` run in **one Postgres transaction** (GORM `Transaction`).
2. After commit the service calls `dispatcher.Notify()`. A dedicated goroutine wakes up immediately
   and publishes pending rows to Kafka (idempotent producer, acks=all), then marks them published.
3. A 1s poll is the safety net if the process died between commit and publish → at-least-once.
4. Exactly one dispatcher per service is leader (Postgres advisory lock) and relays strictly by id →
   per-key ordering on Kafka equals commit order. Standbys take over automatically.
5. Published rows and inbox rows are purged after 72h.

### Inbox (consumer side), `pkg/inbox` + `pkg/kafka.GroupConsumer`
1. The consumer receives a batch from one partition (ordered).
2. In one transaction: `inbox.Claim(tx, consumer, event.ID)` → if false it is a duplicate, skip;
   otherwise apply the side effects (and possibly `outbox.Add` follow-up events).
3. Commit the DB tx, then commit Kafka offsets. Crash between both → redelivery → inbox dedups.
4. Failures retry with exponential backoff; after N attempts the offending message goes to `<topic>.dlq`.

`market.*` topics are derived snapshots published directly (no outbox): losing one is harmless
because the next snapshot supersedes it.

## 3. Core flows

### Register
gateway → identity.Register → tx{insert user, insert refresh token, outbox `user.registered`} →
wallet consumes → tx{inbox, create balances, airdrop demo funds (10,000 USDT, ledger ref AIRDROP),
outbox `balance.updated`}.

### Place order (sync risk check, async matching)
1. gateway (JWT) → order.PlaceOrder.
2. order validates symbol filters (tickSize, stepSize, minNotional, type/TIF combinations),
   dedups `client_order_id` per user, inserts the order as `PENDING_NEW`.
3. order → wallet.LockFunds(order_id, asset, amount) **idempotent on order_id**.
   - LIMIT BUY locks `price*qty` quote · SELL locks `qty` base · MARKET BUY locks `quote_quantity`.
   - Insufficient → order `REJECTED` (HTTP 400 `INSUFFICIENT_BALANCE`).
4. tx{order → `NEW`, outbox `order.accepted` (topic order.commands, key=symbol)} → Notify().
5. Recovery job: orders stuck in `PENDING_NEW` > 10s are reconciled with wallet.GetLock
   (found → proceed to step 4, not found → REJECTED). No dangling locks, no lost orders.

### Matching (`matching`)
* Consumer group on `order.commands`; partition = hash(symbol) → one writer per symbol.
* Per partition batch: one DB transaction containing inbox claims, `book_orders` upserts/deletes,
  `symbol_state` (trade id / update id sequences) and outbox events. In-memory book is the hot path;
  if the tx fails the affected books are discarded and reloaded from `book_orders` (no divergence).
* Books are loaded lazily per symbol from `book_orders`; dropped on partition revoke.
* Order types: LIMIT (GTC/IOC/FOK), LIMIT_MAKER (post-only, rejected if it would cross), MARKET
  (SELL by quantity, BUY by quote budget). Self-trade prevention: EXPIRE_TAKER.
* Emits, in order: `trade.executed` per fill, then `order.status_changed` per touched order.
* Every 100ms publishes `market.depth` (top 50) for books that changed.

### Settlement (`wallet`, consumer of matching.events)
* `trade.executed`: buyer locked quote −= quote_qty, free base += qty − buyer_fee;
  seller locked base −= qty, free quote += quote_qty − seller_fee; fee account += fees;
  order lock `remaining` decremented. One ledger row per balance movement.
* `order.status_changed` terminal (FILLED/CANCELED/EXPIRED/REJECTED): release remaining lock
  (price improvement leftovers, cancels, IOC remainders).
* Emits `balance.updated` via outbox.

### Order state (`order`, consumer of matching.events)
* `order.status_changed` → update status/executed/cum quote **only if seq > last_seq**.
* `trade.executed` → insert two user trades (buyer & seller, commission, is_maker).
* Emits `order.updated` via outbox (→ WS executionReport).

### Cancel
order validates ownership & open status (NEW / PARTIALLY_FILLED) → outbox `order.cancel_requested`
(status is NOT changed synchronously; the API returns the current order) → matching removes it →
`order.status_changed CANCELED` → order & wallet react (lock released). Cancelling an order that was
filled in the meantime is a no-op in matching. Idempotent: repeated cancels are harmless.

### Market data
Consumes `trade.executed` (group): tx{inbox, insert public trade, upsert klines for
1m/5m/15m/1h/4h/1d}. Every 1s recomputes 24h tickers of dirty symbols from 1m klines and publishes
`market.ticker`; publishes `market.kline` on change (throttled 250ms). Tail-consumes `market.depth`
and `market.ticker` to serve REST from memory on every instance.

## 4. Public API (gateway, `/api/v1`)

JSON, decimals as strings, errors `{"code":"INSUFFICIENT_BALANCE","msg":"..."}` with proper HTTP status.
Auth: `Authorization: Bearer <access_token>`.

| Method | Path | Auth | Notes |
|---|---|---|---|
| POST | /auth/register | – | `{email,password}` → `{user:{userId,email},accessToken,refreshToken,expiresIn}` |
| POST | /auth/login | – | same response |
| POST | /auth/refresh | – | `{refreshToken}` (rotation) |
| POST | /auth/logout | ✓ | `{refreshToken}` |
| GET | /exchangeInfo | – | symbols + filters |
| GET | /ticker/24hr?symbol= | – | one or all |
| GET | /klines?symbol=&interval=&limit= | – | `[[openTime,open,high,low,close,volume,closeTime,quoteVolume,trades],...]` |
| GET | /depth?symbol=&limit= | – | `{lastUpdateId,bids:[[p,q]],asks:[[p,q]]}` |
| GET | /trades?symbol=&limit= | – | recent public trades |
| GET | /account | ✓ | `{balances:[{asset,free,locked}]}` |
| POST | /wallet/deposit | ✓ | `{asset,amount}` simulated deposit (faucet) |
| POST | /wallet/withdraw | ✓ | `{asset,amount,address}` |
| GET | /wallet/ledger?limit= | ✓ | ledger entries |
| POST | /order | ✓ | `{symbol,side,type,timeInForce,price,quantity,quoteOrderQty,newClientOrderId}` |
| DELETE | /order?orderId= | ✓ | cancel |
| GET | /order?orderId= | ✓ | |
| GET | /openOrders?symbol= | ✓ | |
| GET | /allOrders?symbol=&limit= | ✓ | |
| GET | /myTrades?symbol=&limit= | ✓ | |

Rate limits (Redis, fixed window, fail-open): 1200 req/min per IP, 50 orders/10s per user.

### Response shapes (camelCase, decimals as strings, times in epoch ms)
* **Order**: `{orderId, clientOrderId, symbol, side, type, timeInForce, price, origQty, quoteOrderQty, executedQty, cummulativeQuoteQty, status, rejectReason, time, updateTime}`
* **MyTrade**: `{id, orderId, symbol, side, price, qty, quoteQty, commission, commissionAsset, isMaker, time}`
* **PublicTrade**: `{id, price, qty, quoteQty, time, isBuyerMaker}`
* **Ticker**: `{symbol, lastPrice, openPrice, highPrice, lowPrice, volume, quoteVolume, priceChange, priceChangePercent, count, closeTime}` (array when no symbol)
* **exchangeInfo**: `{serverTime, symbols:[{symbol, baseAsset, quoteAsset, tickSize, stepSize, minNotional, makerFee, takerFee, status}]}`
* **account**: `{balances:[{asset, free, locked}]}`
* **ledger**: `[{id, asset, deltaFree, deltaLocked, freeAfter, lockedAfter, refType, refId, time}]`
* **error**: `{code, msg}`. HTTP mapping: InvalidArgument→400, FailedPrecondition→400, Unauthenticated→401,
  PermissionDenied→403, NotFound→404, AlreadyExists→409, ResourceExhausted→429, Unavailable→503, else 500.

## 5. WebSocket (`/ws`)

Binance-style streams plus a WS API for trading.

```jsonc
// subscribe / unsubscribe
{"method":"SUBSCRIBE","params":["btcusdt@trade","btcusdt@depth20","btcusdt@kline_1m","!ticker@arr"],"id":1}
{"method":"UNSUBSCRIBE","params":["btcusdt@trade"],"id":2}
// authenticate this connection for user data + trading
{"method":"LOGIN","params":{"token":"<access_token>"},"id":3}
// trading over websocket (after LOGIN)
{"method":"order.place","params":{"symbol":"BTCUSDT","side":"BUY","type":"LIMIT","timeInForce":"GTC","price":"95000","quantity":"0.001"},"id":4}
{"method":"order.cancel","params":{"orderId":"..."},"id":5}
{"method":"PING","id":6}
// responses
{"id":1,"result":null}  /  {"id":4,"result":{...order...}}  /  {"id":4,"error":{"code":"...","msg":"..."}}
// stream pushes
{"stream":"btcusdt@trade","data":{"e":"trade","s":"BTCUSDT","t":1,"p":"97000.00","q":"0.01","T":1731150000000,"m":true}}
{"stream":"btcusdt@depth20","data":{"lastUpdateId":5,"bids":[["p","q"]],"asks":[["p","q"]]}}
{"stream":"btcusdt@kline_1m","data":{"e":"kline","s":"BTCUSDT","k":{"t":..,"T":..,"i":"1m","o":"","h":"","l":"","c":"","v":"","q":"","n":0}}}
{"stream":"!ticker@arr","data":[{"s":"BTCUSDT","c":"last","o":"open","h":"","l":"","v":"","q":"","p":"change","P":"pct","n":0}]}
{"stream":"btcusdt@ticker","data":{...same single object...}}
// user data (after LOGIN), pushed automatically
{"stream":"executionReport","data":{"e":"executionReport", ...Order shape..., "lastTradeId":1}}
{"stream":"outboundAccountPosition","data":{"e":"outboundAccountPosition","balances":[{"asset":"USDT","free":"1","locked":"0"}]}}
```
Server pings every 20s; clients idle > 60s are closed; slow consumers (send buffer full) are dropped.

## 6. Production notes / roadmap
* Kafka: RF=3, `min.insync.replicas=2` in prod (compose uses a single KRaft broker).
* mTLS between services (or a mesh), JWT EdDSA + JWKS, API keys with HMAC signatures.
* OpenTelemetry tracing across gRPC/Kafka (propagate trace context in record headers).
* Matching snapshotting + command log replay for sub-second failover; int64 fixed-point engine.
* Not yet implemented: stop orders, margin, futures, P2P, earn, real chain deposits/withdrawals.
