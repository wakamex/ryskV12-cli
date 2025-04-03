package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/urfave/cli/v2"
)

type Response struct {
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	Signature string `json:"signature,omitempty"`
}

var conn *websocket.Conn

func connect(url string) *websocket.Conn {
	var err error
	fmt.Printf(`{"status":"connecting", "url":%s}`, url)
	conn, _, err = websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}

	fmt.Printf(`{"status":"connected", "url":%s}`, url)
	return conn
}

func listenMessages(conn *websocket.Conn) {
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			return
		}
		fmt.Printf(`{"status":"received","message":"%s"}`+"\n", msg)
	}
}

func pipeCommands(fifoPath string, ch chan<- []byte) {
	if _, err := os.Stat(fifoPath); err == nil {
		log.Fatalf("error: FIFO already exists at %s", fifoPath)
	} else if !os.IsNotExist(err) {
		log.Fatalf("error checking FIFO: %v", err)
	}
	log.Printf("Mk fifo")
	err := syscall.Mkfifo(fifoPath, 0666)
	if err != nil {
		log.Fatalf("failed to create FIFO: %v", err)
	}
	log.Printf("Open")
	fifo, err := os.OpenFile(fifoPath, os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		log.Fatalf("failed to open fifo: %v", err)
	}
	scanner := bufio.NewScanner(fifo)

	for scanner.Scan() {
		ch <- scanner.Bytes()
	}
}

func writeToFifo(channelID string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("invalid payload: %v", err)
	}

	fifo, err := os.OpenFile(fmt.Sprintf("/tmp/%s", channelID), os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		return fmt.Errorf("failed to open fifo: %v", err)
	}

	_, err = fifo.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to FIFO: %v", err)
	}
	return nil
}

func transfer(c *cli.Context) error {
	channelID := c.String("channel_id")
	nonce := c.String("nonce")
	payload := JsonRPCRequest{
		JsonRPC: "2.0",
		ID:      nonce,
		Method:  "transfer",
	}

	t := Transfer{
		Asset:     c.String("asset"),
		ChainID:   int(c.Int64("chain_id")),
		Amount:    c.String("amount"),
		IsDeposit: c.Bool("is_deposit"),
		Nonce:     nonce,
	}

	msgHash, _, _ := CreateTransferMessage(t)
	sig, _ := Sign(msgHash, c.String("private_key"))
	t.Signature = sig
	payload.Params = t
	return writeToFifo(channelID, payload)

}

func quote(c *cli.Context) error {
	channelID := c.String("channel_id")
	reply_id := c.String("reply_id")
	pk := c.String("private_key")

	payload := JsonRPCRequest{
		JsonRPC: "2.0",
		ID:      reply_id,
		Method:  "quote",
	}

	q := Quote{
		AssetAddress: c.String("asset"),
		ChainID:      c.Int("chain_id"),
		Expiry:       c.Int64("expiry"),
		IsPut:        c.Bool("is_put"),
		IsTakerBuy:   c.Bool("is_taker_buy"),
		Maker:        c.String("maker"),
		Nonce:        c.String("nonce"),
		Price:        c.String("price"),
		Quantity:     c.String("quantity"),
		Strike:       c.String("strike"),
		ValidUntil:   c.Int64("valid_until"),
	}

	msgHash, _, _ := CreateQuoteMessage(q)
	sig, _ := Sign(msgHash, pk)
	q.Signature = sig
	payload.Params = q

	return writeToFifo(channelID, payload)
}

func daemon(c *cli.Context) error {
	channel_id := c.String("channel_id")
	fifopath := fmt.Sprintf("/tmp/%s", channel_id)
	cmdChan := make(chan []byte)
	go pipeCommands(fifopath, cmdChan)

	conn := connect(c.String("url"))
	go listenMessages(conn)

	for cmd := range cmdChan {
		conn.WriteMessage(websocket.TextMessage, cmd)
	}

	return nil
}

func main() {
	app := &cli.App{
		Name: "rysk-v12-cli",
		Commands: []*cli.Command{
			{
				Name:  "transfer",
				Usage: "request a transfer",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "channel_id",
						Required: true,
						Usage:    "the socket id to send messages into",
					},
					&cli.Int64Flag{
						Name:     "chain_id",
						Required: true,
						Usage:    "chain_id",
					},
					&cli.StringFlag{
						Name:     "asset",
						Required: true,
						Usage:    "asset address",
					},
					&cli.StringFlag{
						Name:     "amount",
						Required: true,
						Usage:    "amount to deposit",
					},
					&cli.BoolFlag{
						Name:     "is_deposit",
						Required: true,
						Usage:    "whether you want to deposit or withdraw",
					},
					&cli.BoolFlag{
						Name:     "nonce",
						Required: true,
						Usage:    "nonce to sign the message with",
					},
					&cli.StringFlag{
						Name:     "private_key",
						Required: true,
						Usage:    "private key to sign messages with",
					},
				},
				Action: func(c *cli.Context) error {
					return transfer(c)
				},
			},
			{
				Name:  "quote",
				Usage: "Send a quote",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "channel_id",
						Required: true,
						Usage:    "the socket id to send messages into",
					},
					&cli.IntFlag{
						Name:     "chain_id",
						Required: true,
					},
					&cli.Int64Flag{
						Name:     "expiry",
						Required: true,
					},
					&cli.BoolFlag{
						Name:     "is_put",
						Required: true,
					},
					&cli.BoolFlag{
						Name:     "is_taker_buy",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "maker",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "nonce",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "price",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "quantity",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "strike",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "valid_until",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "private_key",
						Required: true,
						Usage:    "private key to sign messages with",
					},
				},
				Action: func(c *cli.Context) error {
					return quote(c)
				},
			},
			{
				Name:  "start",
				Usage: "Run the CLI in daemon mode",
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
					return daemon(c)
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
