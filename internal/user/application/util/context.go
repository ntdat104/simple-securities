package util

import (
	"context"
)

func GetValueFromCtx(ctx context.Context, key string) string {
	if v, ok := ctx.Value(key).(string); ok {
		return v
	}
	return ""
}
