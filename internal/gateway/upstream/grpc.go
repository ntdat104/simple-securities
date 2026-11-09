package upstream

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"

	futuresv1 "simple-securities/gen/futures/v1"
	identityv1 "simple-securities/gen/identity/v1"
	marketdatav1 "simple-securities/gen/marketdata/v1"
	orderv1 "simple-securities/gen/order/v1"
	walletv1 "simple-securities/gen/wallet/v1"
	"simple-securities/pkg/grpcx"
)

// timeoutConn applies a per-call deadline to every unary RPC.
type timeoutConn struct {
	cc      *grpc.ClientConn
	timeout time.Duration
}

var _ grpc.ClientConnInterface = timeoutConn{}

func (t timeoutConn) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	return t.cc.Invoke(ctx, method, args, reply, opts...)
}

func (t timeoutConn) NewStream(
	ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	return t.cc.NewStream(ctx, desc, method, opts...)
}

func newGRPC(cfg Config) (*Clients, error) {
	c := &Clients{}
	dial := func(name, addr string) (grpc.ClientConnInterface, error) {
		cc, err := grpcx.Dial(addr)
		if err != nil {
			return nil, fmt.Errorf("dial %s %s: %w", name, addr, err)
		}
		c.closers = append(c.closers, cc.Close)
		return timeoutConn{cc: cc, timeout: CallTimeout}, nil
	}
	idc, err := dial("identity", cfg.IdentityGRPC)
	if err != nil {
		return nil, err
	}
	wc, err := dial("wallet", cfg.WalletGRPC)
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	oc, err := dial("order", cfg.OrderGRPC)
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	mc, err := dial("marketdata", cfg.MarketDataGRPC)
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	fc, err := dial("futures", cfg.FuturesGRPC)
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	fmc, err := dial("futures marketdata", cfg.FuturesMarketDataGRPC)
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	c.Identity = grpcIdentity{c: identityv1.NewIdentityServiceClient(idc)}
	c.Wallet = grpcWallet{c: walletv1.NewWalletServiceClient(wc)}
	c.Order = grpcOrder{c: orderv1.NewOrderServiceClient(oc)}
	c.MarketData = grpcMarketData{c: marketdatav1.NewMarketDataServiceClient(mc)}
	c.Futures = grpcFutures{c: futuresv1.NewFuturesServiceClient(fc)}
	c.FuturesMarketData = grpcMarketData{c: marketdatav1.NewMarketDataServiceClient(fmc)}
	return c, nil
}

type grpcIdentity struct {
	c identityv1.IdentityServiceClient
}

func (g grpcIdentity) Register(ctx context.Context, r *identityv1.RegisterRequest) (*identityv1.AuthResponse, error) {
	return g.c.Register(ctx, r)
}

func (g grpcIdentity) Login(ctx context.Context, r *identityv1.LoginRequest) (*identityv1.AuthResponse, error) {
	return g.c.Login(ctx, r)
}

func (g grpcIdentity) Refresh(ctx context.Context, r *identityv1.RefreshRequest) (*identityv1.AuthResponse, error) {
	return g.c.Refresh(ctx, r)
}

func (g grpcIdentity) Logout(ctx context.Context, r *identityv1.LogoutRequest) (*identityv1.LogoutResponse, error) {
	return g.c.Logout(ctx, r)
}

type grpcWallet struct{ c walletv1.WalletServiceClient }

func (g grpcWallet) GetBalances(
	ctx context.Context, r *walletv1.GetBalancesRequest,
) (*walletv1.GetBalancesResponse, error) {
	return g.c.GetBalances(ctx, r)
}

func (g grpcWallet) Deposit(ctx context.Context, r *walletv1.DepositRequest) (*walletv1.LedgerEntry, error) {
	return g.c.Deposit(ctx, r)
}

func (g grpcWallet) Withdraw(ctx context.Context, r *walletv1.WithdrawRequest) (*walletv1.LedgerEntry, error) {
	return g.c.Withdraw(ctx, r)
}

func (g grpcWallet) ListLedger(
	ctx context.Context, r *walletv1.ListLedgerRequest,
) (*walletv1.ListLedgerResponse, error) {
	return g.c.ListLedger(ctx, r)
}

func (g grpcWallet) TransferToFutures(
	ctx context.Context, r *walletv1.TransferToFuturesRequest,
) (*walletv1.LedgerEntry, error) {
	return g.c.TransferToFutures(ctx, r)
}

type grpcOrder struct{ c orderv1.OrderServiceClient }

func (g grpcOrder) PlaceOrder(ctx context.Context, r *orderv1.PlaceOrderRequest) (*orderv1.Order, error) {
	return g.c.PlaceOrder(ctx, r)
}

func (g grpcOrder) CancelOrder(ctx context.Context, r *orderv1.CancelOrderRequest) (*orderv1.Order, error) {
	return g.c.CancelOrder(ctx, r)
}

func (g grpcOrder) GetOrder(ctx context.Context, r *orderv1.GetOrderRequest) (*orderv1.Order, error) {
	return g.c.GetOrder(ctx, r)
}

func (g grpcOrder) ListOpenOrders(
	ctx context.Context, r *orderv1.ListOpenOrdersRequest,
) (*orderv1.ListOrdersResponse, error) {
	return g.c.ListOpenOrders(ctx, r)
}

func (g grpcOrder) ListOrders(ctx context.Context, r *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	return g.c.ListOrders(ctx, r)
}

func (g grpcOrder) ListTrades(ctx context.Context, r *orderv1.ListTradesRequest) (*orderv1.ListTradesResponse, error) {
	return g.c.ListTrades(ctx, r)
}

type grpcMarketData struct {
	c marketdatav1.MarketDataServiceClient
}

func (g grpcMarketData) GetExchangeInfo(
	ctx context.Context, r *marketdatav1.GetExchangeInfoRequest,
) (*marketdatav1.ExchangeInfo, error) {
	return g.c.GetExchangeInfo(ctx, r)
}

func (g grpcMarketData) GetTickers(
	ctx context.Context, r *marketdatav1.GetTickersRequest,
) (*marketdatav1.GetTickersResponse, error) {
	return g.c.GetTickers(ctx, r)
}

func (g grpcMarketData) GetKlines(
	ctx context.Context, r *marketdatav1.GetKlinesRequest,
) (*marketdatav1.GetKlinesResponse, error) {
	return g.c.GetKlines(ctx, r)
}

func (g grpcMarketData) GetDepth(ctx context.Context, r *marketdatav1.GetDepthRequest) (*marketdatav1.Depth, error) {
	return g.c.GetDepth(ctx, r)
}

func (g grpcMarketData) GetTrades(
	ctx context.Context, r *marketdatav1.GetTradesRequest,
) (*marketdatav1.GetTradesResponse, error) {
	return g.c.GetTrades(ctx, r)
}
