package ipfs

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/ipfs/go-ipfs/config"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/node/libp2p"
	"github.com/ipfs/go-ipfs/plugin/loader"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	icore "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"go.uber.org/zap"
)

type IPFS struct {
	icore.CoreAPI
	NodeId        string
	subscriptions map[string]struct{}
	messages      chan icore.PubSubMessage
	logger        *zap.Logger
}

func New(
	ctx context.Context,
	logger *zap.Logger,
	repository string,
) (*IPFS, error) {
	plugins, err := loader.NewPluginLoader("plugins")
	if err != nil {
		return nil, fmt.Errorf("error loading plugins: %s", err)
	}
	// Load preloaded and external plugins
	if err := plugins.Initialize(); err != nil {
		return nil, fmt.Errorf("error initializing plugins: %s", err)
	}

	if err := plugins.Inject(); err != nil {
		return nil, fmt.Errorf("error initializing plugins: %s", err)
	}

	if !fsrepo.IsInitialized(repository) {
		cfg, err := config.Init(ioutil.Discard, 4096)
		if err != nil {
			return nil, err
		}

		cfg.Bootstrap = []string{}
		cfg.Discovery.MDNS.Enabled = true
		cfg.Experimental.FilestoreEnabled = true
		cfg.Experimental.UrlstoreEnabled = true
		cfg.Experimental.Libp2pStreamMounting = true
		cfg.Experimental.P2pHttpProxy = true
		cfg.Experimental.StrategicProviding = true
		cfg.Pubsub.Enabled = config.True
		cfg.Pubsub.Router = "gossipsub"
		cfg.Pubsub.DisableSigning = true

		if err := fsrepo.Init(repository, cfg); err != nil {
			return nil, fmt.Errorf("failed to init ephemeral node: %s", err)
		}
	}

	repo, err := fsrepo.Open(repository)
	if err != nil {
		return nil, err
	}

	node, err := core.NewNode(ctx, &core.BuildCfg{
		Online:    true,
		Routing:   libp2p.DHTOption,
		Repo:      repo,
		Permanent: true,
		ExtraOpts: map[string]bool{
			"pubsub": true,
		},
	})

	if err != nil {
		return nil, err
	}

	api, err := coreapi.NewCoreAPI(node)
	if err != nil {
		return nil, err
	}

	return &IPFS{
		CoreAPI:       api,
		NodeId:        node.Identity.String(),
		subscriptions: map[string]struct{}{},
		messages:      make(chan icore.PubSubMessage),
		logger:        logger.With(zap.String("component", "ipfs")),
	}, nil
}

func (i *IPFS) Subscribe(
	ctx context.Context,
	topic string,
) error {
	if _, ok := i.subscriptions[topic]; ok {
		return nil
	}

	subscription, err := i.PubSub().Subscribe(ctx, topic, options.PubSub.Discover(true))
	if err != nil {
		return err
	}

	i.subscriptions[topic] = struct{}{}
	go func() {
		i.logger.Info("subscribed to topic", zap.String("topic", topic))
		defer delete(i.subscriptions, topic)
		for {
			msg, err := subscription.Next(ctx)
			if err != nil {
				i.logger.Error("receiving message")
				continue
			}

			i.messages <- msg
		}
	}()

	return nil
}

func (i *IPFS) Messages() <-chan icore.PubSubMessage {
	return i.messages
}
