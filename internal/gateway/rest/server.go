// Package rest serves the public REST API (/api/v1 spot, /fapi/v1 futures), the static UI and mounts
// the WebSocket hub.
package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	identityv1 "simple-securities/gen/identity/v1"
	marketdatav1 "simple-securities/gen/marketdata/v1"
	orderv1 "simple-securities/gen/order/v1"
	walletv1 "simple-securities/gen/wallet/v1"
	"simple-securities/internal/gateway/api"
	"simple-securities/internal/gateway/upstream"
	"simple-securities/internal/gateway/ws"
	"simple-securities/pkg/apperr"
)

const maxBodyBytes = 1 << 20

// Config configures the HTTP surface.
type Config struct {
	Origins    []string // CORS allow-list, "*" = any
	TrustProxy bool     // use the first X-Forwarded-For hop as client IP
	WebDir     string   // serve the UI from disk (dev hot reload) instead of the embedded index
	Index      []byte   // embedded web/index.html
}

// Server is the gateway's HTTP API.
type Server struct {
	cfg      Config
	ups      *upstream.Clients
	trader   *api.Trader
	verifier ws.Verifier
	limiter  api.Limiter
	hub      http.Handler
	log      *zap.Logger
}

// NewServer returns the API server.
func NewServer(
	cfg Config, ups *upstream.Clients, trader *api.Trader, verifier ws.Verifier, limiter api.Limiter,
	hub http.Handler, log *zap.Logger,
) *Server {
	return &Server{cfg: cfg, ups: ups, trader: trader, verifier: verifier, limiter: limiter, hub: hub, log: log}
}

type route struct {
	pattern string
	h       http.Handler
}

// Handler returns the routed handler wrapped in the middleware chain.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	const p = "/api/v1"
	routes := []route{
		{"POST " + p + "/auth/register", s.public(s.register)},
		{"POST " + p + "/auth/login", s.public(s.login)},
		{"POST " + p + "/auth/refresh", s.public(s.refresh)},
		{"POST " + p + "/auth/logout", s.authed(s.logout)},
		{"GET " + p + "/exchangeInfo", s.public(s.exchangeInfo)},
		{"GET " + p + "/ticker/24hr", s.public(withMD(s.ups.MarketData, ticker24h))},
		{"GET " + p + "/klines", s.public(withMD(s.ups.MarketData, klines))},
		{"GET " + p + "/depth", s.public(withMD(s.ups.MarketData, depth))},
		{"GET " + p + "/trades", s.public(withMD(s.ups.MarketData, trades))},
		{"GET " + p + "/account", s.authed(s.account)},
		{"POST " + p + "/wallet/deposit", s.authed(s.deposit)},
		{"POST " + p + "/wallet/withdraw", s.authed(s.withdraw)},
		{"GET " + p + "/wallet/ledger", s.authed(s.ledger)},
		{"POST " + p + "/order", s.authed(s.placeOrder)},
		{"DELETE " + p + "/order", s.authed(s.cancelOrder)},
		{"GET " + p + "/order", s.authed(s.getOrder)},
		{"GET " + p + "/openOrders", s.authed(s.openOrders)},
		{"GET " + p + "/allOrders", s.authed(s.allOrders)},
		{"GET " + p + "/myTrades", s.authed(s.myTrades)},
	}
	for _, rt := range append(routes, s.futuresRoutes()...) {
		mux.Handle(rt.pattern, rt.h)
	}
	notFound := func(w http.ResponseWriter, _ *http.Request) {
		api.WriteJSON(w, http.StatusNotFound, api.ErrorBody{Code: "NOT_FOUND", Msg: "no such endpoint"})
	}
	mux.HandleFunc("/api/", notFound)
	mux.HandleFunc("/fapi/", notFound)
	if s.hub != nil {
		mux.Handle("GET /ws", s.hub)
	}
	s.mountUI(mux)

	return Chain(mux,
		RequestID,
		AccessLog(s.log),
		Recover(s.log),
		SecurityHeaders,
		CORS(s.cfg.Origins),
		IPRateLimit(s.limiter, s.cfg.TrustProxy),
	)
}

