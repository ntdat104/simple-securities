// Package web embeds the single-page trading UI served by the gateway.
package web

import _ "embed"

// Index is web/index.html.
//
//go:embed index.html
var Index []byte
