// Package config reads 12-factor configuration from environment variables.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// String returns the env value or def when unset/empty.
func String(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// Int returns the env value parsed as int or def.
func Int(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

// Bool returns the env value parsed as bool or def.
func Bool(key string, def bool) bool {
	if v, err := strconv.ParseBool(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

// Duration returns the env value parsed as time.Duration or def.
func Duration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

// List returns a comma separated env value as a slice or def.
func List(key string, def []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	out := make([]string, 0)
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Common holds settings shared by every service.
type Common struct {
	Service     string
	Env         string
	LogLevel    string
	GRPCAddr    string
	HTTPAddr    string
	DatabaseURL string
	Brokers     []string
	Instance    string
}

// LoadCommon loads the shared settings; defaults suit local development.
func LoadCommon(service, defGRPC, defHTTP, defDB string) Common {
	host, _ := os.Hostname()
	return Common{
		Service:     service,
		Env:         String("APP_ENV", "dev"),
		LogLevel:    String("LOG_LEVEL", "info"),
		GRPCAddr:    String("GRPC_ADDR", defGRPC),
		HTTPAddr:    String("HTTP_ADDR", defHTTP),
		DatabaseURL: String("DATABASE_URL", defDB),
		Brokers:     List("KAFKA_BROKERS", []string{"localhost:9094"}),
		Instance:    String("INSTANCE_ID", host),
	}
}
