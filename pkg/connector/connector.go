package connector

import (
	"context"
	"fmt"
	"net/http"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/openfaas/connector-sdk/types"
	"github.com/openfaas/faas-provider/auth"
	"go.uber.org/zap"
)

// Connector is a communication channel between a pubsub topic
// and the openfaas gateway.
type Connector struct {
	controller   types.Controller
	topic        *pubsub.Topic
	subscription *pubsub.Subscription
	logger       *zap.Logger
}

// NewConnector creates a new connector wit the provided pubsub network.
// It will try to join the provided topic and setup the openfaas controller in
// order to communicate with the openfaas gateway. Responses returned by the
// gateway will be handled by the passed subscriber. After creation call it's
// Run to start handling messages.
func NewConnector(
	ps *pubsub.PubSub,
	topicName string,
	controllerCreds *auth.BasicAuthCredentials,
	controllerConfig *types.ControllerConfig,
	subscriber types.ResponseSubscriber,
	logger *zap.Logger,
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
		logger:       logger,
	}, nil
}

// Run start loop that passes messages this connector receives to the openfaas
// gateway via the connectors controller. Messages of passed peerID are dropped.
// It can be stopped using the passed context.
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
		c.logger.Info(
			"received message",
			zap.Stringp("topic", msg.Topic),
			zap.ByteString("data", msg.Data),
		)

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
