package response

// ExchangeInfoResponse define exchange info response
type ExchangeInfoResponse struct {
	Timezone        string            `json:"timezone"`
	ServerTime      uint64            `json:"serverTime"`
	RateLimits      []*RateLimit      `json:"rateLimits"`
	ExchangeFilters []*ExchangeFilter `json:"exchangeFilters"`
	Symbols         []*SymbolInfo     `json:"symbols"`
}

// RateLimit define rate limit
type RateLimit struct {
	RateLimitType string `json:"rateLimitType"`
	Interval      string `json:"interval"`
	Limit         int    `json:"limit"`
}

// ExchangeFilter define exchange filter
type ExchangeFilter struct {
	FilterType string `json:"filterType"`
	MaxNumAlgo int64  `json:"maxNumAlgoOrders"`
}

// Symbol define symbol
type SymbolInfo struct {
	Symbol                          string          `json:"symbol"`
	Status                          string          `json:"status"`
	BaseAsset                       string          `json:"baseAsset"`
	BaseAssetPrecision              int64           `json:"baseAssetPrecision"`
	QuoteAsset                      string          `json:"quoteAsset"`
	QuotePrecision                  int64           `json:"quotePrecision"`
	QuoteAssetPrecision             int64           `json:"quoteAssetPrecision"`
	BaseCommissionPrecision         int64           `json:"baseCommissionPrecision"`
	QuoteCommissionPrecision        int64           `json:"quoteCommissionPrecision"`
	OrderTypes                      []string        `json:"orderTypes"`
	IcebergAllowed                  bool            `json:"icebergAllowed"`
	OcoAllowed                      bool            `json:"ocoAllowed"`
	OtoAllowed                      bool            `json:"otoAllowed"`
	QuoteOrderQtyMarketAllowed      bool            `json:"quoteOrderQtyMarketAllowed"`
	AllowTrailingStop               bool            `json:"allowTrailingStop"`
	CancelReplaceAllowed            bool            `json:"cancelReplaceAllowed"`
	IsSpotTradingAllowed            bool            `json:"isSpotTradingAllowed"`
	IsMarginTradingAllowed          bool            `json:"isMarginTradingAllowed"`
	Filters                         []*SymbolFilter `json:"filters"`
	Permissions                     []string        `json:"permissions"`
	PermissionSets                  [][]string      `json:"permissionSets"`
	DefaultSelfTradePreventionMode  string          `json:"defaultSelfTradePreventionMode"`
	AllowedSelfTradePreventionModes []string        `json:"allowedSelfTradePreventionModes"`
}

// SymbolFilter define symbol filter
type SymbolFilter struct {
	ApplyMinToMarket      bool   `json:"applyMinToMarket"`
	ApplyMaxToMarket      bool   `json:"applyMaxToMarket"`
	AskMultiplierDown     string `json:"askMultiplierDown"`
	AskMultiplierUp       string `json:"askMultiplierUp"`
	AvgPriceMins          int64  `json:"avgPriceMins"`
	BidMultiplierDown     string `json:"bidMultiplierDown"`
	BidMultiplierUp       string `json:"bidMultiplierUp"`
	FilterType            string `json:"filterType"`
	Limit                 uint   `json:"limit"`
	MaxNotional           string `json:"maxNotional"`
	MaxNumAlgoOrders      int64  `json:"maxNumAlgoOrders"`
	MaxNumOrders          int64  `json:"maxNumOrders"`
	MaxPrice              string `json:"maxPrice"`
	MaxQty                string `json:"maxQty"`
	MaxTrailingAboveDelta int64  `json:"maxTrailingAboveDelta"`
	MaxTrailingBelowDelta int64  `json:"maxTrailingBelowDelta"`
	MinNotional           string `json:"minNotional"`
	MinPrice              string `json:"minPrice"`
	MinQty                string `json:"minQty"`
	MinTrailingAboveDelta int64  `json:"minTrailingAboveDelta"`
	MinTrailingBelowDelta int64  `json:"minTrailingBelowDelta"`
	StepSize              string `json:"stepSize"`
	TickSize              string `json:"tickSize"`
}
