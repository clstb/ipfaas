package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/clstb/ipfs-connector/pkg/connector"
	"github.com/clstb/ipfs-connector/pkg/subscriber"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/openfaas/connector-sdk/types"
	"github.com/openfaas/faas-provider/auth"
	"github.com/urfave/cli/v2"
)

// discoveryNotifee gets notified when we find a new peer via mDNS discovery
type discoveryNotifee struct {
	h host.Host
}

// HandlePeerFound connects to peers discovered via mDNS. Once they're connected,
// the PubSub system will automatically start interacting with them if they also
// support PubSub.
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	fmt.Printf("discovered new peer %s\n", pi.ID.Pretty())
	err := n.h.Connect(context.Background(), pi)
	if err != nil {
		fmt.Printf("error connecting to peer %s: %s\n", pi.ID.Pretty(), err)
	}
}

const DiscoveryServiceTag = "openfaas-ipfs-example"

// setupDiscovery creates an mDNS discovery service and attaches it to the libp2p Host.
// This lets us automatically discover peers on the same LAN and connect to them.
func setupDiscovery(h host.Host) error {
	// setup mDNS discovery to find local peers
	s := mdns.NewMdnsService(h, DiscoveryServiceTag, &discoveryNotifee{h: h})
	return s.Start()
}

func Run(ctx *cli.Context) error {
	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}
	defer logger.Sync()

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

	gatewayURL := ctx.String("gateway-url")

	creds := &auth.BasicAuthCredentials{
		User:     ctx.String("gateway-user"),
		Password: ctx.String("gateway-password"),
	}

	var connectors []*connector.Connector
	logSubcriber := subscriber.NewLogSubscriber(logger)

	topics := ctx.StringSlice("topics")
	topicsAsync := ctx.StringSlice("async-topics")

	for i, topic := range append(topics, topicsAsync...) {
		connector, err := connector.NewConnector(
			ps,
			topic,
			creds,
			&types.ControllerConfig{
				RebuildInterval:          time.Millisecond * 1000,
				GatewayURL:               gatewayURL,
				TopicAnnotationDelimiter: ",",
				AsyncFunctionInvocation:  i >= len(topics),
			},
			logSubcriber,
			logger,
		)
		if err != nil {
			return fmt.Errorf("creating connector: %w", err)
		}
		connectors = append(connectors, connector)
	}

	g := &errgroup.Group{}

	for _, connector := range connectors {
		c := connector
		g.Go(func() error {
			return c.Run(ctx.Context, h.ID())
		})
	}

	return g.Wait()
}
