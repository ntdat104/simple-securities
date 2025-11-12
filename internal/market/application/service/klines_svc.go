package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"simple-securities/internal/market/application/dto/response"
	"simple-securities/pkg/client"
	"simple-securities/pkg/hashing"

	"golang.org/x/sync/singleflight"
)

type KlinesSvc interface {
	Execute(ctx context.Context, symbol string, interval string, limit *int, startTime *uint64, endTime *uint64, opts ...client.RequestOption) ([]*response.KlinesResponse, error)
}

type klinesSvc struct {
	c  *client.Client
	sf singleflight.Group
}

func NewKlinesSvc(c *client.Client) KlinesSvc {
	return &klinesSvc{
		c: c,
	}
}

func (s *klinesSvc) Execute(
	ctx context.Context,
	symbol string,
	interval string,
	limit *int,
	startTime *uint64,
	endTime *uint64,
	opts ...client.RequestOption,
) (res []*response.KlinesResponse, err error) {

	// 1. Create a map to hold parameters for key generation
	keyMap := make(map[string]any)
	keyMap["Symbol"] = symbol
	keyMap["Interval"] = interval

	if limit != nil {
		keyMap["Limit"] = *limit
	}
	if startTime != nil {
		keyMap["StartTime"] = *startTime
	}
	if endTime != nil {
		keyMap["EndTime"] = *endTime
	}

	// 2. Marshal the map to JSON bytes
	keyBytes, marshalErr := json.Marshal(keyMap)
	if marshalErr != nil {
		return nil, fmt.Errorf("failed to marshal request key: %w", marshalErr)
	}

	// 3. Hash the bytes to get the singleflight key
	key := hashing.Sha256Hash(keyBytes)

	val, callErr, _ := s.sf.Do(key, func() (any, error) {
		r := &client.Request{
			Method:   http.MethodGet,
			Endpoint: "/api/v3/klines",
			SecType:  client.SecTypeNone,
		}

		r.SetParam("symbol", symbol)
		r.SetParam("interval", interval)

		if limit != nil {
			r.SetParam("limit", *limit)
		}
		if startTime != nil {
			r.SetParam("startTime", *startTime)
		}
		if endTime != nil {
			r.SetParam("endTime", *endTime)
		}

		data, apiErr := s.c.CallAPI(ctx, r, opts...)
		if apiErr != nil {
			return nil, apiErr
		}
		return data, nil
	})

	if callErr != nil {
		return nil, callErr
	}

	data := val.([]byte)

	var klinesResponseArray response.KlinesResponseArray
	if err := json.Unmarshal(data, &klinesResponseArray); err != nil {
		return nil, fmt.Errorf("failed to unmarshal klines raw array: %w", err)
	}

	var klines []*response.KlinesResponse
	for _, kline := range klinesResponseArray {
		if len(kline) < 11 {
			return nil, fmt.Errorf("kline response array has insufficient fields: expected 11, got %d", len(kline))
		}

		// Parse the array values using type assertions
		openTime := kline[0].(float64)
		open := kline[1].(string)
		high := kline[2].(string)
		low := kline[3].(string)
		close := kline[4].(string)
		volume := kline[5].(string)
		closeTime := kline[6].(float64)
		quoteAssetVolume := kline[7].(string)
		numberOfTrades := kline[8].(float64)
		takerBuyBaseAssetVolume := kline[9].(string)
		takerBuyQuoteAssetVolume := kline[10].(string)

		// Map to the final response DTO
		klinesResponse := &response.KlinesResponse{
			OpenTime:                 uint64(openTime),
			Open:                     open,
			High:                     high,
			Low:                      low,
			Close:                    close,
			Volume:                   volume,
			CloseTime:                uint64(closeTime),
			QuoteAssetVolume:         quoteAssetVolume,
			NumberOfTrades:           uint64(numberOfTrades),
			TakerBuyBaseAssetVolume:  takerBuyBaseAssetVolume,
			TakerBuyQuoteAssetVolume: takerBuyQuoteAssetVolume,
		}
		klines = append(klines, klinesResponse)
	}

	return klines, nil
}
