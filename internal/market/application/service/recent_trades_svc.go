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

type RecentTradesSvc interface {
	Execute(ctx context.Context, symbol string, limit *int, opts ...client.RequestOption) ([]*response.RecentTradesListResponse, error)
}

type recentTradesSvc struct {
	c  *client.Client
	sf singleflight.Group
}

func NewRecentTradesSvc(c *client.Client) RecentTradesSvc {
	return &recentTradesSvc{
		c: c,
	}
}

func (s *recentTradesSvc) Execute(
	ctx context.Context,
	symbol string,
	limit *int,
	opts ...client.RequestOption,
) (res []*response.RecentTradesListResponse, err error) {

	// 1. Create a map to hold parameters for key generation
	keyMap := make(map[string]any)
	keyMap["Symbol"] = symbol // Symbol is mandatory

	if limit != nil {
		keyMap["Limit"] = *limit // Limit is optional
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
			Endpoint: "/api/v3/trades",
			SecType:  client.SecTypeNone,
		}

		r.SetParam("symbol", symbol)

		if limit != nil {
			r.SetParam("limit", *limit)
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

	// Note: The response is a slice of DTOs, so we unmarshal into a pointer to a slice.
	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}
