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

type KlinesSvc struct {
	c  *client.Client
	sf singleflight.Group

	symbol    string
	interval  string
	limit     *int32
	startTime *uint64
	endTime   *uint64
}

func NewKlinesSvc(c *client.Client) *KlinesSvc {
	return &KlinesSvc{
		c: c,
	}
}

func (s *KlinesSvc) Symbol(symbol string) *KlinesSvc {
	s.symbol = symbol
	return s
}

func (s *KlinesSvc) Interval(interval string) *KlinesSvc {
	s.interval = interval
	return s
}

func (s *KlinesSvc) Limit(limit int32) *KlinesSvc {
	s.limit = &limit
	return s
}

func (s *KlinesSvc) StartTime(startTime uint64) *KlinesSvc {
	s.startTime = &startTime
	return s
}

func (s *KlinesSvc) EndTime(endTime uint64) *KlinesSvc {
	s.endTime = &endTime
	return s
}

func (s *KlinesSvc) Execute(ctx context.Context, opts ...client.RequestOption) (res []*response.KlinesResponse, err error) {
	// 1. Create a map to hold parameters for key generation
	keyMap := make(map[string]any)
	keyMap["Symbol"] = s.symbol
	keyMap["Interval"] = s.interval

	if s.limit != nil {
		keyMap["Limit"] = *s.limit
	}
	if s.startTime != nil {
		keyMap["StartTime"] = *s.startTime
	}
	if s.endTime != nil {
		keyMap["EndTime"] = *s.endTime
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

		r.SetParam("symbol", s.symbol)
		r.SetParam("interval", s.interval)

		if s.limit != nil {
			r.SetParam("limit", *s.limit)
		}
		if s.startTime != nil {
			r.SetParam("startTime", *s.startTime)
		}
		if s.endTime != nil {
			r.SetParam("endTime", *s.endTime)
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
