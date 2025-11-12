package service

import (
	"context"
	"net/http"
	"simple-securities/pkg/client"

	"golang.org/x/sync/singleflight"
)

type PingSvc interface {
	Execute(ctx context.Context, opts ...client.RequestOption) error
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
	_, err, _ = s.sf.Do("ping", func() (any, error) {
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
