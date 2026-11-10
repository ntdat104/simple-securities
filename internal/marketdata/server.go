package marketdata

import (
	"context"
	"strings"
	"time"

	marketdatav1 "simple-securities/gen/marketdata/v1"
	"simple-securities/pkg/apperr"
	"simple-securities/pkg/events"
	"simple-securities/pkg/symbols"
)

const (
	defaultKlineLimit = 500
	maxKlineLimit     = 1000
	defaultDepthLimit = 20
	maxDepthLimit     = 50
	defaultTradeLimit = 50
	maxTradeLimit     = 1000
)

// Server implements marketdatav1.MarketDataServiceServer.
type Server struct {
	marketdatav1.UnimplementedMarketDataServiceServer
	market string
	store  *Store
	cache  *Cache
}

// NewServer returns the gRPC service implementation for a market (spot | futures).
func NewServer(market string, store *Store, cache *Cache) *Server {
	return &Server{market: market, store: store, cache: cache}
}

// GetExchangeInfo lists every trading pair (or perpetual contract) of the market with its filters.
func (s *Server) GetExchangeInfo(context.Context, *marketdatav1.GetExchangeInfoRequest) (*marketdatav1.ExchangeInfo, error) {
	all := symbols.AllMarket(s.market)
	out := &marketdatav1.ExchangeInfo{ServerTime: time.Now().UnixMilli(),
		Symbols: make([]*marketdatav1.SymbolInfo, 0, len(all))}
	for _, sym := range all {
		out.Symbols = append(out.Symbols, &marketdatav1.SymbolInfo{
			Symbol: sym.Name, BaseAsset: sym.Base, QuoteAsset: sym.Quote,
			TickSize: sym.TickSize.String(), StepSize: sym.StepSize.String(),
			MinNotional: sym.MinNotional.String(), MakerFee: sym.MakerFee.String(),
			TakerFee: sym.TakerFee.String(), Status: "TRADING",
		})
	}
	return out, nil
}

// GetTickers returns the 24h ticker of one symbol or of all symbols.
func (s *Server) GetTickers(_ context.Context, req *marketdatav1.GetTickersRequest) (*marketdatav1.GetTickersResponse, error) {
	if req.GetSymbol() == "" {
		cached := s.cache.Tickers()
		out := &marketdatav1.GetTickersResponse{Tickers: make([]*marketdatav1.Ticker, 0, len(cached))}
		for _, t := range cached {
			out.Tickers = append(out.Tickers, tickerPB(t))
		}
		return out, nil
	}
	sym := strings.ToUpper(req.GetSymbol())
	if _, ok := symbols.GetMarket(s.market, sym); !ok {
		return nil, apperr.NotFound("INVALID_SYMBOL", "unknown symbol %s", req.GetSymbol())
	}
	t, ok := s.cache.Ticker(sym)
	if !ok {
		t = buildTicker(sym, tickerStats{}, time.Now())
	}
	return &marketdatav1.GetTickersResponse{Tickers: []*marketdatav1.Ticker{tickerPB(t)}}, nil
}

// GetKlines returns candlesticks ascending by open time.
func (s *Server) GetKlines(ctx context.Context, req *marketdatav1.GetKlinesRequest) (*marketdatav1.GetKlinesResponse, error) {
	sym, err := s.symbolArg(req.GetSymbol())
	if err != nil {
		return nil, err
	}
	iv, ok := ParseInterval(req.GetInterval())
	if !ok {
		return nil, apperr.Invalid("INVALID_INTERVAL", "invalid interval %q (1m, 5m, 15m, 1h, 4h, 1d)", req.GetInterval())
	}
	end := time.Now().Add(24 * time.Hour)
	if req.GetEndTime() > 0 {
		end = time.UnixMilli(req.GetEndTime())
	}
	ks, err := s.store.Klines(ctx, sym, iv, end, clampLimit(req.GetLimit(), defaultKlineLimit, maxKlineLimit))
	if err != nil {
		return nil, err
	}
	out := &marketdatav1.GetKlinesResponse{Klines: make([]*marketdatav1.Kline, 0, len(ks))}
	for _, k := range ks {
		out.Klines = append(out.Klines, &marketdatav1.Kline{
			OpenTime: k.OpenTime, CloseTime: k.CloseTime, Open: k.Open.String(), High: k.High.String(),
			Low: k.Low.String(), Close: k.Close.String(), Volume: k.Volume.String(),
			QuoteVolume: k.QuoteVolume.String(), TradeCount: k.Count,
		})
	}
	return out, nil
}

