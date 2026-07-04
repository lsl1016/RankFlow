package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"go.uber.org/zap"
)

const TraceHeader = "X-Trace-Id"

type contextKey string

const (
	traceIDKey contextKey = "traceId"
	loggerKey  contextKey = "logger"
)

func NewTraceID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

func TraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(traceIDKey).(string)
	return v
}

func WithLogger(ctx context.Context, log *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, log)
}

func Logger(ctx context.Context, fallback *zap.Logger, fields ...zap.Field) *zap.Logger {
	log := fallback
	if ctx != nil {
		if v, ok := ctx.Value(loggerKey).(*zap.Logger); ok && v != nil {
			log = v
		} else if traceID := TraceID(ctx); traceID != "" && log != nil {
			log = log.With(zap.String("traceId", traceID))
		}
	}
	if log == nil {
		log = zap.NewNop()
	}
	if len(fields) > 0 {
		log = log.With(fields...)
	}
	return log
}
