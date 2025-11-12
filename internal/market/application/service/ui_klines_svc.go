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

type UiKlinesSvc interface {
	Execute(ctx context.Context, symbol string, interval string, limit *int, startTime *uint64, endTime *uint64, opts ...client.RequestOption) ([]*response.UiKlinesResponse, error)
}

type uiKlinesSvc struct {
	c  *client.Client
	sf singleflight.Group
}

func NewUiKlinesSvc(c *client.Client) UiKlinesSvc {
	return &uiKlinesSvc{
		c: c,
	}
}

func (s *uiKlinesSvc) Execute(
	ctx context.Context,
	symbol string,
	interval string,
	limit *int,
	startTime *uint64,
	endTime *uint64,
	opts ...client.RequestOption,
) (res []*response.UiKlinesResponse, err error) {

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
			Endpoint: "/api/v3/uiKlines",
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

	var uiklinesResponseArray response.UiKlinesResponseArray
	if err := json.Unmarshal(data, &uiklinesResponseArray); err != nil {
		return nil, fmt.Errorf("failed to unmarshal uiklines raw array: %w", err)
	}

	var uiklines []*response.UiKlinesResponse
	for _, uikline := range uiklinesResponseArray {
		if len(uikline) < 11 {
			return nil, fmt.Errorf("uikline response array has insufficient fields: expected 11, got %d", len(uikline))
		}

		// Parse the array values using type assertions
		openTime := uikline[0].(float64)
		open := uikline[1].(string)
		high := uikline[2].(string)
		low := uikline[3].(string)
		close := uikline[4].(string)
		volume := uikline[5].(string)
		closeTime := uikline[6].(float64)
		quoteAssetVolume := uikline[7].(string)
		numberOfTrades := uikline[8].(float64)
		takerBuyBaseAssetVolume := uikline[9].(string)
		takerBuyQuoteAssetVolume := uikline[10].(string)

		// Map to the final response DTO
		uiklinesResponse := &response.UiKlinesResponse{
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
		uiklines = append(uiklines, uiklinesResponse)
	}

	return uiklines, nil
}
