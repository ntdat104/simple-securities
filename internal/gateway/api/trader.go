package api

import (
	"context"
	"net"
	"net/http"
	"strings"

	futuresv1 "simple-securities/gen/futures/v1"
	orderv1 "simple-securities/gen/order/v1"
	"simple-securities/internal/gateway/ratelimit"
	"simple-securities/internal/gateway/upstream"
	"simple-securities/pkg/apperr"
)

const maxClientOrderIDLen = 36

// PlaceOrderRequest is the public order request (REST body and WS order.place params).
type PlaceOrderRequest struct {
	Symbol           string `json:"symbol"`
	Side             string `json:"side"`
	Type             string `json:"type"`
	TimeInForce      string `json:"timeInForce"`
	Price            string `json:"price"`
	Quantity         string `json:"quantity"`
	QuoteOrderQty    string `json:"quoteOrderQty"`
	NewClientOrderID string `json:"newClientOrderId"`
}

// CancelOrderRequest is the WS order.cancel params.
type CancelOrderRequest struct {
	OrderID string `json:"orderId"`
}

// NormalizeSymbol upper-cases and validates a symbol (letters and digits only).
func NormalizeSymbol(s string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return "", apperr.Invalid("INVALID_SYMBOL", "symbol is required")
	}
	if len(s) > 20 || strings.IndexFunc(s, func(r rune) bool { return !isUpperAlnum(r) }) >= 0 {
		return "", apperr.Invalid("INVALID_SYMBOL", "invalid symbol %q", s)
	}
	return s, nil
}

// OptionalSymbol normalizes s when present.
func OptionalSymbol(s string) (string, error) {
	if strings.TrimSpace(s) == "" {
		return "", nil
	}
	return NormalizeSymbol(s)
}

func isUpperAlnum(r rune) bool { return (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') }

// ToProto validates the request shape and binds it to the authenticated user. Business rules
// (filters, balances, type/TIF combinations) are enforced by the order service.
func (r PlaceOrderRequest) ToProto(userID string) (*orderv1.PlaceOrderRequest, error) {
	sym, err := NormalizeSymbol(r.Symbol)
	if err != nil {
		return nil, err
	}
	side := strings.ToUpper(strings.TrimSpace(r.Side))
	if side != "BUY" && side != "SELL" {
		return nil, apperr.Invalid("INVALID_SIDE", "side must be BUY or SELL")
	}
	typ := strings.ToUpper(strings.TrimSpace(r.Type))
	switch typ {
	case "LIMIT", "MARKET", "LIMIT_MAKER":
	default:
		return nil, apperr.Invalid("INVALID_ORDER_TYPE", "type must be LIMIT, MARKET or LIMIT_MAKER")
	}
	if len(r.NewClientOrderID) > maxClientOrderIDLen {
		return nil, apperr.Invalid("INVALID_PARAMETER", "newClientOrderId longer than %d", maxClientOrderIDLen)
	}
	return &orderv1.PlaceOrderRequest{
		UserId:        userID,
		Symbol:        sym,
		Side:          side,
		Type:          typ,
		TimeInForce:   strings.ToUpper(strings.TrimSpace(r.TimeInForce)),
		Price:         strings.TrimSpace(r.Price),
		Quantity:      strings.TrimSpace(r.Quantity),
		QuoteQuantity: strings.TrimSpace(r.QuoteOrderQty),
		ClientOrderId: r.NewClientOrderID,
	}, nil
}

// Limiter is the rate limiter used by the order operations.
type Limiter interface {
	Allow(ctx context.Context, rule ratelimit.Rule, subject string) ratelimit.Decision
}

// Trader places and cancels spot and futures orders for an authenticated user; shared by REST and
// WS so both get the same validation and per-user order rate limit (one budget for both markets).
type Trader struct {
	orders  upstream.Order
	futures upstream.Futures
	limiter Limiter
}

// NewTrader returns a Trader.
func NewTrader(orders upstream.Order, futures upstream.Futures, limiter Limiter) *Trader {
	return &Trader{orders: orders, futures: futures, limiter: limiter}
}

// PlaceFutures validates, rate limits (shared order budget) and forwards a new futures order.
func (t *Trader) PlaceFutures(ctx context.Context, userID string, req FuturesPlaceOrderRequest) (FuturesOrder, error) {
	p, err := req.ToProto(userID)
	if err != nil {
		return FuturesOrder{}, err
	}
	if d := t.limiter.Allow(ctx, ratelimit.PerOrder, userID); !d.Allowed {
		return FuturesOrder{}, &RateLimitError{RetryAfter: d.RetryAfter}
	}
	o, err := t.futures.PlaceOrder(ctx, p)
	if err != nil {
		return FuturesOrder{}, err
	}
	return FuturesOrderFrom(o), nil
}

// CancelFutures requests the cancellation of a futures order owned by the user.
func (t *Trader) CancelFutures(ctx context.Context, userID, orderID string) (FuturesOrder, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return FuturesOrder{}, apperr.Invalid("INVALID_PARAMETER", "orderId is required")
	}
	o, err := t.futures.CancelOrder(ctx, &futuresv1.CancelOrderRequest{UserId: userID, OrderId: orderID})
	if err != nil {
		return FuturesOrder{}, err
	}
	return FuturesOrderFrom(o), nil
}

// Place validates, rate limits and forwards a new order.
func (t *Trader) Place(ctx context.Context, userID string, req PlaceOrderRequest) (Order, error) {
	p, err := req.ToProto(userID)
	if err != nil {
		return Order{}, err
	}
	if d := t.limiter.Allow(ctx, ratelimit.PerOrder, userID); !d.Allowed {
		return Order{}, &RateLimitError{RetryAfter: d.RetryAfter}
	}
	o, err := t.orders.PlaceOrder(ctx, p)
	if err != nil {
		return Order{}, err
	}
	return OrderFrom(o), nil
}

// Cancel requests the cancellation of an order owned by the user.
func (t *Trader) Cancel(ctx context.Context, userID, orderID string) (Order, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return Order{}, apperr.Invalid("INVALID_PARAMETER", "orderId is required")
	}
	o, err := t.orders.CancelOrder(ctx, &orderv1.CancelOrderRequest{UserId: userID, OrderId: orderID})
	if err != nil {
		return Order{}, err
	}
	return OrderFrom(o), nil
}

// ClientIP returns the caller's IP: the first X-Forwarded-For hop when the proxy is trusted,
// otherwise the remote address.
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first, _, _ := strings.Cut(xff, ",")
			if ip := strings.TrimSpace(first); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
