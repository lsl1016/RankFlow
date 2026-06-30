package observability

import "go.uber.org/zap"

// NewLogger returns a production-ish JSON logger.
func NewLogger() (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.DisableStacktrace = true
	return cfg.Build()
}
