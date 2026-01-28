package grpc

import (
	"context"
	"fmt"
	"math/big"

	marketpb "simple-securities/gen/market/v1"
	"simple-securities/internal/market/application/service"
)

// MarketGrpcSvc defines the collection of services required by the gRPC handler.
type MarketGrpcSvc struct {
	// General
	PingSvc       service.PingSvc
	ServerTimeSvc service.ServerTimeSvc
	// Market Data
	ExchangeInfoSvc          service.ExchangeInfoSvc
	OrderBookSvc             service.OrderBookSvc
	RecentTradesSvc          service.RecentTradesSvc
	HistoricalTradeLookupSvc service.HistoricalTradeLookupSvc
	AggTradesListSvc         service.AggTradesListSvc
	KlinesSvc                *service.KlinesSvc
	UiKlinesSvc              service.UiKlinesSvc
	AvgPriceSvc              service.AvgPriceSvc
	Ticker24hrSvc            service.Ticker24hrSvc
	TickerPriceSvc           service.TickerPriceSvc
	TickerBookTickerSvc      service.TickerBookTickerSvc
	TickerSvc                service.TickerSvc // For Rolling Ticker
}

type MarketGrpcHandler struct {
	marketpb.UnimplementedMarketServiceServer
	// General
	pingSvc       service.PingSvc
	serverTimeSvc service.ServerTimeSvc
	// Market Data
	exchangeInfoSvc          service.ExchangeInfoSvc
	orderBookSvc             service.OrderBookSvc
	recentTradesSvc          service.RecentTradesSvc
	historicalTradeLookupSvc service.HistoricalTradeLookupSvc
	aggTradesListSvc         service.AggTradesListSvc
	klinesSvc                *service.KlinesSvc
	uiKlinesSvc              service.UiKlinesSvc
	avgPriceSvc              service.AvgPriceSvc
	ticker24hrSvc            service.Ticker24hrSvc
	tickerPriceSvc           service.TickerPriceSvc
	tickerBookTickerSvc      service.TickerBookTickerSvc
	tickerSvc                service.TickerSvc
}

func NewMarketGrpcHandler(svc MarketGrpcSvc) marketpb.MarketServiceServer {
	return &MarketGrpcHandler{
		pingSvc:                  svc.PingSvc,
		serverTimeSvc:            svc.ServerTimeSvc,
		exchangeInfoSvc:          svc.ExchangeInfoSvc,
		orderBookSvc:             svc.OrderBookSvc,
		recentTradesSvc:          svc.RecentTradesSvc,
		historicalTradeLookupSvc: svc.HistoricalTradeLookupSvc,
		aggTradesListSvc:         svc.AggTradesListSvc,
		klinesSvc:                svc.KlinesSvc,
		uiKlinesSvc:              svc.UiKlinesSvc,
		avgPriceSvc:              svc.AvgPriceSvc,
		ticker24hrSvc:            svc.Ticker24hrSvc,
		tickerPriceSvc:           svc.TickerPriceSvc,
		tickerBookTickerSvc:      svc.TickerBookTickerSvc,
		tickerSvc:                svc.TickerSvc,
	}
}

// --- Helper functions to map gRPC zero-value fields to service option pointers ---

// toPtrInt converts a non-zero int32 (gRPC field) to *int (service option).
func toPtrInt(val int32) *int {
	if val != 0 {
		v := int(val)
		return &v
	}
	return nil
}

// toPtrUint converts a non-zero uint32 (gRPC field) to *uint (service option).
func toPtrUint(val uint32) *uint {
	if val != 0 {
		v := uint(val)
		return &v
	}
	return nil
}

// toPtrString converts a non-empty string (gRPC field) to *string (service option).
func toPtrString(val string) *string {
	if val != "" {
		return &val
	}
	return nil
}

// mapOrderBookLevels maps service DTOs (using []*big.Float for precision) to gRPC types.
func mapOrderBookLevels(levels [][]*big.Float) []*marketpb.OrderLevel {
	if levels == nil {
		return nil
	}
	res := make([]*marketpb.OrderLevel, len(levels))
	for i, level := range levels {
		priceStr := ""
		qtyStr := ""

		// Expecting [price, quantity]
		if len(level) == 2 {
			// Convert price to string (using big.Float's String() method for full precision)
			if level[0] != nil {
				priceStr = level[0].String()
			}
			// Convert quantity to string
			if level[1] != nil {
				qtyStr = level[1].String()
			}
		}

		res[i] = &marketpb.OrderLevel{
			Price: priceStr,
			Qty:   qtyStr,
		}
	}
	return res
}

