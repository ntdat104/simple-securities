# USDⓈ-M Perpetual Futures

Binance-style USDT-margined perpetual contracts. Scope: one-way position mode, CROSSED and ISOLATED
margin, per-symbol leverage, LIMIT (GTC/IOC/FOK/GTX post-only) and MARKET orders, reduce-only,
close-position, mark/index price, funding, liquidation with insurance fund, spot ↔ futures
transfers. Not implemented yet: hedge mode, TP/SL/stop orders, ADL, multi-asset collateral.

## 1. Components

| Component | How | Notes |
|---|---|---|
| futures (new) | `cmd/futures`, gRPC 9106 / HTTP 8106, DB `futures` | account, positions, orders, risk, mark price, funding, liquidation |
| matching-futures | same `cmd/matching` binary with `MARKET=futures`, HTTP 8107, DB `futures_matching` | topics `futures.order.commands` → `futures.matching.events`, `futures.market.depth` |
| marketdata-futures | same `cmd/marketdata` binary with `MARKET=futures`, gRPC 9108 / HTTP 8108, DB `futures_marketdata` | klines/tickers/trades of the futures market |
| wallet | new RPC `TransferToFutures` + consumer of `futures.transfers` | spot side of transfers |
| gateway | `/fapi/v1/*` REST + futures WS streams/methods | |
| mmbot | futures market making around the spot index | |

Contracts are in `pkg/symbols` (`symbols.GetMarket(symbols.MarketFutures, "BTCUSDT")`): tick/step,
minNotional, maker 0.02% / taker 0.05% (both paid in USDT, `FeeInQuote`), `MaxLeverage`,
`MaintMarginRate` (single bracket), `Underlying` spot symbol (index source).
Topics and payloads: `pkg/events` (`TopicsFor("futures")`, `Transfer`, `FuturesAccountUpdate`, `MarkPrice`,
`OrderAccepted.ReduceOnly/Liquidation`, `OrderUpdated.ReduceOnly/Liquidation/AvgPrice/RealizedPnl`).

## 2. Matching / marketdata in futures mode (`MARKET=futures`)
* Use `symbols.AllMarket/GetMarket(market, …)` and `events.TopicsFor(market)` everywhere; consumer group
  names suffixed `-futures`; DB from `DATABASE_URL`.
* Engine: when `sym.FeeInQuote`, BOTH fees are `quote_qty * rate` (buyer_fee in quote). MARKET BUY with
  `quantity` (and empty `quote_quantity`) must be supported: match by quantity like MARKET SELL.
  Spot behaviour unchanged.

## 3. Futures account model (futures service)

Per user, asset USDT:
* `wallet_balance` – cross collateral cash: transfers in/out, realized PnL, commissions and funding of
  CROSSED positions. Isolated margin is moved OUT of it into the position.
* Position (one row per user+symbol, one-way): `size` (signed qty), `entry_price`, `margin_type`,
  `leverage`, `isolated_margin` (ISOLATED only, cash).
* Open orders freeze `order_margin` (tracked per order, remaining part released on terminal status).

Formulas (mark price `M`, contract maint rate `mmr`, taker fee `ft`):
* `notional = |size| * M`, `uPnL = size * (M - entry)`
* position initial margin `IM = notional / leverage`, maintenance `MM = notional * mmr`
* cross uPnL = Σ uPnL of CROSSED positions; cross IM = Σ IM of CROSSED positions
* `available = wallet_balance + cross_uPnL - cross_IM - Σ open order margin` (floored at 0)
* `margin_balance = wallet_balance + cross_uPnL` (+ Σ isolated_margin + isolated uPnL for display totals)
* `max_withdraw = min(available, wallet_balance - cross_IM - Σ order margin)` floored at 0

### Placing an order (synchronous risk check, single DB transaction)
1. Validate contract filters (tick, step, minNotional on `price*qty`; MARKET uses mark price), side, type,
   TIF (`GTX` = post-only → engine type `LIMIT_MAKER`), leverage/margin type from the user's symbol
   setting (default leverage 20 capped to MaxLeverage, CROSSED).
2. One-way netting: `close_qty = min(qty, |size|)` when the order side is opposite to the position,
   `open_qty = qty - close_qty`. Reduce-only: reject `REDUCE_ONLY_REJECTED` if there is no opposite
   position; cap qty to `|size|`; `open_qty = 0`.
3. `ref_price` = LIMIT price, MARKET → mark price × (1.005 for BUY, 0.995 for SELL).
   `order_margin = open_qty * ref_price / leverage + qty * ref_price * ft` (fee reserve).
