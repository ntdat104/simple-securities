package dto

type KlineDto struct {
	OpenTime                 int64  `json:"open_time"`
	Open                     string `json:"open"`
	High                     string `json:"high"`
	Low                      string `json:"low"`
	Close                    string `json:"close"`
	Volume                   string `json:"volume"`
	CloseTime                int64  `json:"close_time"`
	QuoteAssetVolume         string `json:"quote_asset_volume"`
	NumberOfTrades           int32  `json:"number_of_trades"`
	TakerBuyBaseAssetVolume  string `json:"taker_buy_base_asset_volume"`
	TakerBuyQuoteAssetVolume string `json:"taker_buy_quote_asset_volume"`
}

type ServerTimeDto struct {
	ServerTime int64 `json:"server_time"`
}

type GetKlinesReq struct {
	Symbol   string `json:"symbol"`
	Interval string `json:"interval"`
	Limit    int32  `json:"limit"`
}
