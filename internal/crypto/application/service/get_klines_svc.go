package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"simple-securities/internal/crypto/application/dto"
)

type GetKlinesSvc interface {
	Handle(ctx context.Context, req *dto.GetKlinesReq) ([]*dto.KlineDto, error)
}

type getKlinesSvc struct {
	BaseURL string
}

func NewGetKlinesSvc() GetKlinesSvc {
	return &getKlinesSvc{
		BaseURL: "https://api.binance.com/api/v3/klines",
	}
}

func (s *getKlinesSvc) Handle(ctx context.Context, req *dto.GetKlinesReq) ([]*dto.KlineDto, error) {
	// Build URL with params
	url := fmt.Sprintf("%s?symbol=%s&interval=%s&limit=%d",
		s.BaseURL, req.Symbol, req.Interval, req.Limit)

	// Make HTTP request
	reqHttp, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(reqHttp)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("binance api error: %s", string(body))
	}

	// Binance returns array of arrays
	var raw [][]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	klines := make([]*dto.KlineDto, 0, len(raw))
	for _, k := range raw {
		if len(k) < 11 {
			continue
		}
		dtoItem := &dto.KlineDto{
			OpenTime:                 int64(k[0].(float64)),
			Open:                     k[1].(string),
			High:                     k[2].(string),
			Low:                      k[3].(string),
			Close:                    k[4].(string),
			Volume:                   k[5].(string),
			CloseTime:                int64(k[6].(float64)),
			QuoteAssetVolume:         k[7].(string),
			NumberOfTrades:           int32(k[8].(float64)),
			TakerBuyBaseAssetVolume:  k[9].(string),
			TakerBuyQuoteAssetVolume: k[10].(string),
		}
		klines = append(klines, dtoItem)
	}

	return klines, nil
}
