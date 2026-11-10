package upstream

import (
	"context"
	"net/http"

	futuresv1 "simple-securities/gen/futures/v1"
	"simple-securities/pkg/httpx"
)

// grpcFutures adapts the generated client (variadic call options) to Futures.
type grpcFutures struct {
	c futuresv1.FuturesServiceClient
}

func (g grpcFutures) GetMarkPrices(
	ctx context.Context, r *futuresv1.GetMarkPricesRequest,
) (*futuresv1.GetMarkPricesResponse, error) {
	return g.c.GetMarkPrices(ctx, r)
}

func (g grpcFutures) ListFundingRates(
	ctx context.Context, r *futuresv1.ListFundingRatesRequest,
) (*futuresv1.ListFundingRatesResponse, error) {
	return g.c.ListFundingRates(ctx, r)
}

func (g grpcFutures) GetAccount(
	ctx context.Context, r *futuresv1.GetAccountRequest,
) (*futuresv1.Account, error) {
	return g.c.GetAccount(ctx, r)
}

func (g grpcFutures) ListPositions(
	ctx context.Context, r *futuresv1.ListPositionsRequest,
) (*futuresv1.ListPositionsResponse, error) {
	return g.c.ListPositions(ctx, r)
}

func (g grpcFutures) SetLeverage(
	ctx context.Context, r *futuresv1.SetLeverageRequest,
) (*futuresv1.SymbolSetting, error) {
	return g.c.SetLeverage(ctx, r)
}

func (g grpcFutures) SetMarginType(
	ctx context.Context, r *futuresv1.SetMarginTypeRequest,
) (*futuresv1.SymbolSetting, error) {
	return g.c.SetMarginType(ctx, r)
}

func (g grpcFutures) AdjustIsolatedMargin(
	ctx context.Context, r *futuresv1.AdjustIsolatedMarginRequest,
) (*futuresv1.Position, error) {
	return g.c.AdjustIsolatedMargin(ctx, r)
}

func (g grpcFutures) TransferToSpot(
	ctx context.Context, r *futuresv1.TransferToSpotRequest,
) (*futuresv1.TransferResponse, error) {
	return g.c.TransferToSpot(ctx, r)
}

func (g grpcFutures) ListIncome(
	ctx context.Context, r *futuresv1.ListIncomeRequest,
) (*futuresv1.ListIncomeResponse, error) {
	return g.c.ListIncome(ctx, r)
}

func (g grpcFutures) PlaceOrder(
	ctx context.Context, r *futuresv1.PlaceOrderRequest,
) (*futuresv1.Order, error) {
	return g.c.PlaceOrder(ctx, r)
}

func (g grpcFutures) CancelOrder(
	ctx context.Context, r *futuresv1.CancelOrderRequest,
) (*futuresv1.Order, error) {
	return g.c.CancelOrder(ctx, r)
}

func (g grpcFutures) ClosePosition(
	ctx context.Context, r *futuresv1.ClosePositionRequest,
) (*futuresv1.Order, error) {
	return g.c.ClosePosition(ctx, r)
}

func (g grpcFutures) GetOrder(
	ctx context.Context, r *futuresv1.GetOrderRequest,
) (*futuresv1.Order, error) {
	return g.c.GetOrder(ctx, r)
}

func (g grpcFutures) ListOpenOrders(
	ctx context.Context, r *futuresv1.ListOpenOrdersRequest,
) (*futuresv1.ListOrdersResponse, error) {
	return g.c.ListOpenOrders(ctx, r)
}

func (g grpcFutures) ListOrders(
	ctx context.Context, r *futuresv1.ListOrdersRequest,
) (*futuresv1.ListOrdersResponse, error) {
	return g.c.ListOrders(ctx, r)
}

func (g grpcFutures) ListTrades(
	ctx context.Context, r *futuresv1.ListTradesRequest,
) (*futuresv1.ListTradesResponse, error) {
	return g.c.ListTrades(ctx, r)
}

// httpFutures calls the futures service through its grpc-gateway HTTP/JSON surface.
type httpFutures struct{ c *httpx.Client }

func (h httpFutures) GetMarkPrices(
	ctx context.Context, r *futuresv1.GetMarkPricesRequest,
) (*futuresv1.GetMarkPricesResponse, error) {
	return call[futuresv1.GetMarkPricesResponse](ctx, h.c, http.MethodGet, "/v1/futures/mark-prices", query("symbol", r.GetSymbol()), nil)
}

func (h httpFutures) ListFundingRates(
	ctx context.Context, r *futuresv1.ListFundingRatesRequest,
) (*futuresv1.ListFundingRatesResponse, error) {
	return call[futuresv1.ListFundingRatesResponse](ctx, h.c, http.MethodGet, "/v1/futures/funding-rates",
		query("symbol", r.GetSymbol(), "limit", itoa(r.GetLimit())), nil)
}

