package cmd

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/clstb/ipfs-connector/pkg/connector"
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

// ResponseReceiver enables connector to receive results from the
// function invocation
type ResponseReceiver struct{}

// Response is triggered by the controller when a message is
// received from the function invocation
func (ResponseReceiver) Response(res types.InvokerResponse) {
	if res.Error != nil {
		log.Printf("tester got error: %s", res.Error.Error())
	} else {
		log.Printf("tester got result: [%d] %s => %s (%d) bytes", res.Status, res.Topic, res.Function, len(*res.Body))
	}
}

func Run(ctx *cli.Context) error {
	// create a new libp2p Host that listens on a random TCP port
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"))
	if err != nil {
		log.Fatal(err)
	}

	// create a new PubSub service using the GossipSub router
	ps, err := pubsub.NewGossipSub(ctx.Context, h)
	if err != nil {
		log.Fatal(err)
	}

	// setup local mDNS discovery
	if err := setupDiscovery(h); err != nil {
		log.Fatal(err)
	}

	gatewayURL := ctx.String("gateway-url")

	creds := &auth.BasicAuthCredentials{
		User:     ctx.String("gateway-user"),
		Password: ctx.String("gateway-password"),
	}

	var connectors []*connector.Connector

	for _, topic := range ctx.StringSlice("topics") {
		connector, err := connector.NewConnector(
			ps,
			topic,
			creds,
			&types.ControllerConfig{
				RebuildInterval:          time.Millisecond * 1000,
				GatewayURL:               gatewayURL,
				PrintResponse:            true,
				PrintResponseBody:        true,
				TopicAnnotationDelimiter: ",",
				AsyncFunctionInvocation:  false,
			},
			&ResponseReceiver{},
		)
		if err != nil {
			return fmt.Errorf("creating connector: %w", err)
		}
		connectors = append(connectors, connector)
	}

	for _, topic := range ctx.StringSlice("topics-async") {
		connector, err := connector.NewConnector(
			ps,
			topic,
			creds,
			&types.ControllerConfig{
				RebuildInterval:          time.Millisecond * 1000,
				GatewayURL:               gatewayURL,
				PrintResponse:            true,
				PrintResponseBody:        true,
				TopicAnnotationDelimiter: ",",
				AsyncFunctionInvocation:  true,
			},
			&ResponseReceiver{},
		)
		if err != nil {
			return fmt.Errorf("creating connector: %w", err)
		}
		connectors = append(connectors, connector)
	}

	g := &errgroup.Group{}

	for _, connector := range connectors {
		g.Go(func() error {
			return connector.Run(ctx.Context, h.ID())
		})
	}

	return g.Wait()
}
