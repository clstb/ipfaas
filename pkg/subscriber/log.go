package subscriber

import (
	"github.com/openfaas/connector-sdk/types"
	"go.uber.org/zap"
)

type LogSubscriber struct {
	logger *zap.Logger
}

func NewLogSubscriber(logger *zap.Logger) *LogSubscriber {
	return &LogSubscriber{
		logger: logger,
	}
}

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
