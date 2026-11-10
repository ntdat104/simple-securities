package futures

import (
	"context"
	"strings"
	"time"

	futuresv1 "simple-securities/gen/futures/v1"
)

// Server adapts Service to the generated gRPC (and grpc-gateway) FuturesService API.
type Server struct {
	futuresv1.UnimplementedFuturesServiceServer
	svc *Service
}

// NewServer returns the gRPC server implementation.
func NewServer(svc *Service) *Server { return &Server{svc: svc} }

func normSymbol(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// GetMarkPrices implements futuresv1.FuturesServiceServer.
func (s *Server) GetMarkPrices(_ context.Context, r *futuresv1.GetMarkPricesRequest) (*futuresv1.GetMarkPricesResponse, error) {
	marks, err := s.svc.MarkPrices(normSymbol(r.GetSymbol()))
	if err != nil {
		return nil, err
	}
	out := &futuresv1.GetMarkPricesResponse{Prices: make([]*futuresv1.MarkPrice, 0, len(marks))}
	for _, m := range marks {
		out.Prices = append(out.Prices, &futuresv1.MarkPrice{
			Symbol: m.Symbol, MarkPrice: m.Mark.String(), IndexPrice: m.Index.String(),
			LastFundingRate: m.FundingRate.String(), NextFundingTime: m.NextFundingTime, Time: m.Time,
		})
	}
	return out, nil
}

// ListFundingRates implements futuresv1.FuturesServiceServer.
func (s *Server) ListFundingRates(ctx context.Context, r *futuresv1.ListFundingRatesRequest) (*futuresv1.ListFundingRatesResponse, error) {
	rates, err := s.svc.ListFundingRates(ctx, normSymbol(r.GetSymbol()), int(r.GetLimit()))
	if err != nil {
		return nil, err
	}
	out := &futuresv1.ListFundingRatesResponse{Rates: make([]*futuresv1.FundingRate, 0, len(rates))}
	for _, f := range rates {
		out.Rates = append(out.Rates, &futuresv1.FundingRate{
			Symbol: f.Symbol, FundingRate: f.Rate.String(), FundingTime: f.FundingTime.UnixMilli(),
			MarkPrice: f.MarkPrice.String(),
		})
	}
	return out, nil
}

// GetAccount implements futuresv1.FuturesServiceServer.
func (s *Server) GetAccount(ctx context.Context, r *futuresv1.GetAccountRequest) (*futuresv1.Account, error) {
	v, err := s.svc.GetAccount(ctx, r.GetUserId())
	if err != nil {
		return nil, err
	}
	rk := v.Risk
	return &futuresv1.Account{
		Asset: Asset, WalletBalance: rk.WalletBalance.String(), UnrealizedPnl: rk.UPnL().String(),
		MarginBalance: rk.MarginBalance.String(), AvailableBalance: rk.Available.String(),
		MaxWithdrawAmount: rk.MaxWithdraw.String(), TotalInitialMargin: rk.TotalIM.String(),
		TotalMaintMargin: rk.TotalMM.String(), TotalOpenOrderMargin: rk.OrderMargin.String(),
		Positions: toProtoPositions(v.Positions),
	}, nil
}

// ListPositions implements futuresv1.FuturesServiceServer.
func (s *Server) ListPositions(ctx context.Context, r *futuresv1.ListPositionsRequest) (*futuresv1.ListPositionsResponse, error) {
	ps, err := s.svc.ListPositions(ctx, r.GetUserId(), normSymbol(r.GetSymbol()))
	if err != nil {
		return nil, err
	}
	return &futuresv1.ListPositionsResponse{Positions: toProtoPositions(ps)}, nil
}

// SetLeverage implements futuresv1.FuturesServiceServer.
func (s *Server) SetLeverage(ctx context.Context, r *futuresv1.SetLeverageRequest) (*futuresv1.SymbolSetting, error) {
	st, err := s.svc.SetLeverage(ctx, r.GetUserId(), r.GetSymbol(), int(r.GetLeverage()))
	if err != nil {
		return nil, err
	}
	return toProtoSetting(st), nil
}

// SetMarginType implements futuresv1.FuturesServiceServer.
func (s *Server) SetMarginType(ctx context.Context, r *futuresv1.SetMarginTypeRequest) (*futuresv1.SymbolSetting, error) {
	st, err := s.svc.SetMarginType(ctx, r.GetUserId(), r.GetSymbol(), r.GetMarginType())
	if err != nil {
		return nil, err
	}
	return toProtoSetting(st), nil
}

// AdjustIsolatedMargin implements futuresv1.FuturesServiceServer.
func (s *Server) AdjustIsolatedMargin(ctx context.Context, r *futuresv1.AdjustIsolatedMarginRequest) (*futuresv1.Position, error) {
	p, err := s.svc.AdjustIsolatedMargin(ctx, r.GetUserId(), r.GetSymbol(), r.GetAmount(), r.GetType())
	if err != nil {
		return nil, err
	}
	return toProtoPosition(p), nil
}

// TransferToSpot implements futuresv1.FuturesServiceServer.
func (s *Server) TransferToSpot(ctx context.Context, r *futuresv1.TransferToSpotRequest) (*futuresv1.TransferResponse, error) {
	t, err := s.svc.TransferToSpot(ctx, r.GetUserId(), r.GetAsset(), r.GetAmount(), r.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	return &futuresv1.TransferResponse{TransferId: t.TransferID, Asset: t.Asset, Amount: t.Amount.String(),
		Status: t.Status}, nil
}

// ListIncome implements futuresv1.FuturesServiceServer.
func (s *Server) ListIncome(ctx context.Context, r *futuresv1.ListIncomeRequest) (*futuresv1.ListIncomeResponse, error) {
	rows, err := s.svc.ListIncome(ctx, r.GetUserId(), normSymbol(r.GetSymbol()), normSymbol(r.GetIncomeType()),
		int(r.GetLimit()))
	if err != nil {
		return nil, err
	}
	out := &futuresv1.ListIncomeResponse{Income: make([]*futuresv1.Income, 0, len(rows))}
	for _, in := range rows {
		out.Income = append(out.Income, &futuresv1.Income{
			Id: in.ID, Symbol: in.Symbol, IncomeType: in.IncomeType, Income: in.Income.String(), Asset: in.Asset,
			Info: in.Info, Time: formatTime(in.Time),
		})
	}
	return out, nil
}

// PlaceOrder implements futuresv1.FuturesServiceServer.
func (s *Server) PlaceOrder(ctx context.Context, r *futuresv1.PlaceOrderRequest) (*futuresv1.Order, error) {
	o, err := s.svc.PlaceOrder(ctx, PlaceInput{
		UserID: r.GetUserId(), Symbol: r.GetSymbol(), Side: r.GetSide(), Type: r.GetType(),
		TimeInForce: r.GetTimeInForce(), Price: r.GetPrice(), Quantity: r.GetQuantity(),
		ReduceOnly: r.GetReduceOnly(), ClientOrderID: r.GetClientOrderId(),
	})
	if err != nil {
		return nil, err
	}
	return toProtoOrder(o), nil
}

// CancelOrder implements futuresv1.FuturesServiceServer.
func (s *Server) CancelOrder(ctx context.Context, r *futuresv1.CancelOrderRequest) (*futuresv1.Order, error) {
	o, err := s.svc.CancelOrder(ctx, r.GetUserId(), r.GetOrderId())
	if err != nil {
		return nil, err
	}
	return toProtoOrder(o), nil
}

// ClosePosition implements futuresv1.FuturesServiceServer.
func (s *Server) ClosePosition(ctx context.Context, r *futuresv1.ClosePositionRequest) (*futuresv1.Order, error) {
	o, err := s.svc.ClosePosition(ctx, r.GetUserId(), r.GetSymbol())
	if err != nil {
		return nil, err
	}
	return toProtoOrder(o), nil
}

// GetOrder implements futuresv1.FuturesServiceServer.
func (s *Server) GetOrder(ctx context.Context, r *futuresv1.GetOrderRequest) (*futuresv1.Order, error) {
	o, err := s.svc.GetOrder(ctx, r.GetUserId(), r.GetOrderId())
	if err != nil {
		return nil, err
	}
	return toProtoOrder(o), nil
}

// ListOpenOrders implements futuresv1.FuturesServiceServer.
func (s *Server) ListOpenOrders(ctx context.Context, r *futuresv1.ListOpenOrdersRequest) (*futuresv1.ListOrdersResponse, error) {
	orders, err := s.svc.ListOpenOrders(ctx, r.GetUserId(), normSymbol(r.GetSymbol()))
	if err != nil {
		return nil, err
	}
	return toProtoOrders(orders), nil
}

// ListOrders implements futuresv1.FuturesServiceServer.
func (s *Server) ListOrders(ctx context.Context, r *futuresv1.ListOrdersRequest) (*futuresv1.ListOrdersResponse, error) {
	orders, err := s.svc.ListOrders(ctx, r.GetUserId(), normSymbol(r.GetSymbol()), int(r.GetLimit()))
	if err != nil {
		return nil, err
	}
	return toProtoOrders(orders), nil
}

// ListTrades implements futuresv1.FuturesServiceServer.
func (s *Server) ListTrades(ctx context.Context, r *futuresv1.ListTradesRequest) (*futuresv1.ListTradesResponse, error) {
	trades, err := s.svc.ListTrades(ctx, r.GetUserId(), normSymbol(r.GetSymbol()), int(r.GetLimit()))
	if err != nil {
		return nil, err
	}
	out := &futuresv1.ListTradesResponse{Trades: make([]*futuresv1.Trade, 0, len(trades))}
	for _, t := range trades {
		out.Trades = append(out.Trades, &futuresv1.Trade{
			TradeId: t.TradeID, OrderId: t.OrderID, Symbol: t.Symbol, Side: t.Side, Price: t.Price.String(),
			Qty: t.Qty.String(), QuoteQty: t.QuoteQty.String(), RealizedPnl: t.RealizedPnl.String(),
			Commission: t.Commission.String(), CommissionAsset: t.CommissionAsset, IsMaker: t.IsMaker,
			Time: formatTime(t.Time),
		})
	}
	return out, nil
}

func toProtoOrders(orders []Order) *futuresv1.ListOrdersResponse {
	out := &futuresv1.ListOrdersResponse{Orders: make([]*futuresv1.Order, 0, len(orders))}
	for _, o := range orders {
		out.Orders = append(out.Orders, toProtoOrder(o))
	}
	return out
}

func toProtoOrder(o Order) *futuresv1.Order {
	return &futuresv1.Order{
		OrderId: o.ID, ClientOrderId: o.ClientOrderID, UserId: o.UserID, Symbol: o.Symbol, Side: o.Side,
		Type: o.Type, TimeInForce: o.TimeInForce, Price: o.Price.String(), Quantity: o.Quantity.String(),
		ExecutedQty: o.ExecutedQty.String(), CumQuote: o.CumQuote.String(), AvgPrice: o.AvgPrice.String(),
		ReduceOnly: o.ReduceOnly, Liquidation: o.Liquidation, Status: o.Status, RejectReason: o.RejectReason,
		RealizedPnl: o.RealizedPnl.String(), CreatedAt: formatTime(o.CreatedAt), UpdatedAt: formatTime(o.UpdatedAt),
	}
}

func toProtoPositions(ps []PositionView) []*futuresv1.Position {
	out := make([]*futuresv1.Position, 0, len(ps))
	for _, p := range ps {
		out = append(out, toProtoPosition(p))
	}
	return out
}

func toProtoPosition(p PositionView) *futuresv1.Position {
	updated := ""
	if !p.UpdatedAt.IsZero() {
		updated = formatTime(p.UpdatedAt)
	}
	return &futuresv1.Position{
		Symbol: p.Symbol, PositionAmt: p.Size.String(), EntryPrice: p.EntryPrice.String(),
		BreakEvenPrice: p.BreakEvenPrice.String(), MarkPrice: p.Mark.String(), UnrealizedPnl: p.UPnL.String(),
		LiquidationPrice: p.LiquidationPrice.String(), Leverage: int32(p.Leverage), MarginType: p.MarginType,
		IsolatedMargin: p.IsolatedMargin.String(), Notional: p.Notional.String(),
		InitialMargin: p.InitialMargin.String(), MaintMargin: p.MaintMargin.String(), UpdateTime: updated,
	}
}

func toProtoSetting(s SymbolSetting) *futuresv1.SymbolSetting {
	return &futuresv1.SymbolSetting{Symbol: s.Symbol, Leverage: int32(s.Leverage), MarginType: s.MarginType,
		MaxNotional: s.MaxNotional.String()}
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