4. Reject `INSUFFICIENT_MARGIN` if `order_margin > available`.
5. tx{insert order NEW with margin_frozen, outbox `order.accepted` → `futures.order.commands`
   (key symbol)} → Notify. Cancel: like spot (`order.cancel_requested`, status unchanged until CANCELED).

### Applying fills (consumer of `futures.matching.events`, inbox dedup, one tx per batch)
For each side of `trade.executed` whose order belongs to this service (both sides normally):
1. signed fill `q` (+BUY/−SELL), price `P`, fee `F` (buyer_fee/seller_fee, USDT).
2. If position is flat or same direction: open/increase → `entry = (|s|·entry + |q|·P)/(|s|+|q|)`;
   ISOLATED: move `|q|·P/leverage` from wallet_balance into isolated_margin.
3. If opposite: `c = min(|q|,|s|)`, realized `R = c·(P − entry)·sign(s)`; ISOLATED: release
   `isolated_margin·c/|s|` back to wallet_balance; CROSSED/ISOLATED: wallet_balance += R; then if
   `|q| > |s|` the rest opens a new position at `P` (step 2).
4. wallet_balance −= F. Release the order's frozen margin pro rata to the filled qty.
5. Income rows: REALIZED_PNL (if ≠0), COMMISSION (−F). User trade row (realized PnL, commission, maker).
6. `order.status_changed`: update order (only if seq > last_seq); terminal → release remaining frozen
   margin. When a position becomes flat: cancel the user's open reduce-only orders of that symbol.
7. Emit `order.updated` (topic `futures.order.updates`) and one `futures.account_update`
   (topic `futures.account.events`) per affected user, after commit.

### Mark price, index, funding (leader only: Postgres advisory lock)
* Index price: underlying SPOT mid price from `market.depth` (fallback spot last from `market.ticker`).
* Futures mid/last from `futures.market.depth` / `futures.matching.events`.
* Every 1s: `premium = (futures_mid − index) / index` sampled; `basis_ema` EMA(60s) of `futures_mid − index`.
  `mark = median(index·(1 + est_rate·t/interval), index + basis_ema, futures_last)` (missing inputs
  → index). Publish `MarkPrice` to `futures.mark_price` and cache it.
* Funding interval env `FUNDING_INTERVAL` (default 8h, aligned to epoch; compose uses 1h).
  `est_rate = clamp(avg_premium + clamp(0.0001 − avg_premium, −0.0005, 0.0005), −0.0075, 0.0075)`.
  At settlement time, idempotent per (symbol, funding_time): for every open position
  `payment = size · M · rate` (longs pay when rate > 0, shorts receive); CROSSED from/to
  wallet_balance, ISOLATED from/to isolated_margin; income FUNDING_FEE; store `funding_rates` row.

### Liquidation (leader only, every 1s on mark price)
* ISOLATED position: liquidate when `isolated_margin + uPnL ≤ MM`.
* CROSSED: liquidate ALL cross positions of the user when `wallet_balance + cross_uPnL ≤ Σ cross MM`.
* Procedure (tx): flag position `liquidating`, cancel the user's open orders of that symbol (outbox
  cancel commands), place a MARKET IOC reduce-only order for `|size|` with `liquidation=true`.
  When the liquidation order is terminal: clear the flag. ISOLATED: the remaining isolated margin of the
  closed position goes to the insurance fund (INSURANCE_CLEAR); CROSSED: if wallet_balance < 0 the
  insurance fund covers the deficit (balance reset to 0). Partially filled liquidation → retried later.
* Liquidation price shown on positions: long `(|s|·E − M_avail)/(|s|·(1 − mmr))`,
  short `(|s|·E + M_avail)/(|s|·(1 + mmr))`, where `M_avail` = isolated_margin (ISOLATED) or
  `wallet_balance + other cross uPnL − other cross MM` (CROSSED); "0" if ≤ 0.

### Transfers
* Spot → futures: wallet.TransferToFutures → tx{debit spot free, ledger FUTURES_TRANSFER_OUT, outbox
  `transfer.spot_to_futures` on `wallet.transfers`} → futures consumes (inbox) → wallet_balance +=,
  income TRANSFER.
* Futures → spot: futures.TransferToSpot → check `amount ≤ max_withdraw` → tx{wallet_balance −=, income
  TRANSFER, outbox `transfer.futures_to_spot` on `futures.transfers`} → wallet consumes → spot free +=,
  ledger FUTURES_TRANSFER_IN.
