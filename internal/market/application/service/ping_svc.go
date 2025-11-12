package service

import (
	"context"
	"net/http"
	"simple-securities/pkg/client"

	"golang.org/x/sync/singleflight"
)

type PingSvc interface {
	Execute(ctx context.Context, opts ...client.RequestOption) error
	ExecuteSf(ctx context.Context, opts ...client.RequestOption) error
}

type pingSvc struct {
	c  *client.Client
	sf singleflight.Group
}

func NewPingSvc(c *client.Client) PingSvc {
	return &pingSvc{
		c: c,
	}
}

func (s *pingSvc) Execute(ctx context.Context, opts ...client.RequestOption) (err error) {
	r := &client.Request{
		Method:   http.MethodGet,
		Endpoint: "/api/v3/ping",
		SecType:  client.SecTypeNone,
	}
	_, err = s.c.CallAPI(ctx, r, opts...)
	if err != nil {
		return err
	}
	return nil
}

func (s *pingSvc) ExecuteSf(ctx context.Context, opts ...client.RequestOption) (err error) {
	// All concurrent calls with the same key ("ping") will share one API call
	_, err, _ = s.sf.Do("ping", func() (interface{}, error) {
		r := &client.Request{
			Method:   http.MethodGet,
			Endpoint: "/api/v3/ping",
			SecType:  client.SecTypeNone,
		}
		_, callErr := s.c.CallAPI(ctx, r, opts...)
		return nil, callErr
	})
	return err
}
