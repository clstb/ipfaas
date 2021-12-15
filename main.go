package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/clstb/ipfs-connector/pkg/openfaas"
	ipfs "github.com/ipfs/go-ipfs-api"
	"github.com/openfaas/connector-sdk/types"
	"github.com/openfaas/faas-provider/auth"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

func main() {
	app := &cli.App{
		Name: "IPFS-Connector",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "gateway-url",
				Usage:    "openfaas gateway url",
				EnvVars:  []string{"GATEWAY_URL"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "gateway-user",
				Usage:    "openfaas gateway username",
				EnvVars:  []string{"GATEWAY_USER"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "gateway-password",
				Usage:    "openfaas gateway password",
				EnvVars:  []string{"GATEWAY_PASSWORD"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "ipfs-url",
				Usage:    "ipfs gateway url",
				EnvVars:  []string{"IPFS_URL"},
				Required: true,
			},
		},
		Action: Run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func Run(ctx *cli.Context) error {
	gatewayURL := ctx.String("gateway-url")
	gatewayUser := ctx.String("gateway-user")
	gatewayPassword := ctx.String("gateway-password")

	client := openfaas.NewClient(gatewayURL, openfaas.WithBasicAuth(
		gatewayUser,
		gatewayPassword,
	))
	controller := types.NewController(
		&auth.BasicAuthCredentials{
			User:     gatewayUser,
			Password: gatewayPassword,
		},
		&types.ControllerConfig{
			RebuildInterval:          time.Millisecond * 1000,
			GatewayURL:               gatewayURL,
			TopicAnnotationDelimiter: ",",
			AsyncFunctionInvocation:  true,
		},
	)
	controller.BeginMapBuilder()

	ipfsURL := ctx.String("ipfs-url")
	shell := ipfs.NewShell(ipfsURL)

	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}
	defer logger.Sync()

	topics := topicWatcher(ctx.Context, controller)
	messages := topicJoiner(logger, topics, shell)

	parsedMessages := parse(logger, messages)
	splitted := fanOut(2, parsedMessages)
	deploy(logger, splitted[0], client)
	responses := invoke(logger, splitted[1], controller)
	publishPS(logger, responses, shell)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	sig := <-sigCh
	logger.Info("received signal", zap.String("signal", sig.String()))

	return nil
}
