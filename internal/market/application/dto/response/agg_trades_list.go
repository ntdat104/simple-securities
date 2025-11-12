package response

type AggTradesListResponse struct {
	AggTradeId   uint64 `json:"a"`
	Price        string `json:"p"`
	Qty          string `json:"q"`
	FirstTradeId uint64 `json:"f"`
	LastTradeId  uint64 `json:"l"`
	Time         uint64 `json:"T"`
	IsBuyer      bool   `json:"m"`
	IsBest       bool   `json:"M"`
}