// --- General endpoints ---

// Ping implements marketpb.MarketServiceServer
func (h *MarketGrpcHandler) Ping(ctx context.Context, req *marketpb.PingRequest) (*marketpb.PingResponse, error) {
	err := h.pingSvc.Execute(ctx)
	if err != nil {
		return nil, err
	}
	return &marketpb.PingResponse{}, nil
}

// GetServerTime implements marketpb.MarketServiceServer
func (h *MarketGrpcHandler) GetServerTime(ctx context.Context, req *marketpb.GetServerTimeRequest) (*marketpb.GetServerTimeResponse, error) {
	result, err := h.serverTimeSvc.Execute(ctx)
	if err != nil {
		return nil, err
	}
	return &marketpb.GetServerTimeResponse{
		ServerTime: int64(result.ServerTime),
	}, nil
}

// --- Exchange Info ---

// GetExchangeInfo implements marketpb.MarketServiceServer
func (h *MarketGrpcHandler) GetExchangeInfo(ctx context.Context, req *marketpb.GetExchangeInfoRequest) (*marketpb.GetExchangeInfoResponse, error) {
	var symbols *[]string
	if len(req.GetSymbols()) > 0 {
		symbols = &req.Symbols
	}

	result, err := h.exchangeInfoSvc.Execute(
		ctx,
		req.Symbol,             // *string (optional field)
		symbols,                // *[]string (repeated field)
		req.Permissions,        // *string (optional field)
		req.ShowPermissionSets, // *bool (optional field)
		req.SymbolStatus,       // *string (optional field)
	)
	if err != nil {
		return nil, err
	}

	res := &marketpb.GetExchangeInfoResponse{
		Timezone:   result.Timezone,
		ServerTime: int64(result.ServerTime),
		RateLimits: make([]*marketpb.RateLimit, len(result.RateLimits)),
		Symbols:    make([]*marketpb.SymbolInfo, len(result.Symbols)),
	}

	for i, rl := range result.RateLimits {
		res.RateLimits[i] = &marketpb.RateLimit{
			RateLimitType: rl.RateLimitType,
			Interval:      rl.Interval,
			Limit:         int32(rl.Limit),
		}
	}

	// Map Exchange Filters
	for _, ef := range result.ExchangeFilters {
		filter := &marketpb.ExchangeFilter{
			FilterType: ef.FilterType,
		}
		// Safely parse MaxNumAlgoOrders
		if ef.MaxNumAlgo != 0 {
			filter.MaxNumAlgoOrders = ef.MaxNumAlgo
		}
		res.ExchangeFilters = append(res.ExchangeFilters, filter)
	}

	// Map Symbols
	for i, s := range result.Symbols {
		symbolInfo := &marketpb.SymbolInfo{
			Symbol:               s.Symbol,
			Status:               s.Status,
			BaseAsset:            s.BaseAsset,
			BaseAssetPrecision:   s.BaseAssetPrecision,
			QuoteAsset:           s.QuoteAsset,
			QuotePrecision:       s.QuotePrecision,
			OrderTypes:           s.OrderTypes,
			IcebergAllowed:       s.IcebergAllowed,
			IsSpotTradingAllowed: s.IsSpotTradingAllowed,
			Filters:              make([]*marketpb.SymbolFilter, len(s.Filters)),
		}

		for j, f := range s.Filters {
			symbolInfo.Filters[j] = &marketpb.SymbolFilter{
				FilterType: f.FilterType,
				MinPrice:   f.MinPrice,
				MaxPrice:   f.MaxPrice,
				TickSize:   f.TickSize,
				MinQty:     f.MinQty,
				MaxQty:     f.MaxQty,
				StepSize:   f.StepSize,
			}
		}
		res.Symbols[i] = symbolInfo
	}

	return res, nil
}

// --- Market Depth / Order Book ---

