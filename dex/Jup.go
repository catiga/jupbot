package dex

import (
	"math/big"

	"shelfrobot/config"
	"shelfrobot/sys"

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

var ins Jup

func init() {
	ins = &JupImpl{
		userPk: config.GetConfig().Wallet.Pk,
	}
	ins.Init()
}

func GetIns() Jup {
	if ins == nil {
		sys.Logger.Fatal("unable to init instance...")
	}
	return ins
}
