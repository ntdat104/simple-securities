package order

import (
	"context"
	"time"

	orderv1 "simple-securities/gen/order/v1"
)

// Server adapts Service to the generated gRPC (and grpc-gateway) OrderService API.
type Server struct {
	orderv1.UnimplementedOrderServiceServer
	svc *Service
}

// NewServer returns the gRPC server implementation.
func NewServer(svc *Service) *Server { return &Server{svc: svc} }

// PlaceOrder implements orderv1.OrderServiceServer.
func (s *Server) PlaceOrder(ctx context.Context, r *orderv1.PlaceOrderRequest) (*orderv1.Order, error) {
	o, err := s.svc.PlaceOrder(ctx, PlaceInput{
		UserID: r.GetUserId(), Symbol: r.GetSymbol(), Side: r.GetSide(), Type: r.GetType(),
		TimeInForce: r.GetTimeInForce(), Price: r.GetPrice(), Quantity: r.GetQuantity(),
		QuoteQuantity: r.GetQuoteQuantity(), ClientOrderID: r.GetClientOrderId(),
	})
	if err != nil {
		return nil, err
	}
	return toProto(o), nil
}

// CancelOrder implements orderv1.OrderServiceServer.
func (s *Server) CancelOrder(ctx context.Context, r *orderv1.CancelOrderRequest) (*orderv1.Order, error) {
	o, err := s.svc.CancelOrder(ctx, r.GetUserId(), r.GetOrderId())
	if err != nil {
		return nil, err
	}
	return toProto(o), nil
}

// GetOrder implements orderv1.OrderServiceServer.
func (s *Server) GetOrder(ctx context.Context, r *orderv1.GetOrderRequest) (*orderv1.Order, error) {
	o, err := s.svc.GetOrder(ctx, r.GetUserId(), r.GetOrderId())
	if err != nil {
		return nil, err
	}
	return toProto(o), nil
}

// ListOpenOrders implements orderv1.OrderServiceServer.
func (s *Server) ListOpenOrders(ctx context.Context, r *orderv1.ListOpenOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	orders, err := s.svc.ListOpenOrders(ctx, r.GetUserId(), r.GetSymbol())
	if err != nil {
		return nil, err
	}
	return toProtoList(orders), nil
}

// ListOrders implements orderv1.OrderServiceServer.
func (s *Server) ListOrders(ctx context.Context, r *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	orders, err := s.svc.ListOrders(ctx, r.GetUserId(), r.GetSymbol(), int(r.GetLimit()))
	if err != nil {
		return nil, err
	}
	return toProtoList(orders), nil
}

// ListTrades implements orderv1.OrderServiceServer.
func (s *Server) ListTrades(ctx context.Context, r *orderv1.ListTradesRequest) (*orderv1.ListTradesResponse, error) {
	trades, err := s.svc.ListTrades(ctx, r.GetUserId(), r.GetSymbol(), int(r.GetLimit()))
	if err != nil {
		return nil, err
	}
	out := &orderv1.ListTradesResponse{Trades: make([]*orderv1.Trade, 0, len(trades))}
	for _, t := range trades {
		out.Trades = append(out.Trades, &orderv1.Trade{
			TradeId: t.TradeID, OrderId: t.OrderID, Symbol: t.Symbol, Side: t.Side,
			Price: t.Price.String(), Qty: t.Qty.String(), QuoteQty: t.QuoteQty.String(),
			Commission: t.Commission.String(), CommissionAsset: t.CommissionAsset, IsMaker: t.IsMaker,
			Time: formatTime(t.Time),
		})
	}
	return out, nil
}

func toProtoList(orders []Order) *orderv1.ListOrdersResponse {
	out := &orderv1.ListOrdersResponse{Orders: make([]*orderv1.Order, 0, len(orders))}
	for _, o := range orders {
		out.Orders = append(out.Orders, toProto(o))
	}
	return out
}

func toProto(o Order) *orderv1.Order {
	return &orderv1.Order{
		OrderId: o.ID, ClientOrderId: o.ClientOrderID, UserId: o.UserID, Symbol: o.Symbol, Side: o.Side,
		Type: o.Type, TimeInForce: o.TimeInForce, Price: o.Price.String(), Quantity: o.Quantity.String(),
		QuoteQuantity: o.QuoteQuantity.String(), ExecutedQty: o.ExecutedQty.String(),
		CumQuoteQty: o.CumQuoteQty.String(), Status: o.Status, RejectReason: o.RejectReason,
		CreatedAt: formatTime(o.CreatedAt), UpdatedAt: formatTime(o.UpdatedAt),
	}
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
