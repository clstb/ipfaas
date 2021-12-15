package main

import (
	"encoding/json"

	"github.com/clstb/ipfs-connector/pkg/message"
	ipfs "github.com/ipfs/go-ipfs-api"
	"go.uber.org/zap"
)

func publishPS(
	logger *zap.Logger,
	in <-chan message.Message,
	shell *ipfs.Shell,
) {
	go func() {
		for msg := range in {
			if msg.Type != message.FunctionResponse {
				continue
			}

			b, err := json.Marshal(msg)
			if err != nil {
				logger.Error(
					"marshalling message",
					zap.Error(err),
				)
				continue
			}

			if err := shell.PubSubPublish(msg.Topic, string(b)); err != nil {
				logger.Error(
					"publishing response",
					zap.Error(err),
				)
				continue
			}

			logger.Info("published response", zap.String("topic", msg.Topic))
		}
	}()
}
