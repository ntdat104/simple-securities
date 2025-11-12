package client

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type secType int

const (
	SecTypeNone secType = iota
	SecTypeAPIKey
	SecTypeSigned // if the 'timestamp' parameter is required
)

type params map[string]any

// request define an API request
type Request struct {
	Method     string
	Endpoint   string
	Query      url.Values
	Form       url.Values
	RecvWindow int64
	SecType    secType
	Header     http.Header
	Body       io.Reader
	FullURL    string
}

// addParam add param with key/value to query string
func (r *Request) addParam(key string, value any) *Request {
	if r.Query == nil {
		r.Query = url.Values{}
	}
	r.Query.Add(key, fmt.Sprintf("%v", value))
	return r
}

// setParam set param with key/value to query string
func (r *Request) setParam(key string, value any) *Request {
	if r.Query == nil {
		r.Query = url.Values{}
	}
	r.Query.Set(key, fmt.Sprintf("%v", value))
	return r
}

// setParams set params with key/values to query string
func (r *Request) setParams(m params) *Request {
	for k, v := range m {
		r.setParam(k, v)
	}
	return r
}

func (r *Request) validate() (err error) {
	if r.Query == nil {
		r.Query = url.Values{}
	}
	if r.Form == nil {
		r.Form = url.Values{}
	}
	return nil
}

// Append `WithRecvWindow(insert_recvwindow)` to request to modify the default recvWindow value
func WithRecvWindow(recvWindow int64) RequestOption {
	return func(r *Request) {
		r.RecvWindow = recvWindow
	}
}

// RequestOption define option type for request
type RequestOption func(*Request)
