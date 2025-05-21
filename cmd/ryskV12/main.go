package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "ryskV12",
		Usage: "CLI for Rysk V1.2 System",
		Commands: []*cli.Command{
			approveAction, // Refactored and added
			balancesAction, // Refactored and added

			connectAction, // Defined in connect.go (handles disconnect IPC)

			// diconnect.go and diconnectAction removed as 'connect' handles disconnect IPC.

			positionsAction, // Refactored and added

			quoteAction,    // Defined in quote.go
			transferAction, // Defined in transfer.go
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