func (h httpFutures) GetAccount(
	ctx context.Context, r *futuresv1.GetAccountRequest,
) (*futuresv1.Account, error) {
	return call[futuresv1.Account](ctx, h.c, http.MethodGet, "/v1/futures/users/"+seg(r.GetUserId())+"/account", nil, nil)
}

func (h httpFutures) ListPositions(
	ctx context.Context, r *futuresv1.ListPositionsRequest,
) (*futuresv1.ListPositionsResponse, error) {
	return call[futuresv1.ListPositionsResponse](ctx, h.c, http.MethodGet, "/v1/futures/users/"+seg(r.GetUserId())+"/positions",
		query("symbol", r.GetSymbol()), nil)
}

func (h httpFutures) SetLeverage(
	ctx context.Context, r *futuresv1.SetLeverageRequest,
) (*futuresv1.SymbolSetting, error) {
	return call[futuresv1.SymbolSetting](ctx, h.c, http.MethodPost, "/v1/futures/leverage", nil, r)
}

func (h httpFutures) SetMarginType(
	ctx context.Context, r *futuresv1.SetMarginTypeRequest,
) (*futuresv1.SymbolSetting, error) {
	return call[futuresv1.SymbolSetting](ctx, h.c, http.MethodPost, "/v1/futures/margin-type", nil, r)
}

func (h httpFutures) AdjustIsolatedMargin(
	ctx context.Context, r *futuresv1.AdjustIsolatedMarginRequest,
) (*futuresv1.Position, error) {
	return call[futuresv1.Position](ctx, h.c, http.MethodPost, "/v1/futures/position-margin", nil, r)
}

func (h httpFutures) TransferToSpot(
	ctx context.Context, r *futuresv1.TransferToSpotRequest,
) (*futuresv1.TransferResponse, error) {
	return call[futuresv1.TransferResponse](ctx, h.c, http.MethodPost, "/v1/futures/transfers/spot", nil, r)
}

func (h httpFutures) ListIncome(
	ctx context.Context, r *futuresv1.ListIncomeRequest,
) (*futuresv1.ListIncomeResponse, error) {
	return call[futuresv1.ListIncomeResponse](ctx, h.c, http.MethodGet, "/v1/futures/users/"+seg(r.GetUserId())+"/income",
		query("symbol", r.GetSymbol(), "income_type", r.GetIncomeType(), "limit", itoa(r.GetLimit())), nil)
}

func (h httpFutures) PlaceOrder(
	ctx context.Context, r *futuresv1.PlaceOrderRequest,
) (*futuresv1.Order, error) {
	return call[futuresv1.Order](ctx, h.c, http.MethodPost, "/v1/futures/orders", nil, r)
}

func (h httpFutures) CancelOrder(
	ctx context.Context, r *futuresv1.CancelOrderRequest,
) (*futuresv1.Order, error) {
	return call[futuresv1.Order](ctx, h.c, http.MethodPost, "/v1/futures/orders/"+seg(r.GetOrderId())+"/cancel", nil, r)
}

func (h httpFutures) ClosePosition(
	ctx context.Context, r *futuresv1.ClosePositionRequest,
) (*futuresv1.Order, error) {
	return call[futuresv1.Order](ctx, h.c, http.MethodPost, "/v1/futures/close-position", nil, r)
}

func (h httpFutures) GetOrder(
	ctx context.Context, r *futuresv1.GetOrderRequest,
) (*futuresv1.Order, error) {
	return call[futuresv1.Order](ctx, h.c, http.MethodGet, "/v1/futures/orders/"+seg(r.GetOrderId()),
		query("user_id", r.GetUserId()), nil)
}

func (h httpFutures) ListOpenOrders(
	ctx context.Context, r *futuresv1.ListOpenOrdersRequest,
) (*futuresv1.ListOrdersResponse, error) {
	return call[futuresv1.ListOrdersResponse](ctx, h.c, http.MethodGet, "/v1/futures/users/"+seg(r.GetUserId())+"/open-orders",
		query("symbol", r.GetSymbol()), nil)
}

func (h httpFutures) ListOrders(
	ctx context.Context, r *futuresv1.ListOrdersRequest,
) (*futuresv1.ListOrdersResponse, error) {
	return call[futuresv1.ListOrdersResponse](ctx, h.c, http.MethodGet, "/v1/futures/users/"+seg(r.GetUserId())+"/orders",
		query("symbol", r.GetSymbol(), "limit", itoa(r.GetLimit())), nil)
}

func (h httpFutures) ListTrades(
	ctx context.Context, r *futuresv1.ListTradesRequest,
) (*futuresv1.ListTradesResponse, error) {
	return call[futuresv1.ListTradesResponse](ctx, h.c, http.MethodGet, "/v1/futures/users/"+seg(r.GetUserId())+"/trades",
		query("symbol", r.GetSymbol(), "limit", itoa(r.GetLimit())), nil)
}
