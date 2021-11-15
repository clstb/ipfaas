package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/openfaas/connector-sdk/types"
	"github.com/openfaas/faas-provider/auth"
)

type Message struct {
	data  []byte
	topic string
}

func main() {
	ctx := context.Background()

	var (
		gatewayUsername string
		gatewayPassword string
		gatewayFlag     string
		asyncInvoke     bool
	)

	flag.StringVar(&gatewayUsername, "gw-username", "admin", "Username for the gateway")
	flag.StringVar(&gatewayPassword, "gw-password", "", "Password for gateway")
	flag.StringVar(&gatewayFlag, "gateway", "", "gateway")
	flag.BoolVar(&asyncInvoke, "async-invoke", false, "Invoke via queueing using NATS and the function's async endpoint")
	flag.Parse()

	var creds *auth.BasicAuthCredentials
	if len(gatewayPassword) > 0 {
		creds = &auth.BasicAuthCredentials{
			User:     gatewayUsername,
			Password: gatewayPassword,
		}
	} else {
		creds = types.GetCredentials()
	}

	gatewayURL := os.Getenv("gateway_url")

	if len(gatewayFlag) > 0 {
		gatewayURL = gatewayFlag
	}

	if len(gatewayURL) == 0 {
		log.Panicln(`a value must be set for env "gatewayURL" or via the -gateway flag for your OpenFaaS gateway`)
		return
	}

	config := &types.ControllerConfig{
		RebuildInterval:          time.Millisecond * 1000,
		GatewayURL:               gatewayURL,
		PrintResponse:            true,
		PrintResponseBody:        true,
		TopicAnnotationDelimiter: ",",
		AsyncFunctionInvocation:  asyncInvoke,
	}

	// create openfaas controller
	controller := types.NewController(creds, config)

	// create response receiver
	receiver := ResponseReceiver{}

	// subscribe controller to receiver
	controller.Subscribe(&receiver)
	controller.BeginMapBuilder()

	// create a new libp2p Host that listens on a random TCP port
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"))
	if err != nil {
		log.Fatal(err)
	}

	// create a new PubSub service using the GossipSub router
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		log.Fatal(err)
	}

	// setup local mDNS discovery
	if err := setupDiscovery(h); err != nil {
		log.Fatal(err)
	}

	// join the pubsub topic
	topic, err := ps.Join("test-topic")
	if err != nil {
		log.Fatal(err)
	}

	// and subscribe to it
	sub, err := topic.Subscribe()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			if err := topic.Publish(
				ctx,
				[]byte(fmt.Sprintf("hello from peer %s", h.ID())),
			); err != nil {
				log.Fatal(err)
			}
			time.Sleep(5 * time.Second)

		}
	}()

	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			log.Fatal(err)
		}

		// only forward messages delivered by others
		if msg.ReceivedFrom == h.ID() {
			continue
		}

		log.Printf("Invoking (%s) on topic: %q, value: %q", gatewayURL, topic, msg.Data)

		controller.Invoke(
			msg.GetTopic(),
			&msg.Data,
			http.Header{},
		)
	}
}

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
