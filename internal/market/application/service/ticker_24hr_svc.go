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

// Ticker24hrSvc defines the interface for fetching 24hr ticker price change statistics.
type Ticker24hrSvc interface {
	Execute(ctx context.Context, symbol *string, symbols *[]string, opts ...client.RequestOption) ([]*response.Ticker24hrResponse, error)
}

type ticker24hrSvc struct {
	c  *client.Client
	sf singleflight.Group
}

func NewTicker24hrSvc(c *client.Client) Ticker24hrSvc {
	return &ticker24hrSvc{
		c: c,
	}
}

func (s *ticker24hrSvc) Execute(
	ctx context.Context,
	symbol *string,
	symbols *[]string,
	opts ...client.RequestOption,
) (res []*response.Ticker24hrResponse, err error) {

	// 1. Create a map to hold parameters for key generation
	keyMap := make(map[string]any)

	if symbol != nil {
		keyMap["Symbol"] = *symbol
	}
	if symbols != nil {
		// Include the pointer to the slice content for key generation
		keyMap["Symbols"] = *symbols
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
			Endpoint: "/api/v3/ticker/24hr",
			SecType:  client.SecTypeNone,
		}

		if symbol != nil {
			r.SetParam("symbol", *symbol)
		}

		if symbols != nil {
			// Binance requires the 'symbols' parameter value to be a JSON array string
			symbolsJson, mErr := json.Marshal(*symbols)
			if mErr != nil {
				return nil, fmt.Errorf("failed to marshal symbols parameter: %w", mErr)
			}
			r.SetParam("symbols", string(symbolsJson))
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

	// Handle dynamic response: single object (all tickers) or array (specific symbols)
	var raw json.RawMessage
	if err = json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw JSON message: %w", err)
	}

	// Check if the raw JSON starts with '[' (is an array)
	if len(raw) > 0 && raw[0] == '[' {
		res = make([]*response.Ticker24hrResponse, 0)
		if err = json.Unmarshal(data, &res); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into array response: %w", err)
		}
	} else {
		// The response is a single object (when no symbol/symbols specified or one symbol specified)
		singleRes := new(response.Ticker24hrResponse)
		if err = json.Unmarshal(data, &singleRes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into single object response: %w", err)
		}
		res = append(res, singleRes)
	}

	return res, nil
}
