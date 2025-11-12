package response

type RecentTradesListResponse struct {
	Id           uint64 `json:"id"`
	Price        string `json:"price"`
	Qty          string `json:"qty"`
	Time         uint64 `json:"time"`
	QuoteQty     string `json:"quoteQty"`
	IsBuyerMaker bool   `json:"isBuyerMaker"`
	IsBest       bool   `json:"isBestMatch"`
}
