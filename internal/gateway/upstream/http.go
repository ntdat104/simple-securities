package upstream

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	identityv1 "simple-securities/gen/identity/v1"
	marketdatav1 "simple-securities/gen/marketdata/v1"
	orderv1 "simple-securities/gen/order/v1"
	walletv1 "simple-securities/gen/wallet/v1"
	"simple-securities/pkg/httpx"
)

func newHTTP(cfg Config) *Clients {
	return &Clients{
		Identity:   httpIdentity{c: httpx.NewClient(cfg.IdentityHTTP)},
		Wallet:     httpWallet{c: httpx.NewClient(cfg.WalletHTTP)},
		Order:      httpOrder{c: httpx.NewClient(cfg.OrderHTTP)},
		MarketData: httpMarketData{c: httpx.NewClient(cfg.MarketDataHTTP)},
		Futures:    httpFutures{c: httpx.NewClient(cfg.FuturesHTTP)},

		FuturesMarketData: httpMarketData{c: httpx.NewClient(cfg.FuturesMarketDataHTTP)},
	}
}

// call performs one grpc-gateway request with the per-call timeout and guarantees a status error.
func call[T any, PT interface {
	*T
	proto.Message
}](ctx context.Context, c *httpx.Client, method, path string, q url.Values, body proto.Message) (PT, error) {
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	out := PT(new(T))
	if err := c.Do(ctx, method, path, q, body, out); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return out, nil
}

// query builds GET parameters with proto field names, skipping empty values.
func query(kv ...string) url.Values {
	q := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] != "" && kv[i+1] != "0" {
			q.Set(kv[i], kv[i+1])
		}
	}
	return q
}

func itoa[T int32 | int64](v T) string { return strconv.FormatInt(int64(v), 10) }

func seg(s string) string { return url.PathEscape(s) }

type httpIdentity struct{ c *httpx.Client }

func (h httpIdentity) Register(ctx context.Context, r *identityv1.RegisterRequest) (*identityv1.AuthResponse, error) {
	return call[identityv1.AuthResponse](ctx, h.c, http.MethodPost, "/v1/identity/register", nil, r)
}

func (h httpIdentity) Login(ctx context.Context, r *identityv1.LoginRequest) (*identityv1.AuthResponse, error) {
	return call[identityv1.AuthResponse](ctx, h.c, http.MethodPost, "/v1/identity/login", nil, r)
}

func (h httpIdentity) Refresh(ctx context.Context, r *identityv1.RefreshRequest) (*identityv1.AuthResponse, error) {
	return call[identityv1.AuthResponse](ctx, h.c, http.MethodPost, "/v1/identity/refresh", nil, r)
}

func (h httpIdentity) Logout(ctx context.Context, r *identityv1.LogoutRequest) (*identityv1.LogoutResponse, error) {
	return call[identityv1.LogoutResponse](ctx, h.c, http.MethodPost, "/v1/identity/logout", nil, r)
}

type httpWallet struct{ c *httpx.Client }

func (h httpWallet) GetBalances(
	ctx context.Context, r *walletv1.GetBalancesRequest,
) (*walletv1.GetBalancesResponse, error) {
	return call[walletv1.GetBalancesResponse](ctx, h.c, http.MethodGet,
		"/v1/wallet/users/"+seg(r.GetUserId())+"/balances", nil, nil)
}

func (h httpWallet) Deposit(ctx context.Context, r *walletv1.DepositRequest) (*walletv1.LedgerEntry, error) {
	return call[walletv1.LedgerEntry](ctx, h.c, http.MethodPost, "/v1/wallet/deposit", nil, r)
}

func (h httpWallet) Withdraw(ctx context.Context, r *walletv1.WithdrawRequest) (*walletv1.LedgerEntry, error) {
	return call[walletv1.LedgerEntry](ctx, h.c, http.MethodPost, "/v1/wallet/withdraw", nil, r)
}

func (h httpWallet) ListLedger(
	ctx context.Context, r *walletv1.ListLedgerRequest,
) (*walletv1.ListLedgerResponse, error) {
	return call[walletv1.ListLedgerResponse](ctx, h.c, http.MethodGet,
		"/v1/wallet/users/"+seg(r.GetUserId())+"/ledger",
		query("limit", itoa(r.GetLimit()), "before_id", itoa(r.GetBeforeId())), nil)
}

