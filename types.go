package main

type Transfer struct {
	Asset     string `json:"asset"`
	ChainID   int    `json:"chainId"`
	Amount    string `json:"amount"`
	IsDeposit bool   `json:"isDeposit"`
	Nonce     string `json:"nonce"`
	Signature string `json:"signature"`
}

type Request struct {
	Asset      string `json:"asset"`
	AssetName  string `json:"assetName"`
	ChainID    int    `json:"chainId"`
	Expiry     int64  `json:"expiry"`
	IsPut      bool   `json:"isPut"`
	IsTakerBuy bool   `json:"isTakerBuy"`
	Quantity   string `json:"quantity"`
	Strike     string `json:"strike"`
	Taker      string `json:"taker"`
}

type Quote struct {
	AssetAddress string  `json:"assetAddress"`
	ChainID      int     `json:"chainId"`
	Expiry       int64   `json:"expiry"`
	IsPut        bool    `json:"isPut"`
	IsTakerBuy   bool    `json:"isTakerBuy"`
	Maker        string  `json:"maker"`
	Nonce        string  `json:"nonce"`
	Price        string  `json:"price"`
	Quantity     string  `json:"quantity"`
	Strike       string  `json:"strike"`
	Signature    string  `json:"signature"`
	ValidUntil   int64   `json:"validUntil"`
}
