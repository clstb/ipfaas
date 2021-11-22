package main

import (
	"log"
	"os"

	"github.com/clstb/ipfs-connector/cmd"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name: "IPFS-Connector",
		Commands: []*cli.Command{
			{
				Name:  "run",
				Usage: "runs the connector",
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
					&cli.StringSliceFlag{
						Name:    "topics",
						Usage:   "topics forwarded to the gateway with synchronous invocation",
						EnvVars: []string{"TOPICS"},
					},
					&cli.StringSliceFlag{
						Name:    "topics-async",
						Usage:   "topics forwarded to the gateway with asynchronous invocation",
						EnvVars: []string{"TOPICS_ASYNC"},
					},
				},
				Action: cmd.Run,
			},
			{
				Name:  "gossiper",
				Usage: "runs a peer that send dummy messages on a topic",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "topic",
						Required: true,
					},
				},
				Action: cmd.Gossiper,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
