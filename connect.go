package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/urfave/cli/v2"
)

var connectAction = &cli.Command{
	Name:  "connect",
	Usage: "instantiate a websocket connection",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "channel_id",
			Required: true,
			Usage:    "a unique id so you can later interact with this specific websocket",
		},
		&cli.StringFlag{
			Name:     "url",
			Required: true,
			Usage:    "ws url to connect to",
		},
	},
	Action: func(c *cli.Context) error {
		return connect(c)
	},
}

func connect(c *cli.Context) error {
	channel_id := c.String("channel_id")
	fifopath := fmt.Sprintf("/tmp/%s", channel_id)
	cmdChan := make(chan []byte)
	go pipeCommands(fifopath, cmdChan)
	// ws connection
	ctx, cancel := context.WithCancel(c.Context)
	conn, _, err := websocket.DefaultDialer.Dial(c.String("url"), nil)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	client := NewClient(ctx, cancel, conn)
	client.SetHandler(func(msg []byte) {
		fmt.Println(string(msg))
	})
	go listenMessages(client)

	for {
		select {
		case <-ctx.Done():
			return nil
		case cmd := <-cmdChan:
			fmt.Println(string(cmd))
			client.Send(cmd)
		}
	}
}

func listenMessages(client *Client) {
	for {
		select {
		case <-client.Ctx.Done():
			log.Print("connection closed by peer")
			return
		default:
			_, msg, err := client.Connection.ReadMessage()
			if err != nil {
				log.Fatalf("Read error: %s", err.Error())
			}
			client.Ingest(msg)
		}
	}
}

func pipeCommands(fifoPath string, ch chan<- []byte) {
	if _, err := os.Stat(fifoPath); err == nil {
		log.Fatalf("error: FIFO already exists at %s", fifoPath)
	} else if !os.IsNotExist(err) {
		log.Fatalf("error checking FIFO: %v", err)
	}
	err := syscall.Mkfifo(fifoPath, 0666)
	if err != nil {
		log.Fatalf("failed to create FIFO: %v", err)
	}
	fifo, err := os.OpenFile(fifoPath, os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		log.Fatalf("failed to open fifo: %v", err)
	}
	scanner := bufio.NewScanner(fifo)
	for scanner.Scan() {
		ch <- scanner.Bytes()
	}
}
