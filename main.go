package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func writeToFifo(channelID string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("invalid payload: %v", err)
	}

	fifo, err := os.OpenFile(fmt.Sprintf("/tmp/%s", channelID), os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		return fmt.Errorf("failed to open fifo: %v", err)
	}
	defer fifo.Close()

	_, err = fifo.Write(append(data, '\n'))
	if err != nil {
		return fmt.Errorf("failed to write to FIFO: %v", err)
	}
	return nil
}

func main() {
	app := &cli.App{
		Name: "rysk-v12-cli",
		Commands: []*cli.Command{
			approveAction,
			balancessAction,
			connectAction,
			positionsAction,
			quoteAction,
			transferAction,
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