func (h httpWallet) TransferToFutures(
	ctx context.Context, r *walletv1.TransferToFuturesRequest,
) (*walletv1.LedgerEntry, error) {
	return call[walletv1.LedgerEntry](ctx, h.c, http.MethodPost, "/v1/wallet/transfers/futures", nil, r)
}

type httpOrder struct{ c *httpx.Client }

func (h httpOrder) PlaceOrder(ctx context.Context, r *orderv1.PlaceOrderRequest) (*orderv1.Order, error) {
	return call[orderv1.Order](ctx, h.c, http.MethodPost, "/v1/orders", nil, r)
}

func (h httpOrder) CancelOrder(ctx context.Context, r *orderv1.CancelOrderRequest) (*orderv1.Order, error) {
	return call[orderv1.Order](ctx, h.c, http.MethodPost, "/v1/orders/"+seg(r.GetOrderId())+"/cancel", nil, r)
}

func (h httpOrder) GetOrder(ctx context.Context, r *orderv1.GetOrderRequest) (*orderv1.Order, error) {
	return call[orderv1.Order](ctx, h.c, http.MethodGet, "/v1/orders/"+seg(r.GetOrderId()),
		query("user_id", r.GetUserId()), nil)
}

func (h httpOrder) ListOpenOrders(
	ctx context.Context, r *orderv1.ListOpenOrdersRequest,
) (*orderv1.ListOrdersResponse, error) {
	return call[orderv1.ListOrdersResponse](ctx, h.c, http.MethodGet,
		"/v1/users/"+seg(r.GetUserId())+"/open-orders", query("symbol", r.GetSymbol()), nil)
}

func (h httpOrder) ListOrders(ctx context.Context, r *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	return call[orderv1.ListOrdersResponse](ctx, h.c, http.MethodGet,
		"/v1/users/"+seg(r.GetUserId())+"/orders",
		query("symbol", r.GetSymbol(), "limit", itoa(r.GetLimit())), nil)
}

func (h httpOrder) ListTrades(ctx context.Context, r *orderv1.ListTradesRequest) (*orderv1.ListTradesResponse, error) {
	return call[orderv1.ListTradesResponse](ctx, h.c, http.MethodGet,
		"/v1/users/"+seg(r.GetUserId())+"/trades",
		query("symbol", r.GetSymbol(), "limit", itoa(r.GetLimit())), nil)
}

type httpMarketData struct{ c *httpx.Client }

func (h httpMarketData) GetExchangeInfo(
	ctx context.Context, _ *marketdatav1.GetExchangeInfoRequest,
) (*marketdatav1.ExchangeInfo, error) {
	return call[marketdatav1.ExchangeInfo](ctx, h.c, http.MethodGet, "/v1/market/exchange-info", nil, nil)
}

func (h httpMarketData) GetTickers(
	ctx context.Context, r *marketdatav1.GetTickersRequest,
) (*marketdatav1.GetTickersResponse, error) {
	return call[marketdatav1.GetTickersResponse](ctx, h.c, http.MethodGet, "/v1/market/tickers",
		query("symbol", r.GetSymbol()), nil)
}

func (h httpMarketData) GetKlines(
	ctx context.Context, r *marketdatav1.GetKlinesRequest,
) (*marketdatav1.GetKlinesResponse, error) {
	return call[marketdatav1.GetKlinesResponse](ctx, h.c, http.MethodGet, "/v1/market/klines",
		query("symbol", r.GetSymbol(), "interval", r.GetInterval(), "limit", itoa(r.GetLimit()),
			"end_time", itoa(r.GetEndTime())), nil)
}

func (h httpMarketData) GetDepth(ctx context.Context, r *marketdatav1.GetDepthRequest) (*marketdatav1.Depth, error) {
	return call[marketdatav1.Depth](ctx, h.c, http.MethodGet, "/v1/market/depth",
		query("symbol", r.GetSymbol(), "limit", itoa(r.GetLimit())), nil)
}

func (h httpMarketData) GetTrades(
	ctx context.Context, r *marketdatav1.GetTradesRequest,
) (*marketdatav1.GetTradesResponse, error) {
	return call[marketdatav1.GetTradesResponse](ctx, h.c, http.MethodGet, "/v1/market/trades",
		query("symbol", r.GetSymbol(), "limit", itoa(r.GetLimit())), nil)
}