// GetDepth returns the cached order book snapshot (empty when none was received yet).
func (s *Server) GetDepth(_ context.Context, req *marketdatav1.GetDepthRequest) (*marketdatav1.Depth, error) {
	sym, err := s.symbolArg(req.GetSymbol())
	if err != nil {
		return nil, err
	}
	limit := clampLimit(req.GetLimit(), defaultDepthLimit, maxDepthLimit)
	d, _ := s.cache.Depth(sym)
	return &marketdatav1.Depth{Symbol: sym, LastUpdateId: d.LastUpdateID,
		Bids: levelsPB(d.Bids, limit), Asks: levelsPB(d.Asks, limit)}, nil
}

// GetTrades returns the most recent public trades, oldest first.
func (s *Server) GetTrades(ctx context.Context, req *marketdatav1.GetTradesRequest) (*marketdatav1.GetTradesResponse, error) {
	sym, err := s.symbolArg(req.GetSymbol())
	if err != nil {
		return nil, err
	}
	ts, err := s.store.Trades(ctx, sym, clampLimit(req.GetLimit(), defaultTradeLimit, maxTradeLimit))
	if err != nil {
		return nil, err
	}
	out := &marketdatav1.GetTradesResponse{Trades: make([]*marketdatav1.PublicTrade, 0, len(ts))}
	for _, t := range ts {
		out.Trades = append(out.Trades, &marketdatav1.PublicTrade{
			TradeId: t.ID, Price: t.Price.String(), Qty: t.Qty.String(), QuoteQty: t.QuoteQty.String(),
			Time: t.Time.UnixMilli(), IsBuyerMaker: t.BuyerIsMaker,
		})
	}
	return out, nil
}

func (s *Server) symbolArg(raw string) (string, error) {
	if raw == "" {
		return "", apperr.Invalid("INVALID_SYMBOL", "symbol is required")
	}
	sym := strings.ToUpper(raw)
	if _, ok := symbols.GetMarket(s.market, sym); !ok {
		return "", apperr.Invalid("INVALID_SYMBOL", "unknown symbol %s", raw)
	}
	return sym, nil
}

func clampLimit(v int32, def, maxV int) int {
	switch {
	case v <= 0:
		return def
	case int(v) > maxV:
		return maxV
	}
	return int(v)
}

func levelsPB(lv []events.PriceLevel, limit int) []*marketdatav1.PriceLevel {
	n := min(len(lv), limit)
	out := make([]*marketdatav1.PriceLevel, 0, n)
	for _, l := range lv[:n] {
		out = append(out, &marketdatav1.PriceLevel{Price: l[0], Qty: l[1]})
	}
	return out
}

func tickerPB(t events.Ticker) *marketdatav1.Ticker {
	return &marketdatav1.Ticker{
		Symbol: t.Symbol, LastPrice: t.LastPrice, OpenPrice: t.OpenPrice, HighPrice: t.HighPrice,
		LowPrice: t.LowPrice, Volume: t.Volume, QuoteVolume: t.QuoteVolume, PriceChange: t.PriceChange,
		PriceChangePercent: t.PriceChangePercent, TradeCount: t.TradeCount, CloseTime: t.CloseTime,
	}
}
