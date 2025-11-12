package response

import "math/big"

type OrderBookResponse struct {
	LastUpdateId uint64         `json:"lastUpdateId"`
	Bids         [][]*big.Float `json:"bids"`
	Asks         [][]*big.Float `json:"asks"`
}
