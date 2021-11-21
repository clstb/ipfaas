package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/openfaas/connector-sdk/types"
	"github.com/openfaas/faas-provider/auth"
)

type Connector struct {
	controller   types.Controller
	topic        *pubsub.Topic
	subscription *pubsub.Subscription
}

func NewConnector(
	ps *pubsub.PubSub,
	topicName string,
	controllerCreds *auth.BasicAuthCredentials,
	controllerConfig *types.ControllerConfig,
	subscriber types.ResponseSubscriber,
) (*Connector, error) {
	topic, err := ps.Join(topicName)
	if err != nil {
		return nil, fmt.Errorf("joining topic %s: %w", topicName, err)
	}

	subscription, err := topic.Subscribe()
	if err != nil {
		return nil, fmt.Errorf("subscribing topic %s: %w", topicName, err)
	}

	controller := types.NewController(controllerCreds, controllerConfig)

	controller.Subscribe(subscriber)
	controller.BeginMapBuilder()

	return &Connector{
		topic:        topic,
		subscription: subscription,
		controller:   controller,
	}, nil
}

func (c *Connector) Run(
	ctx context.Context,
	peerID peer.ID,
) error {
	for {
		// receive next message
		msg, err := c.subscription.Next(ctx)
		if err != nil {
			return fmt.Errorf("receiving message: %w", err)
		}

		// the connector should not be sending message anyways
		// we discard own messages anyway just to be sure
		if msg.ReceivedFrom == peerID {
			continue
		}

		// invoke the function
		c.controller.InvokeWithContext(
			ctx,
			*msg.Topic,
			&msg.Data,
			http.Header{},
		)

		// check if context is alive
		select {
		case <-ctx.Done():
			return fmt.Errorf("context: %w", ctx.Err())
		default:
			continue
		}
	}
}