* Only USDT.

## 4. Public API (gateway)

REST `/fapi/v1` (JSON camelCase, decimals as strings, times epoch ms, errors `{code,msg}`):

| Method | Path | Auth | Upstream |
|---|---|---|---|
| GET | /fapi/v1/exchangeInfo | – | symbols registry (futures) |
| GET | /fapi/v1/depth, /trades, /klines, /ticker/24hr | – | marketdata-futures (same shapes as spot) |
| GET | /fapi/v1/premiumIndex?symbol= | – | futures.GetMarkPrices → object (or array) |
| GET | /fapi/v1/fundingRate?symbol=&limit= | – | futures.ListFundingRates |
| GET | /fapi/v1/account | ✓ | futures.GetAccount |
| GET | /fapi/v1/positionRisk?symbol= | ✓ | futures.ListPositions |
| POST | /fapi/v1/leverage `{symbol,leverage}` | ✓ | |
| POST | /fapi/v1/marginType `{symbol,marginType}` | ✓ | |
| POST | /fapi/v1/positionMargin `{symbol,amount,type:"ADD"\|"REDUCE"}` | ✓ | |
| POST | /fapi/v1/order `{symbol,side,type,timeInForce,price,quantity,reduceOnly,newClientOrderId}` | ✓ | |
| DELETE | /fapi/v1/order?orderId= | ✓ | |
| GET | /fapi/v1/order?orderId=, /openOrders?symbol=, /allOrders?symbol=&limit= | ✓ | |
| POST | /fapi/v1/closePosition `{symbol}` | ✓ | |
| GET | /fapi/v1/userTrades?symbol=&limit= | ✓ | |
| GET | /fapi/v1/income?symbol=&incomeType=&limit= | ✓ | |
| POST | /fapi/v1/transfer `{asset,amount,type:"SPOT_TO_FUTURES"\|"FUTURES_TO_SPOT"}` | ✓ | wallet / futures |

Shapes:
* **FuturesOrder** `{orderId, clientOrderId, symbol, side, type, timeInForce, price, origQty, executedQty, cumQuote, avgPrice, reduceOnly, liquidation, status, rejectReason, realizedPnl, time, updateTime}`
* **Position** `{symbol, positionAmt, entryPrice, breakEvenPrice, markPrice, unRealizedProfit, liquidationPrice, leverage, marginType ("cross"|"isolated"), isolatedMargin, notional, initialMargin, maintMargin, updateTime}`
* **Account** `{asset, walletBalance, unrealizedProfit, marginBalance, availableBalance, maxWithdrawAmount, totalInitialMargin, totalMaintMargin, totalOpenOrderMargin, positions:[Position]}`
* **UserTrade** `{id, orderId, symbol, side, price, qty, quoteQty, realizedPnl, commission, commissionAsset, maker, time}`
* **Income** `{id, symbol, incomeType, income, asset, info, time}`
* **premiumIndex** `{symbol, markPrice, indexPrice, lastFundingRate, nextFundingTime, time}`
* **fundingRate** `[{symbol, fundingRate, fundingTime, markPrice}]`
* **exchangeInfo** `{serverTime, symbols:[{symbol, baseAsset, quoteAsset, contractType:"PERPETUAL", tickSize, stepSize, minNotional, makerFee, takerFee, maxLeverage, maintMarginRate, underlying, status}]}`
* **leverage** `{symbol, leverage, maxNotional}`; **marginType** `{symbol, marginType}`; **transfer** `{tranId, asset, amount, type, status}`

WebSocket (same `/ws`): futures streams are prefixed with `f.`:
`f.<sym>@trade`, `f.<sym>@depth[5|10|20]`, `f.<sym>@kline_<i>`, `f.<sym>@ticker`, `f.!ticker@arr`,
`f.<sym>@markPrice` → `{"e":"markPriceUpdate","s","p","i","r","T","E"}`, `f.!markPrice@arr` (≤1/s).
After LOGIN the user also receives `ORDER_TRADE_UPDATE` (`{"e":"ORDER_TRADE_UPDATE", ...FuturesOrder, lastTradeId}`)
and `ACCOUNT_UPDATE` (`{"e":"ACCOUNT_UPDATE", reason, walletBalance, availableBalance, positions:[{symbol, positionAmt, entryPrice, marginType, isolatedMargin, leverage}]}`).
WS API: `futures.order.place` (params like POST /fapi/v1/order), `futures.order.cancel` `{orderId}`.
