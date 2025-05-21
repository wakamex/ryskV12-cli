package main

import (
	"github.com/urfave/cli/v2"
	"github.com/rysk-finance/rysk-v12-cli/ryskcore" // Adjust if your fork's module path is different
)

// JsonRPCRequest needs to be defined in this package or imported if it's a shared type for the CLI.
// For example:
// type JsonRPCRequest struct {
// 	JsonRPC string      `json:"jsonrpc"`
// 	ID      string      `json:"id"`
// 	Method  string      `json:"method"`
// 	Params  interface{} `json:"params,omitempty"`
// }
// 
// writeToSocket also needs to be defined or imported for this package.
// func writeToSocket(channelID string, payload interface{}) error { /* ... */ return nil }

var quoteAction = &cli.Command{
	Name:  "quote",
	Usage: "Send a quote",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "channel_id",
			Required: true,
			Usage:    "the socket id to send messages into",
		},
		&cli.StringFlag{
			Name:     "rfq_id",
			Required: true,
			Usage:    "the rfq id to respond to",
		},
		&cli.StringFlag{
			Name:     "asset",
			Required: true,
			Usage:    "asset address",
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
			Name: "is_put",
		},
		&cli.BoolFlag{
			Name: "is_taker_buy",
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
			Name:     "valid_until", // Corrected from "valid_untill"
			Required: true,
		},
		&cli.StringFlag{
			Name:     "private_key",
			Required: true,
			Usage:    "private key to sign messages with",
		},
	},
	Action: func(c *cli.Context) error {
		return quoteCmdFunc(c) // Renamed to avoid conflict if quote were a type
	},
}

func quoteCmdFunc(c *cli.Context) error {
	channelID := c.String("channel_id")
	rfqID := c.String("rfq_id") // Corrected variable name to rfqID for consistency
	pk := c.String("private_key")

	// Assuming JsonRPCRequest is defined in this 'main' package scope or imported.
	payload := JsonRPCRequest{
		JsonRPC: "2.0",
		ID:      rfqID,
		Method:  "quote",
	}

	q := ryskcore.Quote{
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

	msgHash, _, err := ryskcore.CreateQuoteMessage(q)
	if err != nil {
		return err
	}
	sig, err := ryskcore.Sign(msgHash, pk)
	if err != nil {
		return err
	}
	q.Signature = sig
	payload.Params = q

	// Assuming writeToSocket is defined in this 'main' package scope or imported.
	return writeToSocket(channelID, payload)
}
