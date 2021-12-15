package main

import (
	"encoding/json"

	"github.com/clstb/ipfs-connector/pkg/message"
	"go.uber.org/zap"
)

func parse(logger *zap.Logger, in <-chan message.Message) <-chan message.Message {
	out := make(chan message.Message)

	go func() {
		for msg := range in {
			if msg.Type != message.IPFS {
				continue
			}

			ipfsMsg, ok := msg.SubMessage.(message.IPFSMessage)
			if !ok {
				logger.Warn("invalid sub message type")
				continue
			}

			var submsg json.RawMessage
			parsed := message.Message{
				SubMessage: &submsg,
			}

			if err := json.Unmarshal(ipfsMsg.Data, &parsed); err != nil {
				logger.Error("parsing message", zap.Error(err))
				continue
			}

			switch parsed.Type {
			case message.FunctionDeploy:
				var v message.FunctionDeployMessage
				if err := json.Unmarshal(submsg, &v); err != nil {
					logger.Error("parsing sub message", zap.Error(err))
					continue
				}
				parsed.SubMessage = v
			case message.FunctionInvoke:
				var v message.FunctionInvokeMessage
				if err := json.Unmarshal(submsg, &v); err != nil {
					logger.Error("parsing sub message", zap.Error(err))
					continue
				}
				parsed.SubMessage = v
			default:
				continue
			}

			parsed.Topic = msg.Topic
			out <- parsed
		}
		close(out)
	}()

	return out
}
