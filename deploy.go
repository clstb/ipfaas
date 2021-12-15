package main

import (
	"github.com/clstb/ipfs-connector/pkg/message"
	"github.com/clstb/ipfs-connector/pkg/openfaas"
	"go.uber.org/zap"
)

func deploy(
	logger *zap.Logger,
	in <-chan message.Message,
	client *openfaas.Client,
) {
	go func() {
		for msg := range in {
			if msg.Type != message.FunctionDeploy {
				continue
			}

			submsg, ok := msg.SubMessage.(message.FunctionDeployMessage)
			if !ok {
				logger.Warn("invalid sub message type")
				continue
			}

			if err := client.Deploy(openfaas.Function(submsg)); err != nil {
				logger.Error("deploying function", zap.Error(err))
				continue
			}

			logger.Info("deployed function", zap.String("service", submsg.Service))
		}
	}()
}