// GetOrderBook implements marketpb.MarketServiceServer
func (h *MarketGrpcHandler) GetOrderBook(ctx context.Context, req *marketpb.GetOrderBookRequest) (*marketpb.GetOrderBookResponse, error) {
	limitPtr := toPtrInt(req.Limit)

	result, err := h.orderBookSvc.Execute(ctx, req.Symbol, limitPtr)
	if err != nil {
		return nil, err
	}

	return &marketpb.GetOrderBookResponse{
		LastUpdateId: int64(result.LastUpdateId),
		Bids:         mapOrderBookLevels(result.Bids),
		Asks:         mapOrderBookLevels(result.Asks),
	}, nil
}

// --- Recent Trades ---

// GetRecentTrades implements marketpb.MarketServiceServer
func (h *MarketGrpcHandler) GetRecentTrades(ctx context.Context, req *marketpb.GetRecentTradesRequest) (*marketpb.GetRecentTradesResponse, error) {
	limitPtr := toPtrInt(req.Limit)

	results, err := h.recentTradesSvc.Execute(ctx, req.Symbol, limitPtr)
	if err != nil {
		return nil, err
	}

	res := &marketpb.GetRecentTradesResponse{
		Trades: make([]*marketpb.Trade, len(results)),
	}

	for i, t := range results {
		res.Trades[i] = &marketpb.Trade{
			Id:           int64(t.Id),
			Price:        t.Price,
			Qty:          t.Qty,
			Time:         int64(t.Time),
			QuoteQty:     t.QuoteQty,
			IsBuyerMaker: t.IsBuyerMaker,
			IsBestMatch:  t.IsBest,
		}
	}

	return res, nil
}

// --- Historical Trades ---

// GetHistoricalTrades implements marketpb.MarketServiceServer
func (h *MarketGrpcHandler) GetHistoricalTrades(ctx context.Context, req *marketpb.GetHistoricalTradesRequest) (*marketpb.GetHistoricalTradesResponse, error) {
	limitPtr := toPtrUint(req.Limit)
	var fromIdPtr *int64
	if req.FromId != 0 {
		fromIdPtr = &req.FromId
	}

	results, err := h.historicalTradeLookupSvc.Execute(ctx, req.Symbol, limitPtr, fromIdPtr)
	if err != nil {
		return nil, err
	}

	res := &marketpb.GetHistoricalTradesResponse{
		Trades: make([]*marketpb.Trade, len(results)),
	}

	for i, t := range results {
		res.Trades[i] = &marketpb.Trade{
			Id:           int64(t.Id),
			Price:        t.Price,
			Qty:          t.Qty,
			Time:         int64(t.Time),
			QuoteQty:     t.QuoteQty,
			IsBuyerMaker: t.IsBuyerMaker,
			IsBestMatch:  t.IsBest,
		}
	}

	return res, nil
}

// --- Aggregate Trades ---

// GetAggTrades implements marketpb.MarketServiceServer
func (h *MarketGrpcHandler) GetAggTrades(ctx context.Context, req *marketpb.GetAggTradesRequest) (*marketpb.GetAggTradesResponse, error) {
	limitPtr := toPtrInt(req.Limit)

	var fromIdPtr *int
	if req.FromId != 0 {
		fromIdInt := int(req.FromId)
		fromIdPtr = &fromIdInt
	}
	var startTimePtr *uint64
	if req.StartTime != 0 {
		startTimePtr = &req.StartTime
	}
	var endTimePtr *uint64
	if req.EndTime != 0 {
		endTimePtr = &req.EndTime
	}

	results, err := h.aggTradesListSvc.Execute(ctx, req.Symbol, limitPtr, fromIdPtr, startTimePtr, endTimePtr)
	if err != nil {
		return nil, err
	}

	res := &marketpb.GetAggTradesResponse{
		Trades: make([]*marketpb.AggTrade, len(results)),
	}

	for i, t := range results {
		res.Trades[i] = &marketpb.AggTrade{
			AggTradeId:   int64(t.AggTradeId),
			Price:        t.Price,
			Qty:          t.Qty,
			FirstTradeId: int64(t.FirstTradeId),
			LastTradeId:  int64(t.LastTradeId),
			Time:         int64(t.Time),
			IsBuyerMaker: t.IsBuyer,
			IsBestMatch:  t.IsBest,
		}
	}

	return res, nil
}

