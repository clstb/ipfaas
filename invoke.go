package main

import (
	"net/http"

	"github.com/clstb/ipfs-connector/pkg/message"
	"github.com/openfaas/connector-sdk/types"
	"go.uber.org/zap"
)

type subscriber chan message.Message

func (s subscriber) Response(res types.InvokerResponse) {
	s <- message.Message{
		Type:       message.FunctionResponse,
		Topic:      res.Topic,
		SubMessage: res,
	}
}

func invoke(
	logger *zap.Logger,
	in <-chan message.Message,
	controller types.Controller,
) <-chan message.Message {
	subscriber := make(subscriber)
	controller.Subscribe(subscriber)

	go func() {
		for msg := range in {
			if msg.Type != message.FunctionInvoke {
				continue
			}

			submsg, ok := msg.SubMessage.(message.FunctionInvokeMessage)
			if !ok {
				logger.Warn("invalid sub message type")
				continue
			}
			b := submsg.Bytes()

			controller.Invoke(msg.Topic, &b, http.Header{})

			logger.Info(
				"invoked functions",
				zap.String("topic", msg.Topic),
			)
		}
		close(subscriber)
	}()

	return subscriber
}
