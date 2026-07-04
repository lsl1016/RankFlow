package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"rankflow/internal/dto"
	"rankflow/internal/observability"
)

// CORS allows the Vite dev server (and any origin) to call the API. Fine for an
// MVP admin tool with no auth; tighten for production.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,"+observability.TraceHeader+",X-User-Id")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// RequestContext attaches a traceId and a request-scoped logger to the context.
func RequestContext(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := strings.TrimSpace(c.GetHeader(observability.TraceHeader))
		if traceID == "" {
			traceID = observability.NewTraceID()
		}
		c.Header(observability.TraceHeader, traceID)
		c.Set("traceId", traceID)

		ctx := observability.WithTraceID(c.Request.Context(), traceID)
		reqLogger := log.With(zap.String("traceId", traceID))
		if userID := strings.TrimSpace(c.GetHeader("X-User-Id")); userID != "" {
			reqLogger = reqLogger.With(zap.String("userId", userID))
		}
		ctx = observability.WithLogger(ctx, reqLogger)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// AccessLog emits a structured access log line per request.
func AccessLog(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		status := c.Writer.Status()
		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("route", c.FullPath()),
			zap.Int("status", status),
			zap.Int64("durationMs", time.Since(start).Milliseconds()),
			zap.Int("responseBytes", c.Writer.Size()),
			zap.String("clientIp", c.ClientIP()),
			zap.String("userAgent", c.Request.UserAgent()),
		}
		fields = append(fields, businessFields(c)...)
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("ginErrors", c.Errors.String()))
		}

		logger := observability.Logger(c.Request.Context(), log)
		switch {
		case status >= http.StatusInternalServerError:
			logger.Error("access", fields...)
		case status >= http.StatusBadRequest:
			logger.Warn("access", fields...)
		default:
			logger.Info("access", fields...)
		}
	}
}

// Recovery logs panics with traceId and returns the unified API failure shape.
func Recovery(log *zap.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		fields := []zap.Field{
			zap.Any("panic", recovered),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("route", c.FullPath()),
		}
		fields = append(fields, businessFields(c)...)
		observability.Logger(c.Request.Context(), log).Error("panic recovered", fields...)
		c.AbortWithStatusJSON(http.StatusInternalServerError, dto.Fail(dto.CodeInternal, "internal server error"))
	})
}

func businessFields(c *gin.Context) []zap.Field {
	fields := make([]zap.Field, 0, 2)
	if rankID := c.Param("rankId"); rankID != "" {
		fields = append(fields, zap.String("rankId", rankID))
	}
	if itemID := c.Param("itemId"); itemID != "" {
		fields = append(fields, zap.String("itemId", itemID))
	}
	return fields
}
