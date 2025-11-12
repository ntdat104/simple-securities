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

// TickerSvc defines the interface for fetching rolling window price change statistics.
type TickerSvc interface {
	Execute(ctx context.Context, symbol string, windowSize *string, tickerType *string, opts ...client.RequestOption) (*response.TickerResponse, error)
}

type tickerSvc struct {
	c  *client.Client
	sf singleflight.Group
}

func NewTickerSvc(c *client.Client) TickerSvc {
	return &tickerSvc{
		c: c,
	}
}

func (s *tickerSvc) Execute(
	ctx context.Context,
	symbol string,
	windowSize *string,
	tickerType *string,
	opts ...client.RequestOption,
) (res *response.TickerResponse, err error) {

	// 1. Create a map to hold parameters for key generation
	keyMap := make(map[string]any)
	keyMap["Symbol"] = symbol // Symbol is mandatory

	if windowSize != nil {
		keyMap["WindowSize"] = *windowSize
	}
	if tickerType != nil {
		keyMap["Type"] = *tickerType
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
			Endpoint: "/api/v3/ticker",
			SecType:  client.SecTypeNone,
		}

		r.SetParam("symbol", symbol)

		if windowSize != nil {
			r.SetParam("windowSize", *windowSize)
		}
		if tickerType != nil {
			r.SetParam("type", *tickerType)
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

	res = new(response.TickerResponse)
	if err = json.Unmarshal(data, res); err != nil {
		return nil, err
	}
	return res, nil
}
