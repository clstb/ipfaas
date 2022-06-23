package main

import (
	"log"
	"os"
	"runtime"

	_ "net/http/pprof"

	"github.com/clstb/ipfaas/pkg/server"
	"github.com/containerd/containerd"
	"github.com/openfaas/faas-provider/types"
	"github.com/openfaas/faasd/pkg/cninetwork"
	"github.com/openfaas/faasd/pkg/provider/config"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

func main() {
	runtime.SetBlockProfileRate(10000)
	app := &cli.App{
		Name: "ipfaas",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "pull-policy",
				Value: "Always",
				Usage: `Set to "Always" to force a pull of images upon deployment, or "IfNotPresent" to try to use a cached image.`,
			},
		},
		Action: Run,
	}
	log.Fatal(app.Run(os.Args))
}

func Run(ctx *cli.Context) error {
	_, providerConfig, err := config.ReadFromEnv(types.OsEnv{})
	if err != nil {
		return err
	}

	cni, err := cninetwork.InitNetwork()
	if err != nil {
		return err
	}

	client, err := containerd.New(providerConfig.Sock)
	if err != nil {
		return err
	}

	loggerConfig := zap.NewProductionConfig()
	loggerConfig.Level.SetLevel(zap.DebugLevel)
	logger, err := loggerConfig.Build()
	if err != nil {
		return err
	}
	defer logger.Sync()

	server, err := server.New(
		ctx.Context,
		logger,
		client,
		cni,
		"./ipfs",
	)
	if err != nil {
		return err
	}

	return server.Listen(":80")
}
