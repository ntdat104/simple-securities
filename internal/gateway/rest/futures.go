package rest

import (
	"net/http"
	"strings"
	"time"

	futuresv1 "simple-securities/gen/futures/v1"
	walletv1 "simple-securities/gen/wallet/v1"
	"simple-securities/internal/gateway/api"
	"simple-securities/pkg/apperr"
	"simple-securities/pkg/symbols"
)

// futuresRoutes is the USDⓈ-M futures API (/fapi/v1), docs/futures.md section 4. Same auth, error
// mapping and middleware as /api/v1.
func (s *Server) futuresRoutes() []route {
	const p = "/fapi/v1"
	md := s.ups.FuturesMarketData
	return []route{
		{"GET " + p + "/exchangeInfo", s.public(s.futuresExchangeInfo)},
		{"GET " + p + "/depth", s.public(withMD(md, depth))},
		{"GET " + p + "/trades", s.public(withMD(md, trades))},
		{"GET " + p + "/klines", s.public(withMD(md, klines))},
		{"GET " + p + "/ticker/24hr", s.public(withMD(md, ticker24h))},
		{"GET " + p + "/premiumIndex", s.public(s.premiumIndex)},
		{"GET " + p + "/fundingRate", s.public(s.fundingRate)},
		{"GET " + p + "/account", s.authed(s.futuresAccount)},
		{"GET " + p + "/positionRisk", s.authed(s.positionRisk)},
		{"POST " + p + "/leverage", s.authed(s.setLeverage)},
		{"POST " + p + "/marginType", s.authed(s.setMarginType)},
		{"POST " + p + "/positionMargin", s.authed(s.positionMargin)},
		{"POST " + p + "/order", s.authed(s.futuresPlaceOrder)},
		{"DELETE " + p + "/order", s.authed(s.futuresCancelOrder)},
		{"GET " + p + "/order", s.authed(s.futuresGetOrder)},
		{"GET " + p + "/openOrders", s.authed(s.futuresOpenOrders)},
		{"GET " + p + "/allOrders", s.authed(s.futuresAllOrders)},
		{"POST " + p + "/closePosition", s.authed(s.closePosition)},
		{"GET " + p + "/userTrades", s.authed(s.futuresUserTrades)},
		{"GET " + p + "/income", s.authed(s.income)},
		{"POST " + p + "/transfer", s.authed(s.futuresTransfer)},
	}
}

// ---------------------------------------------------------------- market

func (s *Server) futuresExchangeInfo(w http.ResponseWriter, _ *http.Request) error {
	return ok(w, api.FuturesExchangeInfoFrom(symbols.AllMarket(symbols.MarketFutures), time.Now()))
}

// premiumIndex returns one object when symbol is given, otherwise an array of every contract.
func (s *Server) premiumIndex(w http.ResponseWriter, r *http.Request) error {
	sym, err := api.OptionalSymbol(r.URL.Query().Get("symbol"))
	if err != nil {
		return err
	}
	res, err := s.ups.Futures.GetMarkPrices(r.Context(), &futuresv1.GetMarkPricesRequest{Symbol: sym})
	if err != nil {
		return err
	}
	if sym == "" {
		return ok(w, api.PremiumIndexesFrom(res.GetPrices()))
	}
	for _, m := range res.GetPrices() {
		if m.GetSymbol() == sym {
			return ok(w, api.PremiumIndexFrom(m))
		}
	}
	return apperr.NotFound("SYMBOL_NOT_FOUND", "no mark price for %s", sym)
}

func (s *Server) fundingRate(w http.ResponseWriter, r *http.Request) error {
	sym, err := api.OptionalSymbol(r.URL.Query().Get("symbol"))
	if err != nil {
		return err
	}
	limit, err := limitParam(r)
	if err != nil {
		return err
	}
	res, err := s.ups.Futures.ListFundingRates(r.Context(), &futuresv1.ListFundingRatesRequest{Symbol: sym, Limit: limit})
	if err != nil {
		return err
	}
	return ok(w, api.FundingRatesFrom(res.GetRates()))
}

// ---------------------------------------------------------------- account

func (s *Server) futuresAccount(w http.ResponseWriter, r *http.Request, uid string) error {
	res, err := s.ups.Futures.GetAccount(r.Context(), &futuresv1.GetAccountRequest{UserId: uid})
	if err != nil {
		return err
	}
	return ok(w, api.FuturesAccountFrom(res))
}

func (s *Server) positionRisk(w http.ResponseWriter, r *http.Request, uid string) error {
	sym, err := api.OptionalSymbol(r.URL.Query().Get("symbol"))
	if err != nil {
		return err
	}
	res, err := s.ups.Futures.ListPositions(r.Context(), &futuresv1.ListPositionsRequest{UserId: uid, Symbol: sym})
	if err != nil {
		return err
	}
	return ok(w, api.PositionsFrom(res.GetPositions()))
}

