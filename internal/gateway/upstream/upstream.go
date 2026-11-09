// Package upstream holds the gateway's clients for the internal services. Every service is reachable
// over gRPC and over HTTP/JSON (grpc-gateway); both implementations return gRPC status errors so the
// gateway handlers treat them identically.
package upstream

import (
	"context"
	"fmt"
	"time"

	futuresv1 "simple-securities/gen/futures/v1"
	identityv1 "simple-securities/gen/identity/v1"
	marketdatav1 "simple-securities/gen/marketdata/v1"
	orderv1 "simple-securities/gen/order/v1"
	walletv1 "simple-securities/gen/wallet/v1"
	"simple-securities/pkg/config"
)

// CallTimeout bounds every upstream call.
const CallTimeout = 5 * time.Second

// Identity is the subset of the identity service used by the gateway.
type Identity interface {
	Register(ctx context.Context, req *identityv1.RegisterRequest) (*identityv1.AuthResponse, error)
	Login(ctx context.Context, req *identityv1.LoginRequest) (*identityv1.AuthResponse, error)
	Refresh(ctx context.Context, req *identityv1.RefreshRequest) (*identityv1.AuthResponse, error)
	Logout(ctx context.Context, req *identityv1.LogoutRequest) (*identityv1.LogoutResponse, error)
}

// Wallet is the subset of the wallet service used by the gateway.
type Wallet interface {
	GetBalances(ctx context.Context, req *walletv1.GetBalancesRequest) (*walletv1.GetBalancesResponse, error)
	Deposit(ctx context.Context, req *walletv1.DepositRequest) (*walletv1.LedgerEntry, error)
	Withdraw(ctx context.Context, req *walletv1.WithdrawRequest) (*walletv1.LedgerEntry, error)
	ListLedger(ctx context.Context, req *walletv1.ListLedgerRequest) (*walletv1.ListLedgerResponse, error)
	TransferToFutures(ctx context.Context, req *walletv1.TransferToFuturesRequest) (*walletv1.LedgerEntry, error)
}

// Order is the subset of the order service used by the gateway.
type Order interface {
	PlaceOrder(ctx context.Context, req *orderv1.PlaceOrderRequest) (*orderv1.Order, error)
	CancelOrder(ctx context.Context, req *orderv1.CancelOrderRequest) (*orderv1.Order, error)
	GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.Order, error)
	ListOpenOrders(ctx context.Context, req *orderv1.ListOpenOrdersRequest) (*orderv1.ListOrdersResponse, error)
	ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error)
	ListTrades(ctx context.Context, req *orderv1.ListTradesRequest) (*orderv1.ListTradesResponse, error)
}

// MarketData is the subset of the market data service used by the gateway.
type MarketData interface {
	GetExchangeInfo(ctx context.Context, req *marketdatav1.GetExchangeInfoRequest) (*marketdatav1.ExchangeInfo, error)
	GetTickers(ctx context.Context, req *marketdatav1.GetTickersRequest) (*marketdatav1.GetTickersResponse, error)
	GetKlines(ctx context.Context, req *marketdatav1.GetKlinesRequest) (*marketdatav1.GetKlinesResponse, error)
	GetDepth(ctx context.Context, req *marketdatav1.GetDepthRequest) (*marketdatav1.Depth, error)
	GetTrades(ctx context.Context, req *marketdatav1.GetTradesRequest) (*marketdatav1.GetTradesResponse, error)
}

