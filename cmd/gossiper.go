package cmd

import (
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/urfave/cli/v2"
)

func Gossiper(ctx *cli.Context) error {
	// create a new libp2p Host that listens on a random TCP port
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"))
	if err != nil {
		return err
	}

	// create a new PubSub service using the GossipSub router
	ps, err := pubsub.NewGossipSub(ctx.Context, h)
	if err != nil {
		return err
	}

	// setup local mDNS discovery
	if err := setupDiscovery(h); err != nil {
		return err
	}

	topic, err := ps.Join(ctx.String("topic"))
	if err != nil {
		return err
	}

	for {
		if err := topic.Publish(
			ctx.Context,
			[]byte(fmt.Sprintf("hello from peer %s", h.ID())),
		); err != nil {
			return err
		}
		time.Sleep(5 * time.Second)
	}
}
