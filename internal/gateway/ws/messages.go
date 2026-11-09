package ws

import (
	"encoding/json"

	"simple-securities/internal/gateway/api"
	"simple-securities/pkg/events"
)

// User data stream names.
const (
	StreamExecutionReport = "executionReport"
	StreamAccountPosition = "outboundAccountPosition"
)

type request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	ID     json.RawMessage `json:"id"`
}

type response struct {
	ID     json.RawMessage `json:"id"`
	Result any             `json:"result"`
}

type errorResponse struct {
	ID    json.RawMessage `json:"id"`
	Error api.ErrorBody   `json:"error"`
}

type push struct {
	Stream string `json:"stream"`
	Data   any    `json:"data"`
}

// tradeMsg is the public trade payload. It never carries user or order ids.
type tradeMsg struct {
	Event        string `json:"e"`
	Symbol       string `json:"s"`
	TradeID      int64  `json:"t"`
	Price        string `json:"p"`
	Qty          string `json:"q"`
	Time         int64  `json:"T"`
	IsBuyerMaker bool   `json:"m"`
}

type depthMsg struct {
	LastUpdateID int64               `json:"lastUpdateId"`
	Bids         []events.PriceLevel `json:"bids"`
	Asks         []events.PriceLevel `json:"asks"`
}

type tickerMsg struct {
	Event              string `json:"e"`
	Symbol             string `json:"s"`
	LastPrice          string `json:"c"`
	OpenPrice          string `json:"o"`
	HighPrice          string `json:"h"`
	LowPrice           string `json:"l"`
	Volume             string `json:"v"`
	QuoteVolume        string `json:"q"`
	PriceChange        string `json:"p"`
	PriceChangePercent string `json:"P"`
	Count              int64  `json:"n"`
	CloseTime          int64  `json:"C"`
}

type klineBody struct {
	OpenTime    int64  `json:"t"`
	CloseTime   int64  `json:"T"`
	Interval    string `json:"i"`
	Open        string `json:"o"`
	High        string `json:"h"`
	Low         string `json:"l"`
	Close       string `json:"c"`
	Volume      string `json:"v"`
	QuoteVolume string `json:"q"`
	Count       int64  `json:"n"`
}

type klineMsg struct {
	Event  string    `json:"e"`
	Symbol string    `json:"s"`
	Kline  klineBody `json:"k"`
}

type executionReport struct {
	Event string `json:"e"`
	api.Order
	LastTradeID int64 `json:"lastTradeId"`
}

type accountPosition struct {
	Event    string               `json:"e"`
	Balances []events.BalanceItem `json:"balances"`
}

func tradeFrom(t events.TradeExecuted) tradeMsg {
	return tradeMsg{Event: "trade", Symbol: t.Symbol, TradeID: t.TradeID, Price: t.Price, Qty: t.Qty,
		Time: t.Time.UnixMilli(), IsBuyerMaker: t.BuyerIsMaker}
}

func depthFrom(d events.DepthSnapshot, levels int) depthMsg {
	cut := func(ls []events.PriceLevel) []events.PriceLevel {
		if ls == nil {
			return []events.PriceLevel{}
		}
		if levels > 0 && len(ls) > levels {
			return ls[:levels]
		}
		return ls
	}
	return depthMsg{LastUpdateID: d.LastUpdateID, Bids: cut(d.Bids), Asks: cut(d.Asks)}
}

func tickerFrom(t events.Ticker) tickerMsg {
	return tickerMsg{Event: "24hrTicker", Symbol: t.Symbol, LastPrice: t.LastPrice, OpenPrice: t.OpenPrice,
		HighPrice: t.HighPrice, LowPrice: t.LowPrice, Volume: t.Volume, QuoteVolume: t.QuoteVolume,
		PriceChange: t.PriceChange, PriceChangePercent: t.PriceChangePercent, Count: t.TradeCount,
		CloseTime: t.CloseTime}
}

func klineFrom(k events.Kline) klineMsg {
	return klineMsg{Event: "kline", Symbol: k.Symbol, Kline: klineBody{OpenTime: k.OpenTime,
		CloseTime: k.CloseTime, Interval: k.Interval, Open: k.Open, High: k.High, Low: k.Low, Close: k.Close,
		Volume: k.Volume, QuoteVolume: k.QuoteVolume, Count: k.TradeCount}}
}

func executionReportFrom(e events.OrderUpdated) executionReport {
	return executionReport{Event: StreamExecutionReport, Order: api.OrderFromEvent(e), LastTradeID: e.LastTradeID}
}

func accountPositionFrom(b events.BalanceUpdated) accountPosition {
	bs := b.Balances
	if bs == nil {
		bs = []events.BalanceItem{}
	}
	return accountPosition{Event: StreamAccountPosition, Balances: bs}
}

// ---------------------------------------------------------------- futures

// Futures user data stream names.
const (
	StreamOrderTradeUpdate = "ORDER_TRADE_UPDATE"
	StreamAccountUpdate    = "ACCOUNT_UPDATE"
)

// markPriceMsg is the futures mark price payload.
type markPriceMsg struct {
	Event           string `json:"e"`
	Symbol          string `json:"s"`
	MarkPrice       string `json:"p"`
	IndexPrice      string `json:"i"`
	FundingRate     string `json:"r"`
	NextFundingTime int64  `json:"T"`
	Time            int64  `json:"E"`
}

type orderTradeUpdate struct {
	Event string `json:"e"`
	api.FuturesOrder
	LastTradeID int64 `json:"lastTradeId"`
}

type accountUpdatePosition struct {
	Symbol         string `json:"symbol"`
	PositionAmt    string `json:"positionAmt"`
	EntryPrice     string `json:"entryPrice"`
	MarginType     string `json:"marginType"` // cross | isolated (like Position)
	IsolatedMargin string `json:"isolatedMargin"`
	Leverage       int    `json:"leverage"`
}

type accountUpdate struct {
	Event            string                  `json:"e"`
	Time             int64                   `json:"E"`
	Reason           string                  `json:"reason"`
	WalletBalance    string                  `json:"walletBalance"`
	AvailableBalance string                  `json:"availableBalance"`
	Positions        []accountUpdatePosition `json:"positions"`
}

func markPriceFrom(m events.MarkPrice) markPriceMsg {
	return markPriceMsg{Event: "markPriceUpdate", Symbol: m.Symbol, MarkPrice: m.MarkPrice, IndexPrice: m.IndexPrice,
		FundingRate: m.FundingRate, NextFundingTime: m.NextFundingTime, Time: m.Time}
}

func orderTradeUpdateFrom(e events.OrderUpdated) orderTradeUpdate {
	return orderTradeUpdate{Event: StreamOrderTradeUpdate, FuturesOrder: api.FuturesOrderFromEvent(e),
		LastTradeID: e.LastTradeID}
}

func accountUpdateFrom(a events.FuturesAccountUpdate) accountUpdate {
	ps := make([]accountUpdatePosition, 0, len(a.Positions))
	for _, p := range a.Positions {
		ps = append(ps, accountUpdatePosition{Symbol: p.Symbol, PositionAmt: p.PositionAmt, EntryPrice: p.EntryPrice,
			MarginType: api.PublicMarginType(p.MarginType), IsolatedMargin: p.IsolatedMargin, Leverage: p.Leverage})
	}
	return accountUpdate{Event: StreamAccountUpdate, Time: a.Time.UnixMilli(), Reason: a.Reason,
		WalletBalance: a.WalletBalance, AvailableBalance: a.AvailableBalance, Positions: ps}
}
