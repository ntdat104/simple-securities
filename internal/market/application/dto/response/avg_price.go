package response

type AvgPriceResponse struct {
	Mins  uint64 `json:"mins"`
	Price string `json:"price"`
}