// --- Klines / Candlestick ---

// GetKlines implements marketpb.MarketServiceServer
func (h *MarketGrpcHandler) GetKlines(
	ctx context.Context,
	req *marketpb.GetKlinesRequest,
) (*marketpb.GetKlinesResponse, error) {
	// if err := req.Validate(); err != nil {
	// 	return nil, errors.NewErrWrap(err, errors.ErrorTypeValidation).GrpcError()
	// }

	if req.GetLimit() == 0 {
		defaultVal := int32(500)
		req.Limit = &defaultVal // Set default limit if not provided
	}

	args := h.klinesSvc.
		Symbol(req.GetSymbol()).
		Interval(req.GetInterval()).
		Limit(req.GetLimit())

	if req.GetStartTime() != 0 {
		args = args.StartTime(req.GetStartTime())
	}
	if req.GetEndTime() != 0 {
		args = args.EndTime(req.GetEndTime())
	}

	results, err := args.Execute(ctx)
	if err != nil {
		return nil, err
	}

	res := &marketpb.GetKlinesResponse{
		Klines: make([]*marketpb.Kline, len(results)),
	}

	for i, k := range results {
		res.Klines[i] = &marketpb.Kline{
			OpenTime:                 k.OpenTime,
			Open:                     k.Open,
			High:                     k.High,
			Low:                      k.Low,
			Close:                    k.Close,
			Volume:                   k.Volume,
			CloseTime:                k.CloseTime,
			QuoteAssetVolume:         k.QuoteAssetVolume,
			NumberOfTrades:           k.NumberOfTrades,
			TakerBuyBaseAssetVolume:  k.TakerBuyBaseAssetVolume,
			TakerBuyQuoteAssetVolume: k.TakerBuyQuoteAssetVolume,
		}
	}

	return res, nil
}

// --- UI Klines ---

// GetUiKlines implements marketpb.MarketServiceServer
func (h *MarketGrpcHandler) GetUiKlines(ctx context.Context, req *marketpb.GetUiKlinesRequest) (*marketpb.GetUiKlinesResponse, error) {
	limitPtr := toPtrInt(req.Limit)
	var startTimePtr *uint64
	if req.StartTime != 0 {
		startTimePtr = &req.StartTime
	}
	var endTimePtr *uint64
	if req.EndTime != 0 {
		endTimePtr = &req.EndTime
	}

	results, err := h.uiKlinesSvc.Execute(ctx, req.Symbol, req.Interval, limitPtr, startTimePtr, endTimePtr)
	if err != nil {
		return nil, err
	}

	res := &marketpb.GetUiKlinesResponse{
		Klines: make([]*marketpb.UiKline, len(results)),
	}

	for i, k := range results {
		res.Klines[i] = &marketpb.UiKline{
			OpenTime:                 k.OpenTime,
			Open:                     k.Open,
			High:                     k.High,
			Low:                      k.Low,
			Close:                    k.Close,
			Volume:                   k.Volume,
			CloseTime:                k.CloseTime,
			QuoteAssetVolume:         k.QuoteAssetVolume,
			NumberOfTrades:           k.NumberOfTrades,
			TakerBuyBaseAssetVolume:  k.TakerBuyBaseAssetVolume,
			TakerBuyQuoteAssetVolume: k.TakerBuyQuoteAssetVolume,
		}
	}

	return res, nil
}

// --- Average Price ---

// GetAvgPrice implements marketpb.MarketServiceServer
func (h *MarketGrpcHandler) GetAvgPrice(ctx context.Context, req *marketpb.GetAvgPriceRequest) (*marketpb.GetAvgPriceResponse, error) {
	result, err := h.avgPriceSvc.Execute(ctx, req.Symbol)
	if err != nil {
		return nil, err
	}

	return &marketpb.GetAvgPriceResponse{
		Mins:  result.Mins,
		Price: result.Price,
	}, nil
}

// --- 24hr Ticker ---