func (s *Server) mountUI(mux *http.ServeMux) {
	if s.cfg.WebDir != "" {
		fs := http.FileServer(http.Dir(s.cfg.WebDir))
		mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-cache")
			fs.ServeHTTP(w, r)
		}))
		return
	}
	index := s.cfg.Index
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}

// ---------------------------------------------------------------- plumbing

type handlerFunc func(w http.ResponseWriter, r *http.Request) error

type authedFunc func(w http.ResponseWriter, r *http.Request, userID string) error

func (s *Server) public(f handlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			s.fail(w, r, err)
		}
	})
}

// authed verifies the Bearer token locally (no network call); the user id always comes from it.
func (s *Server) authed(f authedFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || strings.TrimSpace(token) == "" {
			s.fail(w, r, apperr.Unauthenticated("UNAUTHENTICATED", "missing bearer token"))
			return
		}
		claims, err := s.verifier.Verify(strings.TrimSpace(token))
		if err != nil {
			s.fail(w, r, apperr.Unauthenticated("INVALID_TOKEN", "invalid or expired token"))
			return
		}
		setLogUser(r.Context(), claims.Subject)
		if err := f(w, r, claims.Subject); err != nil {
			s.fail(w, r, err)
		}
	})
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		api.WriteJSON(w, http.StatusRequestEntityTooLarge,
			api.ErrorBody{Code: "PAYLOAD_TOO_LARGE", Msg: "request body too large"})
		return
	}
	if api.Internal(err) {
		s.log.Warn("request failed", zap.String("path", r.URL.Path),
			zap.String("request_id", RequestIDFrom(r.Context())), zap.Error(err))
	}
	api.WriteError(w, err)
}

func ok(w http.ResponseWriter, v any) error {
	api.WriteJSON(w, http.StatusOK, v)
	return nil
}

func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return err
		}
		return apperr.Invalid("BAD_REQUEST", "invalid JSON body")
	}
	return nil
}

func intParam(r *http.Request, name string) (int64, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return 0, apperr.Invalid("INVALID_PARAMETER", "%s must be a non-negative integer", name)
	}
	return n, nil
}

func limitParam(r *http.Request) (int32, error) {
	n, err := intParam(r, "limit")
	if err != nil {
		return 0, err
	}
	if n > 5000 {
		return 0, apperr.Invalid("INVALID_PARAMETER", "limit must be <= 5000")
	}
	return int32(n), nil
}

func required(r *http.Request, name string) (string, error) {
	v := strings.TrimSpace(r.URL.Query().Get(name))
	if v == "" {
		return "", apperr.Invalid("INVALID_PARAMETER", "%s is required", name)
	}
	return v, nil
}

func idempotencyKey(r *http.Request, fromBody string) string {
	if fromBody != "" {
		return fromBody
	}
	if h := r.Header.Get("Idempotency-Key"); h != "" {
		return h
	}
	return uuid.NewString()
}

