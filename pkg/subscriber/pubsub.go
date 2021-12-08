package subscriber

import (
	"bytes"
	"encoding/json"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/openfaas/connector-sdk/types"
	"go.uber.org/zap"
)

// PubSubSubscriber implements the openfaas ResponseSubscriber interface.
// The controller will invoke it's Response method on responses returned by the
// openfaas gateway.
type PubSubSubscriber struct {
	topic  *pubsub.Topic
	logger *zap.Logger
}

// NewPubSubSubscriber creates a new PubSubSubscriber using the passed topic  and zap logger.
func NewPubSubSubscriber(topic *pubsub.Topic, logger *zap.Logger) *PubSubSubscriber {
	return &PubSubSubscriber{
		topic:  topic,
		logger: logger,
	}
}

// Response publishes the InvokerResponse to the PubSubSubscribers topic as json.
func (s PubSubSubscriber) Response(res types.InvokerResponse) {
	m := map[string]interface{}{
		"data": *res.Body,
	}
	if res.Error != nil {
		m["err"] = res.Error
	}

	b := &bytes.Buffer{}
	if err := json.NewEncoder(b).Encode(m); err != nil {
		s.logger.Error("marshaling function response", zap.Error(err))
	}

	if err := s.topic.Publish(res.Context, b.Bytes()); err != nil {
		s.logger.Error("publishing function response", zap.Error(err), zap.String("topic", s.topic.String()))
	}
}
