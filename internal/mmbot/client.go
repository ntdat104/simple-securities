package mmbot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// APIError is a non-2xx gateway response ({code,msg}).
type APIError struct {
	Status int
	Code   string
	Msg    string
}

func (e *APIError) Error() string { return fmt.Sprintf("http %d %s: %s", e.Status, e.Code, e.Msg) }

func statusOf(err error) int {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae.Status
	}
	return 0
}

// flexString accepts a JSON string or number (ids may be encoded either way).
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	*f = flexString(strings.Trim(string(b), `"`))
	return nil
}

// flexInt accepts a JSON number or a quoted number (protojson encodes int64 as strings).
type flexInt int64

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	*f = flexInt(v)
	return err
}

// Client is a small JSON client for the public gateway API.
type Client struct {
	root string // gateway root URL
	base string // root + /api/v1 (spot API)
	hc   *http.Client
}

// NewClient returns a client with bounded timeouts.
func NewClient(base string) *Client {
	root := strings.TrimRight(base, "/")
	return &Client{root: root, base: root + "/api/v1", hc: &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{MaxIdleConnsPerHost: 64, IdleConnTimeout: 90 * time.Second},
	}}
}

// Do performs a request; body and out are JSON (either may be nil). Paths are relative to
// /api/v1 (spot) unless they start with /fapi/ (futures API, relative to the gateway root).
func (c *Client) Do(ctx context.Context, method, path string, q url.Values, token string, body, out any) error {
	u := c.base + path
	if strings.HasPrefix(path, futuresPrefix) {
		u = c.root + path
	}
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		ae := &APIError{Status: resp.StatusCode}
		if json.Unmarshal(raw, ae) != nil || ae.Code == "" {
			ae.Msg = strings.TrimSpace(string(raw))
		}
		return ae
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

type authResp struct {
	AccessToken  string  `json:"accessToken"`
	RefreshToken string  `json:"refreshToken"`
	ExpiresIn    flexInt `json:"expiresIn"`
}

// Session is an authenticated bot user. Order mutations (spot AND futures) are paced by one shared
// limiter so the bot stays below the gateway's per-user order rate limit.
type Session struct {
	c        *Client
	email    string
	password string
	log      *zap.Logger
	limiter  *pacer

	mu      sync.Mutex
	access  string
	refresh string
	expires time.Time
}

// NewSession returns an unauthenticated session; call Login first.
func NewSession(c *Client, email, password string, ordersPerSec float64, log *zap.Logger) *Session {
	return &Session{c: c, email: email, password: password, log: log.With(zap.String("user", email)),
		limiter: newPacer(ordersPerSec)}
}

// Login registers the user or, if the email is taken, logs in.
func (s *Session) Login(ctx context.Context) error {
	creds := map[string]string{"email": s.email, "password": s.password}
	var ar authResp
	err := s.c.Do(ctx, http.MethodPost, "/auth/register", nil, "", creds, &ar)
	if statusOf(err) == http.StatusConflict {
		err = s.c.Do(ctx, http.MethodPost, "/auth/login", nil, "", creds, &ar)
	}
	if err != nil {
		return fmt.Errorf("login %s: %w", s.email, err)
	}
	s.store(ar)
	return nil
}

func (s *Session) store(ar authResp) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.access, s.refresh = ar.AccessToken, ar.RefreshToken
	ttl := time.Duration(ar.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	s.expires = time.Now().Add(ttl)
}

// token returns a valid access token, refreshing (or re-logging in) shortly before expiry.
func (s *Session) token(ctx context.Context) (string, error) {
	s.mu.Lock()
	access, refresh, exp := s.access, s.refresh, s.expires
	s.mu.Unlock()
	if access != "" && time.Until(exp) > 30*time.Second {
		return access, nil
	}
	if refresh != "" {
		var ar authResp
		err := s.c.Do(ctx, http.MethodPost, "/auth/refresh", nil, "", map[string]string{"refreshToken": refresh}, &ar)
		if err == nil && ar.AccessToken != "" {
			s.store(ar)
			return ar.AccessToken, nil
		}
		s.log.Info("token refresh failed, logging in again", zap.Error(err))
	}
	if err := s.Login(ctx); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.access, nil
}

// Do performs an authenticated request, re-logging in once on 401.
func (s *Session) Do(ctx context.Context, method, path string, q url.Values, body, out any) error {
	if method != http.MethodGet && (path == "/order" || path == futuresPrefix+"/order") {
		if err := s.limiter.wait(ctx); err != nil {
			return err
		}
	}
	tok, err := s.token(ctx)
	if err != nil {
		return err
	}
	err = s.c.Do(ctx, method, path, q, tok, body, out)
	if statusOf(err) == http.StatusUnauthorized {
		if lerr := s.Login(ctx); lerr != nil {
			return lerr
		}
		tok, _ = s.token(ctx)
		err = s.c.Do(ctx, method, path, q, tok, body, out)
	}
	return err
}

// pacer spaces calls evenly (no burst) at the configured rate.
type pacer struct {
	mu    sync.Mutex
	every time.Duration
	next  time.Time
}

func newPacer(perSec float64) *pacer {
	if perSec <= 0 {
		perSec = 4
	}
	return &pacer{every: time.Duration(float64(time.Second) / perSec)}
}

func (p *pacer) wait(ctx context.Context) error {
	p.mu.Lock()
	now := time.Now()
	if p.next.Before(now) {
		p.next = now
	}
	d := p.next.Sub(now)
	p.next = p.next.Add(p.every)
	p.mu.Unlock()
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