// ---------------------------------------------------------------- auth

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshBody struct {
	RefreshToken string `json:"refreshToken"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) error {
	var b credentials
	if err := decode(w, r, &b); err != nil {
		return err
	}
	res, err := s.ups.Identity.Register(r.Context(), &identityv1.RegisterRequest{
		Email: strings.TrimSpace(b.Email), Password: b.Password,
	})
	if err != nil {
		return err
	}
	return ok(w, api.AuthFrom(res))
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	var b credentials
	if err := decode(w, r, &b); err != nil {
		return err
	}
	res, err := s.ups.Identity.Login(r.Context(), &identityv1.LoginRequest{
		Email: strings.TrimSpace(b.Email), Password: b.Password,
	})
	if err != nil {
		return err
	}
	return ok(w, api.AuthFrom(res))
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) error {
	var b refreshBody
	if err := decode(w, r, &b); err != nil {
		return err
	}
	if b.RefreshToken == "" {
		return apperr.Invalid("INVALID_PARAMETER", "refreshToken is required")
	}
	res, err := s.ups.Identity.Refresh(r.Context(), &identityv1.RefreshRequest{RefreshToken: b.RefreshToken})
	if err != nil {
		return err
	}
	return ok(w, api.AuthFrom(res))
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request, _ string) error {
	var b refreshBody
	if err := decode(w, r, &b); err != nil {
		return err
	}
	if b.RefreshToken == "" {
		return apperr.Invalid("INVALID_PARAMETER", "refreshToken is required")
	}
	if _, err := s.ups.Identity.Logout(r.Context(), &identityv1.LogoutRequest{RefreshToken: b.RefreshToken}); err != nil {
		return err
	}
	return ok(w, struct{}{})
}

// ---------------------------------------------------------------- market data

func (s *Server) exchangeInfo(w http.ResponseWriter, r *http.Request) error {
	res, err := s.ups.MarketData.GetExchangeInfo(r.Context(), &marketdatav1.GetExchangeInfoRequest{})
	if err != nil {
		return err
	}
	return ok(w, api.ExchangeInfoFrom(res))
}

// mdHandler is a market data handler bound to a marketdata client, so /api (spot) and /fapi
// (futures instance) share the request parsing and response mapping.
type mdHandler func(md upstream.MarketData, w http.ResponseWriter, r *http.Request) error

func withMD(md upstream.MarketData, f mdHandler) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error { return f(md, w, r) }
}

func ticker24h(md upstream.MarketData, w http.ResponseWriter, r *http.Request) error {
	sym, err := api.OptionalSymbol(r.URL.Query().Get("symbol"))
	if err != nil {
		return err
	}
	res, err := md.GetTickers(r.Context(), &marketdatav1.GetTickersRequest{Symbol: sym})
	if err != nil {
		return err
	}
	if sym == "" {
		return ok(w, api.TickersFrom(res.GetTickers()))
	}
	for _, t := range res.GetTickers() {
		if t.GetSymbol() == sym {
			return ok(w, api.TickerFrom(t))
		}
	}
	return apperr.NotFound("SYMBOL_NOT_FOUND", "no ticker for %s", sym)
}

func klines(md upstream.MarketData, w http.ResponseWriter, r *http.Request) error {
	sym, err := api.NormalizeSymbol(r.URL.Query().Get("symbol"))
	if err != nil {
		return err
	}
	interval, err := required(r, "interval")
	if err != nil {
		return err
	}
	limit, err := limitParam(r)
	if err != nil {
		return err
	}
	end, err := intParam(r, "endTime")
	if err != nil {
		return err
	}
	res, err := md.GetKlines(r.Context(), &marketdatav1.GetKlinesRequest{
		Symbol: sym, Interval: interval, Limit: limit, EndTime: end,
	})
	if err != nil {
		return err
	}
	return ok(w, api.KlinesFrom(res.GetKlines()))
}

func depth(md upstream.MarketData, w http.ResponseWriter, r *http.Request) error {
	sym, err := api.NormalizeSymbol(r.URL.Query().Get("symbol"))
	if err != nil {
		return err
	}
	limit, err := limitParam(r)
	if err != nil {
		return err
	}
	res, err := md.GetDepth(r.Context(), &marketdatav1.GetDepthRequest{Symbol: sym, Limit: limit})
	if err != nil {
		return err
	}
	return ok(w, api.DepthFrom(res))
}

func trades(md upstream.MarketData, w http.ResponseWriter, r *http.Request) error {
	sym, err := api.NormalizeSymbol(r.URL.Query().Get("symbol"))
	if err != nil {
		return err
	}
	limit, err := limitParam(r)
	if err != nil {
		return err
	}
	res, err := md.GetTrades(r.Context(), &marketdatav1.GetTradesRequest{Symbol: sym, Limit: limit})
	if err != nil {
		return err
	}
	return ok(w, api.PublicTradesFrom(res.GetTrades()))
}

// ---------------------------------------------------------------- wallet

type depositBody struct {
	Asset          string `json:"asset"`
	Amount         string `json:"amount"`
	Address        string `json:"address"`
	IdempotencyKey string `json:"idempotencyKey"`
}

func (s *Server) account(w http.ResponseWriter, r *http.Request, uid string) error {
	res, err := s.ups.Wallet.GetBalances(r.Context(), &walletv1.GetBalancesRequest{UserId: uid})
	if err != nil {
		return err
	}
	return ok(w, api.AccountFrom(res))
}

func (s *Server) deposit(w http.ResponseWriter, r *http.Request, uid string) error {
	var b depositBody
	if err := decode(w, r, &b); err != nil {
		return err
	}
	res, err := s.ups.Wallet.Deposit(r.Context(), &walletv1.DepositRequest{
		UserId: uid, Asset: strings.ToUpper(strings.TrimSpace(b.Asset)), Amount: strings.TrimSpace(b.Amount),
		IdempotencyKey: idempotencyKey(r, b.IdempotencyKey),
	})
	if err != nil {
		return err
	}
	return ok(w, api.LedgerEntryFrom(res))
}

func (s *Server) withdraw(w http.ResponseWriter, r *http.Request, uid string) error {
	var b depositBody
	if err := decode(w, r, &b); err != nil {
		return err
	}
	res, err := s.ups.Wallet.Withdraw(r.Context(), &walletv1.WithdrawRequest{
		UserId: uid, Asset: strings.ToUpper(strings.TrimSpace(b.Asset)), Amount: strings.TrimSpace(b.Amount),
		Address: strings.TrimSpace(b.Address), IdempotencyKey: idempotencyKey(r, b.IdempotencyKey),
	})
	if err != nil {
		return err
	}
	return ok(w, api.LedgerEntryFrom(res))
}

func (s *Server) ledger(w http.ResponseWriter, r *http.Request, uid string) error {
	limit, err := limitParam(r)
	if err != nil {
		return err
	}
	before, err := intParam(r, "beforeId")
	if err != nil {
		return err
	}
	res, err := s.ups.Wallet.ListLedger(r.Context(), &walletv1.ListLedgerRequest{
		UserId: uid, Limit: limit, BeforeId: before,
	})
	if err != nil {
		return err
	}
	return ok(w, api.LedgerFrom(res.GetEntries()))
}

// ---------------------------------------------------------------- orders

func (s *Server) placeOrder(w http.ResponseWriter, r *http.Request, uid string) error {
	var b api.PlaceOrderRequest
	if err := decode(w, r, &b); err != nil {
		return err
	}
	o, err := s.trader.Place(r.Context(), uid, b)
	if err != nil {
		return err
	}
	return ok(w, o)
}

func (s *Server) cancelOrder(w http.ResponseWriter, r *http.Request, uid string) error {
	o, err := s.trader.Cancel(r.Context(), uid, r.URL.Query().Get("orderId"))
	if err != nil {
		return err
	}
	return ok(w, o)
}

func (s *Server) getOrder(w http.ResponseWriter, r *http.Request, uid string) error {
	id, err := required(r, "orderId")
	if err != nil {
		return err
	}
	o, err := s.ups.Order.GetOrder(r.Context(), &orderv1.GetOrderRequest{UserId: uid, OrderId: id})
	if err != nil {
		return err
	}
	return ok(w, api.OrderFrom(o))
}

func (s *Server) openOrders(w http.ResponseWriter, r *http.Request, uid string) error {
	sym, err := api.OptionalSymbol(r.URL.Query().Get("symbol"))
	if err != nil {
		return err
	}
	res, err := s.ups.Order.ListOpenOrders(r.Context(), &orderv1.ListOpenOrdersRequest{UserId: uid, Symbol: sym})
	if err != nil {
		return err
	}
	return ok(w, api.OrdersFrom(res.GetOrders()))
}

func (s *Server) allOrders(w http.ResponseWriter, r *http.Request, uid string) error {
	sym, err := api.OptionalSymbol(r.URL.Query().Get("symbol"))
	if err != nil {
		return err
	}
	limit, err := limitParam(r)
	if err != nil {
		return err
	}
	res, err := s.ups.Order.ListOrders(r.Context(), &orderv1.ListOrdersRequest{UserId: uid, Symbol: sym, Limit: limit})
	if err != nil {
		return err
	}
	return ok(w, api.OrdersFrom(res.GetOrders()))
}

func (s *Server) myTrades(w http.ResponseWriter, r *http.Request, uid string) error {
	sym, err := api.OptionalSymbol(r.URL.Query().Get("symbol"))
	if err != nil {
		return err
	}
	limit, err := limitParam(r)
	if err != nil {
		return err
	}
	res, err := s.ups.Order.ListTrades(r.Context(), &orderv1.ListTradesRequest{UserId: uid, Symbol: sym, Limit: limit})
	if err != nil {
		return err
	}
	return ok(w, api.MyTradesFrom(res.GetTrades()))
}
