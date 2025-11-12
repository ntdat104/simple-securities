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

// AvgPriceSvc defines the interface for fetching the current average price.
type AvgPriceSvc interface {
	Execute(ctx context.Context, symbol string, opts ...client.RequestOption) (*response.AvgPriceResponse, error)
}

type avgPriceSvc struct {
	c  *client.Client
	sf singleflight.Group
}

func NewAvgPriceSvc(c *client.Client) AvgPriceSvc {
	return &avgPriceSvc{
		c: c,
	}
}

func (s *avgPriceSvc) Execute(
	ctx context.Context,
	symbol string,
	opts ...client.RequestOption,
) (res *response.AvgPriceResponse, err error) {

	// 1. Create a map to hold parameters for key generation
	keyMap := map[string]any{
		"Symbol": symbol, // Symbol is mandatory
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
			Endpoint: "/api/v3/avgPrice",
			SecType:  client.SecTypeNone,
		}

		r.SetParam("symbol", symbol)

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

	res = new(response.AvgPriceResponse)
	err = json.Unmarshal(data, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}
