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

type ExchangeInfoSvc interface {
	Execute(ctx context.Context, symbol *string, symbols *[]string, permissions *string, showPermissionSets *bool, symbolStatus *string, opts ...client.RequestOption) (*response.ExchangeInfoResponse, error)
}

type exchangeInfoSvc struct {
	c  *client.Client
	sf singleflight.Group
}

func NewExchangeInfoSvc(c *client.Client) ExchangeInfoSvc {
	return &exchangeInfoSvc{
		c: c,
	}
}

func (s *exchangeInfoSvc) Execute(
	ctx context.Context,
	symbol *string,
	symbols *[]string,
	permissions *string,
	showPermissionSets *bool,
	symbolStatus *string,
	opts ...client.RequestOption,
) (res *response.ExchangeInfoResponse, err error) {

	// 1. Create a map to hold parameters for key generation
	keyMap := make(map[string]any)

	if symbol != nil {
		keyMap["Symbol"] = *symbol
	}
	if symbols != nil {
		keyMap["Symbols"] = *symbols
	}
	if permissions != nil {
		keyMap["Permissions"] = *permissions
	}
	if showPermissionSets != nil {
		keyMap["ShowPermissionSets"] = *showPermissionSets
	}
	if symbolStatus != nil {
		keyMap["SymbolStatus"] = *symbolStatus
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
			Endpoint: "/api/v3/exchangeInfo",
			SecType:  client.SecTypeNone,
		}

		if symbol != nil {
			r.SetParam("symbol", *symbol)
		}
		if symbols != nil {
			r.SetParam("symbols", *symbols)
		}
		if permissions != nil {
			r.SetParam("permissions", *permissions)
		}
		if showPermissionSets != nil {
			r.SetParam("showPermissionSets", *showPermissionSets)
		}
		if symbolStatus != nil {
			r.SetParam("symbolStatus", *symbolStatus)
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

	res = new(response.ExchangeInfoResponse)
	err = json.Unmarshal(data, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}
