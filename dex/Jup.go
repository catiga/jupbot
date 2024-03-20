package dex

import (
	"math/big"

	"shelfrobot/config"

	"github.com/shopspring/decimal"
)

type Jup interface {
	Init()
	Price(targetToken, vsToken string) (decimal.Decimal, error)
	Quote(input, output string, amountDecimals *big.Int, slippage int) (*SwapResponse, error)
	Swap(input, output string, amountDecimals *big.Int, slippage int) (string, decimal.Decimal, error)
	Tokens() ([]string, error)
	SwapAndSend(input, output string, amountDecimals *big.Int, slippage int) (string, decimal.Decimal, error)
	GetMemTx(signature string) error
	IsTxSuccess(targetToken, signature string) (string, decimal.Decimal, error)
}

// func init() {
// 	ins = &JupImpl{
// 		userPk: config.GetConfig().Wallet.Pk,
// 	}
// 	ins.Init()
// }

var dexHandlers = make(map[string]Jup)

func GetIns(d config.Dex) Jup {
	ins, ok := dexHandlers[d.Name]
	if !ok {
		if d.Chain == "Solana" {
			ins = &JupImpl{
				userPk: d.Wallet.Pk,
			}
			ins.Init()
			dexHandlers[d.Name] = ins
		}
	}
	return ins
}
