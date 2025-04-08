package main

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/urfave/cli/v2"
)

var approveAction = &cli.Command{
	Name:  "approve",
	Usage: "approve spending of default strike asset",
	Flags: []cli.Flag{
		&cli.Int64Flag{
			Name:     "chain_id",
			Required: true,
			Usage:    "chain_id",
		},
		&cli.StringFlag{
			Name:     "rpc_url",
			Required: true,
			Usage:    "rpc url",
		},
		&cli.StringFlag{
			Name:     "amount",
			Required: true,
			Usage:    "amount to approve",
		},
		&cli.StringFlag{
			Name:     "private_key",
			Required: true,
			Usage:    "private key of approving account",
		},
	},
	Action: func(c *cli.Context) error {
		return approve(c)
	},
}

func approve(c *cli.Context) error {
	chain_id := c.Int("chain_id")
	rpc_url := c.String("rpc_url")
	amount := c.String("amount")
	pk := c.String("private_key")

	account, err := newAccountFromPrivateKey(pk)
	if err != nil {
		return err
	}

	bigAmount, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return fmt.Errorf("%s cannot be turned into a big.Int", amount)
	}

	client, err := ethclient.DialContext(c.Context, rpc_url)
	if err != nil {
		return err
	}

	return account.approve(c.Context, chain_id, *client, bigAmount)
}
