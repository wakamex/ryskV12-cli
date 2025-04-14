package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name: "rysk-v12-cli",
		Commands: []*cli.Command{
			approveAction,
			balancessAction,
			connectAction,
			diconnectAction,
			positionsAction,
			quoteAction,
			transferAction,
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