// Futures is the subset of the futures service used by the gateway.
type Futures interface {
	GetMarkPrices(ctx context.Context, req *futuresv1.GetMarkPricesRequest) (*futuresv1.GetMarkPricesResponse, error)
	ListFundingRates(ctx context.Context, req *futuresv1.ListFundingRatesRequest) (*futuresv1.ListFundingRatesResponse, error)
	GetAccount(ctx context.Context, req *futuresv1.GetAccountRequest) (*futuresv1.Account, error)
	ListPositions(ctx context.Context, req *futuresv1.ListPositionsRequest) (*futuresv1.ListPositionsResponse, error)
	SetLeverage(ctx context.Context, req *futuresv1.SetLeverageRequest) (*futuresv1.SymbolSetting, error)
	SetMarginType(ctx context.Context, req *futuresv1.SetMarginTypeRequest) (*futuresv1.SymbolSetting, error)
	AdjustIsolatedMargin(ctx context.Context, req *futuresv1.AdjustIsolatedMarginRequest) (*futuresv1.Position, error)
	TransferToSpot(ctx context.Context, req *futuresv1.TransferToSpotRequest) (*futuresv1.TransferResponse, error)
	ListIncome(ctx context.Context, req *futuresv1.ListIncomeRequest) (*futuresv1.ListIncomeResponse, error)
	PlaceOrder(ctx context.Context, req *futuresv1.PlaceOrderRequest) (*futuresv1.Order, error)
	CancelOrder(ctx context.Context, req *futuresv1.CancelOrderRequest) (*futuresv1.Order, error)
	ClosePosition(ctx context.Context, req *futuresv1.ClosePositionRequest) (*futuresv1.Order, error)
	GetOrder(ctx context.Context, req *futuresv1.GetOrderRequest) (*futuresv1.Order, error)
	ListOpenOrders(ctx context.Context, req *futuresv1.ListOpenOrdersRequest) (*futuresv1.ListOrdersResponse, error)
	ListOrders(ctx context.Context, req *futuresv1.ListOrdersRequest) (*futuresv1.ListOrdersResponse, error)
	ListTrades(ctx context.Context, req *futuresv1.ListTradesRequest) (*futuresv1.ListTradesResponse, error)
}

// Clients bundles every upstream client.
type Clients struct {
	Identity   Identity
	Wallet     Wallet
	Order      Order
	MarketData MarketData
	Futures    Futures
	// FuturesMarketData is the marketdata instance of the futures market (MARKET=futures).
	FuturesMarketData MarketData

	closers []func() error
}

// Close releases the underlying connections.
func (c *Clients) Close() error {
	var first error
	for _, fn := range c.closers {
		if err := fn(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Config selects the transport and the addresses of each service.
type Config struct {
	Transport string // grpc | http

	IdentityGRPC, WalletGRPC, OrderGRPC, MarketDataGRPC string
	IdentityHTTP, WalletHTTP, OrderHTTP, MarketDataHTTP string

	FuturesGRPC, FuturesMarketDataGRPC string
	FuturesHTTP, FuturesMarketDataHTTP string
}

// ConfigFromEnv reads the upstream configuration from the environment.
func ConfigFromEnv() Config {
	return Config{
		Transport:      config.String("UPSTREAM_TRANSPORT", "grpc"),
		IdentityGRPC:   config.String("IDENTITY_GRPC_ADDR", "localhost:9101"),
		WalletGRPC:     config.String("WALLET_GRPC_ADDR", "localhost:9102"),
		OrderGRPC:      config.String("ORDER_GRPC_ADDR", "localhost:9103"),
		MarketDataGRPC: config.String("MARKETDATA_GRPC_ADDR", "localhost:9105"),
		IdentityHTTP:   config.String("IDENTITY_HTTP_URL", "http://localhost:8101"),
		WalletHTTP:     config.String("WALLET_HTTP_URL", "http://localhost:8102"),
		OrderHTTP:      config.String("ORDER_HTTP_URL", "http://localhost:8103"),
		MarketDataHTTP: config.String("MARKETDATA_HTTP_URL", "http://localhost:8105"),

		FuturesGRPC:           config.String("FUTURES_GRPC_ADDR", "localhost:9106"),
		FuturesHTTP:           config.String("FUTURES_HTTP_URL", "http://localhost:8106"),
		FuturesMarketDataGRPC: config.String("FUTURES_MARKETDATA_GRPC_ADDR", "localhost:9108"),
		FuturesMarketDataHTTP: config.String("FUTURES_MARKETDATA_HTTP_URL", "http://localhost:8108"),
	}
}

// New builds the clients for the configured transport.
func New(cfg Config) (*Clients, error) {
	switch cfg.Transport {
	case "grpc", "":
		return newGRPC(cfg)
	case "http":
		return newHTTP(cfg), nil
	default:
		return nil, fmt.Errorf("unknown UPSTREAM_TRANSPORT %q (want grpc|http)", cfg.Transport)
	}
}