type leverageBody struct {
	Symbol   string      `json:"symbol"`
	Leverage api.FlexInt `json:"leverage"`
}

func (s *Server) setLeverage(w http.ResponseWriter, r *http.Request, uid string) error {
	var b leverageBody
	if err := decode(w, r, &b); err != nil {
		return err
	}
	sym, err := api.NormalizeSymbol(b.Symbol)
	if err != nil {
		return err
	}
	if b.Leverage < 1 || b.Leverage > 1000 {
		return apperr.Invalid("INVALID_LEVERAGE", "leverage must be a positive integer")
	}
	res, err := s.ups.Futures.SetLeverage(r.Context(), &futuresv1.SetLeverageRequest{
		UserId: uid, Symbol: sym, Leverage: int32(b.Leverage),
	})
	if err != nil {
		return err
	}
	return ok(w, api.LeverageResult{Symbol: res.GetSymbol(), Leverage: res.GetLeverage(), MaxNotional: res.GetMaxNotional()})
}

type marginTypeBody struct {
	Symbol     string `json:"symbol"`
	MarginType string `json:"marginType"`
}

func (s *Server) setMarginType(w http.ResponseWriter, r *http.Request, uid string) error {
	var b marginTypeBody
	if err := decode(w, r, &b); err != nil {
		return err
	}
	sym, err := api.NormalizeSymbol(b.Symbol)
	if err != nil {
		return err
	}
	mt, err := api.ParseMarginType(b.MarginType)
	if err != nil {
		return err
	}
	res, err := s.ups.Futures.SetMarginType(r.Context(), &futuresv1.SetMarginTypeRequest{
		UserId: uid, Symbol: sym, MarginType: mt,
	})
	if err != nil {
		return err
	}
	return ok(w, api.MarginTypeResult{Symbol: res.GetSymbol(), MarginType: res.GetMarginType()})
}

type positionMarginBody struct {
	Symbol string `json:"symbol"`
	Amount string `json:"amount"`
	Type   string `json:"type"`
}

func (s *Server) positionMargin(w http.ResponseWriter, r *http.Request, uid string) error {
	var b positionMarginBody
	if err := decode(w, r, &b); err != nil {
		return err
	}
	sym, err := api.NormalizeSymbol(b.Symbol)
	if err != nil {
		return err
	}
	typ := strings.ToUpper(strings.TrimSpace(b.Type))
	if typ != "ADD" && typ != "REDUCE" {
		return apperr.Invalid("INVALID_PARAMETER", "type must be ADD or REDUCE")
	}
	amount := strings.TrimSpace(b.Amount)
	if amount == "" {
		return apperr.Invalid("INVALID_PARAMETER", "amount is required")
	}
	res, err := s.ups.Futures.AdjustIsolatedMargin(r.Context(), &futuresv1.AdjustIsolatedMarginRequest{
		UserId: uid, Symbol: sym, Amount: amount, Type: typ,
	})
	if err != nil {
		return err
	}
	return ok(w, api.PositionFrom(res))
}

func (s *Server) income(w http.ResponseWriter, r *http.Request, uid string) error {
	sym, err := api.OptionalSymbol(r.URL.Query().Get("symbol"))
	if err != nil {
		return err
	}
	limit, err := limitParam(r)
	if err != nil {
		return err
	}
	res, err := s.ups.Futures.ListIncome(r.Context(), &futuresv1.ListIncomeRequest{
		UserId: uid, Symbol: sym, Limit: limit,
		IncomeType: strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("incomeType"))),
	})
	if err != nil {
		return err
	}
	return ok(w, api.IncomeFrom(res.GetIncome()))
}

type transferBody struct {
	Asset          string `json:"asset"`
	Amount         string `json:"amount"`
	Type           string `json:"type"`
	IdempotencyKey string `json:"idempotencyKey"`
}