// GetTicker24hr implements marketpb.MarketServiceServer
func (h *MarketGrpcHandler) GetTicker24hr(ctx context.Context, req *marketpb.GetTicker24HrRequest) (*marketpb.GetTicker24HrResponse, error) {
	var symbols *[]string
	if len(req.GetSymbols()) > 0 {
		symbols = &req.Symbols
	}

	results, err := h.ticker24hrSvc.Execute(
		ctx,
		req.Symbol, // *string (optional field)
		symbols,    // *[]string (repeated field)
	)
	if err != nil {
		return nil, err
	}

	res := &marketpb.GetTicker24HrResponse{
		Tickers: make([]*marketpb.Ticker24Hr, len(results)),
	}

	for i, t := range results {
		res.Tickers[i] = &marketpb.Ticker24Hr{
			Symbol:             t.Symbol,
			PriceChange:        t.PriceChange,
			PriceChangePercent: t.PriceChangePercent,
			WeightedAvgPrice:   t.WeightedAvgPrice,
			LastPrice:          t.LastPrice,
			Volume:             t.Volume,
			QuoteVolume:        t.QuoteVolume,
			OpenTime:           t.OpenTime,
			CloseTime:          t.CloseTime,
		}
	}

	return res, nil
}

// --- Symbol Price ---

// GetTickerPrice implements marketpb.MarketServiceServer
func (h *MarketGrpcHandler) GetTickerPrice(ctx context.Context, req *marketpb.GetTickerPriceRequest) (*marketpb.GetTickerPriceResponse, error) {
	var symbols *[]string
	if len(req.GetSymbols()) > 0 {
		symbols = &req.Symbols
	}

	results, err := h.tickerPriceSvc.Execute(
		ctx,
		req.Symbol, // *string (optional field)
		symbols,    // *[]string (repeated field)
	)
	if err != nil {
		return nil, err
	}

	res := &marketpb.GetTickerPriceResponse{
		Tickers: make([]*marketpb.TickerPrice, len(results)),
	}

	for i, t := range results {
		res.Tickers[i] = &marketpb.TickerPrice{
			Symbol: t.Symbol,
			Price:  t.Price,
		}
	}

	return res, nil
}

// --- Book Ticker ---

// GetBookTicker implements marketpb.MarketServiceServer
func (h *MarketGrpcHandler) GetBookTicker(ctx context.Context, req *marketpb.GetBookTickerRequest) (*marketpb.GetBookTickerResponse, error) {
	var symbols *[]string
	if len(req.GetSymbols()) > 0 {
		symbols = &req.Symbols
	}

	results, err := h.tickerBookTickerSvc.Execute(
		ctx,
		req.Symbol, // *string (optional field)
		symbols,    // *[]string (repeated field)
	)
	if err != nil {
		return nil, err
	}

	res := &marketpb.GetBookTickerResponse{
		Tickers: make([]*marketpb.BookTicker, len(results)),
	}

	for i, t := range results {
		res.Tickers[i] = &marketpb.BookTicker{
			Symbol:   t.Symbol,
			BidPrice: t.BidPrice,
			BidQty:   t.BidQty,
			AskPrice: t.AskPrice,
			AskQty:   t.AskQty,
		}
	}

	return res, nil
}

// --- Rolling Ticker ---

// GetRollingTicker implements marketpb.MarketServiceServer
func (h *MarketGrpcHandler) GetRollingTicker(ctx context.Context, req *marketpb.GetRollingTickerRequest) (*marketpb.GetRollingTickerResponse, error) {
	windowSizePtr := toPtrString(req.WindowSize)
	tickerTypePtr := toPtrString(req.Type)

	result, err := h.tickerSvc.Execute(
		ctx,
		req.Symbol,
		windowSizePtr,
		tickerTypePtr,
	)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, fmt.Errorf("rolling ticker returned no data for symbol %s", req.Symbol)
	}

	return &marketpb.GetRollingTickerResponse{
		Symbol:             result.Symbol,
		PriceChange:        result.PriceChange,
		PriceChangePercent: result.PriceChangePercent,
		WeightedAvgPrice:   result.WeightedAvgPrice,
		LastPrice:          result.LastPrice,
		Volume:             result.Volume,
		QuoteVolume:        result.QuoteVolume,
		OpenTime:           result.OpenTime,
		CloseTime:          result.CloseTime,
	}, nil
}
