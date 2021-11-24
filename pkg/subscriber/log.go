package subscriber

import (
	"github.com/openfaas/connector-sdk/types"
	"go.uber.org/zap"
)

// LogSubscriber implements the openfaas ResponseSubscriber interface.
// The controller will invoke it's Response method on responses returned by the
// openfaas gateway.
type LogSubscriber struct {
	logger *zap.Logger
}

// NewLogSubscriber creates a new LogSubscriber using the passed zap logger.
func NewLogSubscriber(logger *zap.Logger) *LogSubscriber {
	return &LogSubscriber{
		logger: logger,
	}
}

// Response logs an error or info entry depending on the passed InvokerResponse.
func (s LogSubscriber) Response(res types.InvokerResponse) {
	fields := []zap.Field{
		zap.ByteString("body", *res.Body),
		zap.String("function", res.Function),
		zap.String("topic", res.Topic),
		zap.Int("status", res.Status),
	}
	if res.Error != nil {
		s.logger.Error(
			"function error",
			append(fields, zap.Error(res.Error))...,
		)
	} else {
		s.logger.Info("function response", fields...)
	}
}
