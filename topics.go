package main

import (
	"context"
	"time"

	"github.com/clstb/ipfs-connector/pkg/message"
	ipfs "github.com/ipfs/go-ipfs-api"
	"github.com/openfaas/connector-sdk/types"
	"go.uber.org/zap"
)

func topicJoiner(
	logger *zap.Logger,
	in <-chan string,
	shell *ipfs.Shell,
) <-chan message.Message {
	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan message.Message)

	listen := func(
		ctx context.Context,
		topic string,
		s *ipfs.PubSubSubscription,
	) {
		defer s.Cancel()
		for {
			msg, err := s.Next()
			if err != nil {
				logger.Error(
					"receiving message",
					zap.Error(err),
				)
				// we return as the subscription is probably dead
				// TODO: resub mechanism
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
				out <- message.Message{
					Type:       message.IPFS,
					Topic:      topic,
					SubMessage: message.IPFSMessage(msg),
				}
				logger.Info(
					"received message",
					zap.String("topic", topic),
				)
			}
		}
	}

	go func() {
		for topic := range in {
			s, err := shell.PubSubSubscribe(topic)
			if err != nil {
				logger.Error(
					"subscribing topic",
					zap.String("topic", topic),
					zap.Error(err),
				)
				continue
			}
			go listen(ctx, topic, s)
			logger.Info(
				"joined topic",
				zap.String("topic", topic),
			)
		}
		cancel()
	}()

	return out
}

func topicWatcher(
	ctx context.Context,
	controller types.Controller,
) <-chan string {

	out := make(chan string)
	// we need to join the control topic, doing it here seems cleanest
	go func() {
		out <- "control"
	}()

	go func() {
		seen := map[string]struct{}{"control": {}}
		for {
			topics := controller.Topics()
			for _, topic := range topics {
				_, ok := seen[topic]
				if ok {
					continue
				}
				out <- topic
				seen[topic] = struct{}{}
			}
			select {
			case <-ctx.Done():
				close(out)
				return
			default:
				time.Sleep(time.Second)
			}
		}
	}()

	return out
}