// futuresTransfer moves USDT between the spot wallet and the futures account. The source side is
// debited synchronously; the destination is credited asynchronously (status PENDING).
func (s *Server) futuresTransfer(w http.ResponseWriter, r *http.Request, uid string) error {
	var b transferBody
	if err := decode(w, r, &b); err != nil {
		return err
	}
	asset := strings.ToUpper(strings.TrimSpace(b.Asset))
	if asset == "" {
		return apperr.Invalid("INVALID_PARAMETER", "asset is required")
	}
	amount := strings.TrimSpace(b.Amount)
	if amount == "" {
		return apperr.Invalid("INVALID_PARAMETER", "amount is required")
	}
	key := idempotencyKey(r, b.IdempotencyKey)
	switch typ := strings.ToUpper(strings.TrimSpace(b.Type)); typ {
	case api.TransferSpotToFutures:
		e, err := s.ups.Wallet.TransferToFutures(r.Context(), &walletv1.TransferToFuturesRequest{
			UserId: uid, Asset: asset, Amount: amount, IdempotencyKey: key,
		})
		if err != nil {
			return err
		}
		return ok(w, api.TransferResult{TranID: e.GetRefId(), Asset: e.GetAsset(),
			Amount: strings.TrimPrefix(e.GetDeltaFree(), "-"), Type: typ, Status: "PENDING"})
	case api.TransferFuturesToSpot:
		res, err := s.ups.Futures.TransferToSpot(r.Context(), &futuresv1.TransferToSpotRequest{
			UserId: uid, Asset: asset, Amount: amount, IdempotencyKey: key,
		})
		if err != nil {
			return err
		}
		return ok(w, api.TransferResult{TranID: res.GetTransferId(), Asset: res.GetAsset(), Amount: res.GetAmount(),
			Type: typ, Status: res.GetStatus()})
	default:
		return apperr.Invalid("INVALID_PARAMETER", "type must be %s or %s", api.TransferSpotToFutures,
			api.TransferFuturesToSpot)
	}
}

// ---------------------------------------------------------------- orders

func (s *Server) futuresPlaceOrder(w http.ResponseWriter, r *http.Request, uid string) error {
	var b api.FuturesPlaceOrderRequest
	if err := decode(w, r, &b); err != nil {
		return err
	}
	o, err := s.trader.PlaceFutures(r.Context(), uid, b)
	if err != nil {
		return err
	}
	return ok(w, o)
}

func (s *Server) futuresCancelOrder(w http.ResponseWriter, r *http.Request, uid string) error {
	o, err := s.trader.CancelFutures(r.Context(), uid, r.URL.Query().Get("orderId"))
	if err != nil {
		return err
	}
	return ok(w, o)
}

func (s *Server) futuresGetOrder(w http.ResponseWriter, r *http.Request, uid string) error {
	id, err := required(r, "orderId")
	if err != nil {
		return err
	}
	o, err := s.ups.Futures.GetOrder(r.Context(), &futuresv1.GetOrderRequest{UserId: uid, OrderId: id})
	if err != nil {
		return err
	}
	return ok(w, api.FuturesOrderFrom(o))
}

func (s *Server) futuresOpenOrders(w http.ResponseWriter, r *http.Request, uid string) error {
	sym, err := api.OptionalSymbol(r.URL.Query().Get("symbol"))
	if err != nil {
		return err
	}
	res, err := s.ups.Futures.ListOpenOrders(r.Context(), &futuresv1.ListOpenOrdersRequest{UserId: uid, Symbol: sym})
	if err != nil {
		return err
	}
	return ok(w, api.FuturesOrdersFrom(res.GetOrders()))
}

func (s *Server) futuresAllOrders(w http.ResponseWriter, r *http.Request, uid string) error {
	sym, err := api.OptionalSymbol(r.URL.Query().Get("symbol"))
	if err != nil {
		return err
	}
	limit, err := limitParam(r)
	if err != nil {
		return err
	}
	res, err := s.ups.Futures.ListOrders(r.Context(), &futuresv1.ListOrdersRequest{
		UserId: uid, Symbol: sym, Limit: limit,
	})
	if err != nil {
		return err
	}
	return ok(w, api.FuturesOrdersFrom(res.GetOrders()))
}

type closePositionBody struct {
	Symbol string `json:"symbol"`
}

func (s *Server) closePosition(w http.ResponseWriter, r *http.Request, uid string) error {
	var b closePositionBody
	if err := decode(w, r, &b); err != nil {
		return err
	}
	sym, err := api.NormalizeSymbol(b.Symbol)
	if err != nil {
		return err
	}
	o, err := s.ups.Futures.ClosePosition(r.Context(), &futuresv1.ClosePositionRequest{UserId: uid, Symbol: sym})
	if err != nil {
		return err
	}
	return ok(w, api.FuturesOrderFrom(o))
}

func (s *Server) futuresUserTrades(w http.ResponseWriter, r *http.Request, uid string) error {
	sym, err := api.OptionalSymbol(r.URL.Query().Get("symbol"))
	if err != nil {
		return err
	}
	limit, err := limitParam(r)
	if err != nil {
		return err
	}
	res, err := s.ups.Futures.ListTrades(r.Context(), &futuresv1.ListTradesRequest{UserId: uid, Symbol: sym, Limit: limit})
	if err != nil {
		return err
	}
	return ok(w, api.FuturesUserTradesFrom(res.GetTrades()))
}
