package main

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	crypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	CHAIN_ID_ETH           int = 1
	CHAIN_ID_BASE          int = 8453
	CHAIN_ID_BASE_SEPOLIA  int = 84532
	CHAIN_ID_MONAD_TESTNET int = 10143
)

type Addresses struct {
	MarginPool  common.Address
	StrikeAsset common.Address
}

type Account struct {
	Public  common.Address
	Private *ecdsa.PrivateKey
}

var ADDRESSES = map[int]Addresses{
	CHAIN_ID_BASE_SEPOLIA: {
		MarginPool:  common.HexToAddress("0xcf571347e69751ca2aef54e9ef3adfee8dd94ab8"),
		StrikeAsset: common.HexToAddress("0x98d56648c9b7f3cb49531f4135115b5000ab1733"),
	},
	CHAIN_ID_MONAD_TESTNET: {
		MarginPool:  common.HexToAddress("0xedb6ef7a8534fd9fe8a448a52f98c2fa62f4e9a1"),
		StrikeAsset: common.HexToAddress("0xf817257fed379853cde0fa4f97ab987181b1e5ea"),
	},
}

func newAccountFromPrivateKey(pk string) (account Account, err error) {
	account = Account{}
	account.Private, err = crypto.HexToECDSA(strings.TrimPrefix(pk, "0x")) // hex w/o 0x
	if err != nil {
		return account, err
	}

	account.Public = crypto.PubkeyToAddress(account.Private.PublicKey)
	return account, nil

}

func (a *Account) newTransactionOpts(ctx context.Context, chainID int, c ethclient.Client) (*bind.TransactOpts, error) {
	nonce, err := c.PendingNonceAt(ctx, a.Public)
	if err != nil {
		return nil, err
	}
	gasprice, err := c.SuggestGasPrice(ctx)
	if err != nil {
		return nil, err
	}
	opts, err := bind.NewKeyedTransactorWithChainID(a.Private, new(big.Int).SetInt64(int64(chainID)))
	if err != nil {
		return nil, err
	}
	opts.Nonce = big.NewInt(int64(nonce))
	opts.GasPrice = gasprice
	return opts, nil
}

func (a *Account) approve(ctx context.Context, chainID int, client ethclient.Client, amount *big.Int) (err error) {
	opts, err := a.newTransactionOpts(ctx, chainID, client)
	if err != nil {
		return nil
	}

	erc20, err := NewIERC20(ADDRESSES[chainID].StrikeAsset, &client)
	if err != nil {
		return err
	}

	tx, err := erc20.Approve(opts, ADDRESSES[chainID].MarginPool, amount)
	if err != nil {
		return err
	}

	_, err = bind.WaitMined(ctx, &client, tx)
	if err != nil {
		return err
	}

	return nil
}
