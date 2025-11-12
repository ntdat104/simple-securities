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

// AggTradesListSvc defines the interface for fetching compressed/aggregate trades.
type AggTradesListSvc interface {
	Execute(ctx context.Context, symbol string, limit *int, fromId *int, startTime *uint64, endTime *uint64, opts ...client.RequestOption) ([]*response.AggTradesListResponse, error)
}

type aggTradesListSvc struct {
	c  *client.Client
	sf singleflight.Group
}

func NewAggTradesListSvc(c *client.Client) AggTradesListSvc {
	return &aggTradesListSvc{
		c: c,
	}
}

func (s *aggTradesListSvc) Execute(
	ctx context.Context,
	symbol string,
	limit *int,
	fromId *int,
	startTime *uint64,
	endTime *uint64,
	opts ...client.RequestOption,
) (res []*response.AggTradesListResponse, err error) {

	// 1. Create a map to hold parameters for key generation
	keyMap := make(map[string]any)
	keyMap["Symbol"] = symbol

	if limit != nil {
		keyMap["Limit"] = *limit
	}
	if fromId != nil {
		keyMap["FromId"] = *fromId
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
			Endpoint: "/api/v3/aggTrades",
			SecType:  client.SecTypeNone,
		}

		r.SetParam("symbol", symbol)

		if limit != nil {
			r.SetParam("limit", *limit)
		}
		if fromId != nil {
			r.SetParam("fromId", *fromId)
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

	// Note: The response is a slice of DTOs.
	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}
