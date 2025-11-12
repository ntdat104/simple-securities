package service

import (
	"context"
	"encoding/json"
	"net/http"
	"simple-securities/internal/market/application/dto/response"
	"simple-securities/pkg/client"

	"golang.org/x/sync/singleflight"
)

type ServerTimeSvc interface {
	Execute(ctx context.Context, opts ...client.RequestOption) (*response.ServerTimeResponse, error)
}

type serverTimeSvc struct {
	c  *client.Client
	sf singleflight.Group
}

func NewServerTimeSvc(c *client.Client) ServerTimeSvc {
	return &serverTimeSvc{
		c: c,
	}
}

func (s *serverTimeSvc) Execute(ctx context.Context, opts ...client.RequestOption) (res *response.ServerTimeResponse, err error) {
	val, err, _ := s.sf.Do("serverTime", func() (any, error) {
		r := &client.Request{
			Method:   http.MethodGet,
			Endpoint: "/api/v3/time",
			SecType:  client.SecTypeNone,
		}
		data, callErr := s.c.CallAPI(ctx, r, opts...)
		if callErr != nil {
			return nil, callErr
		}
		return data, nil
	})

	if err != nil {
		return nil, err
	}

	data := val.([]byte)

	res = new(response.ServerTimeResponse)
	err = json.Unmarshal(data, res)
	if err != nil {
		return nil, err
	}

	return res, nil
}
