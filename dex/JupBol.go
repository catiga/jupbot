package dex

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"strings"

	"github.com/shopspring/decimal"

	solana "github.com/blocto/solana-go-sdk/client"
	"github.com/blocto/solana-go-sdk/rpc"
	"github.com/blocto/solana-go-sdk/types"

	"shelfrobot/config"
	"shelfrobot/sys"
)

type JupImpl struct {
	Jup
	userPk  string
	account *types.Account
}

func (t *JupImpl) Init() {
	signer, err := types.AccountFromBase58(t.userPk)
	if err != nil {
		sys.Logger.Fatal("wrong pk verified")
	}
	t.account = &signer
}

func (*JupImpl) Price(targetToken, vsToken string) (decimal.Decimal, error) {
	api := fmt.Sprint("https://price.jup.ag/v4/price", "?", "ids=", targetToken, "&", "vsToken=", vsToken)

	resp, err := http.Get(api)
	if err != nil {
		sys.Logger.Printf("Get %s vs %s price failed, for %+v", targetToken, vsToken, err)
		return decimal.NewFromInt(0), err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// Handle error
		sys.Logger.Println("Error reading response:", err)
		return decimal.NewFromInt(0), err
	}

	jsonData := string(body)

	var result map[string]interface{}
	err = json.Unmarshal([]byte(jsonData), &result)
	if err != nil {
		sys.Logger.Printf("Error parsing JSON: %v", err)
		return decimal.NewFromInt(0), err
	}

	// 这里要判断被封的情况
	data, ok := result["data"].(map[string]interface{})
	if !ok {
		sys.Logger.Println("获取价格出错")
		return decimal.NewFromInt(0), errors.New("price_got_error")
	}
	olen := data[targetToken].(map[string]interface{})
	price := olen["price"].(float64)

	return decimal.NewFromFloat(price), nil
}

/*
slippage , 100: 1%
*/
func (*JupImpl) Quote(input, output string, amountDecimals *big.Int, slippage int) (*SwapResponse, error) {
	url := fmt.Sprint("https://quote-api.jup.ag/v6/quote", "?", "inputMint=", input, "&", "outputMint=", output, "&", "amount=", amountDecimals.String())
	if slippage > 0 {
		url = fmt.Sprint(url, "&", "slippageBps=", slippage)
	}

	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		sys.Logger.Printf("Error parsing JSON: %v", err)
		return nil, err
	}

	errMsg, ok := result["error"]
	if ok {
		return nil, errors.New(errMsg.(string))
	}

	var response SwapResponse

	// 解析JSON字符串
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (*JupImpl) Tokens() ([]string, error) {
	url := "https://quote-api.jup.ag/v6/tokens"
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	req.Header.Add("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var result []string
	err = json.Unmarshal(body, &result)

	return result, err
}

func (t *JupImpl) Swap(input, output string, amountDecimals *big.Int, slippage int) (string, decimal.Decimal, error) {
	quote, e := t.Quote(input, output, amountDecimals, slippage)
	if e != nil {
		return "", decimal.NewFromInt(0), e
	}
	url := "https://quote-api.jup.ag/v6/swap"
	method := "POST"
	account := t.account

	params := make(map[string]interface{})
	params["userPublicKey"] = account.PublicKey.String()
	params["wrapAndUnwrapSol"] = true
	params["useSharedAccounts"] = true
	// params["feeAccount"] = "设置fee account"
	// params["computeUnitPriceMicroLamports"] = 0
	params["prioritizationFeeLamports"] = 0
	params["asLegacyTransaction"] = false
	params["useTokenLedger"] = false
	// params["destinationTokenAccount"] = "设置destinationTokenAccount"
	params["dynamicComputeUnitLimit"] = true
	params["skipUserAccountsRpcCalls"] = true
	params["quoteResponse"] = *quote

	json_params, err := json.Marshal(params)
	if err != nil {
		return "", decimal.NewFromInt(0), e
	}

	payload := strings.NewReader(string(json_params))

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		return "", decimal.NewFromInt(0), e
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return "", decimal.NewFromInt(0), e
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", decimal.NewFromInt(0), e
	}

	outAmount, _ := decimal.NewFromString(quote.OutAmount)
	return string(body), outAmount, nil
}

func (t *JupImpl) SwapAndSend(input, output string, amountDecimals *big.Int, slippage int) (string, decimal.Decimal, error) {
	transaction, outAmount, err := t.Swap(input, output, amountDecimals, slippage)

	if err != nil {
		sys.Logger.Println(err)
		return "", outAmount, err
	}

	var swapTxMap map[string]interface{}
	err = json.Unmarshal([]byte(transaction), &swapTxMap)
	if err != nil {
		return "", outAmount, err
	}

	txinfo, ok := swapTxMap["swapTransaction"].(string)
	if !ok {
		log.Println("TX:ERROR:", transaction)
		return "", outAmount, errors.New("can_not_get_swap_txinfo")
	}
	txLastValidBlockHeight := uint64(swapTxMap["lastValidBlockHeight"].(float64))

	c := solana.NewClient(config.Get("Solana").Rpc)

	blockHash, err := c.GetLatestBlockhashAndContextWithConfig(context.Background(), solana.GetLatestBlockhashConfig{
		Commitment: rpc.CommitmentConfirmed,
	})
	if err != nil {
		return "", outAmount, err
	}

	b, _ := base64.StdEncoding.DecodeString(txinfo)
	tx, _ := types.TransactionDeserialize(b)
	tx.Message.RecentBlockHash = blockHash.Value.Blockhash

	signer := t.account

	serializedMessage, _ := tx.Message.Serialize()

	signature := signer.Sign(serializedMessage)
	err = tx.AddSignature(signature)
	if err != nil {
		return "", outAmount, err
	}

	// sebyes, _ := tx.Serialize()
	// sebytestr := base64.StdEncoding.EncodeToString(sebyes)
	// log.Println(sebytestr)

	// transactionSimulate, err := c.SimulateTransaction(context.Background(), tx)
	// log.Println(transactionSimulate, err)

	currentBlockHeight, err := c.RpcClient.GetBlockHeight(context.Background())

	txs, _ := tx.Serialize()
	txss := base64.StdEncoding.EncodeToString(txs)

	var transactionSignature string
	for currentBlockHeight.Result < txLastValidBlockHeight {
		transactionSignature, err = c.SendTransaction(context.Background(), tx)
		if err == nil {
			break
		}
		// log.Println("")
		// log.Println(currentBlockHeight.Result, " ", txLastValidBlockHeight, err)
		// log.Println(txss)
		// log.Println("")

		currentBlockHeight, err = c.RpcClient.GetBlockHeight(context.Background())
	}

	if err != nil {
		sys.Logger.Printf("failed to send transaction, err: %v", err)
		return "", outAmount, err
	}

	sys.Logger.Println("submitted tx base64: ", txss)

	return transactionSignature, outAmount, nil
}

func (t *JupImpl) GetMemTx(signatures []string) (interface{}, error) {

	c := solana.NewClient(config.Get("Solana").Rpc)
	status, err := c.GetSignatureStatusesWithConfig(context.Background(), signatures, solana.GetSignatureStatusesConfig{
		SearchTransactionHistory: true,
	})
	if len(status) == 0 && err == nil {
		return nil, errors.New("transactions might be dropped")
	} else if status == nil && err != nil {
		return nil, err
	}
	return status, nil
}

func (t *JupImpl) IsTxSuccess(targetToken, signature string) (string, decimal.Decimal, error) {
	c := solana.NewClient(config.Get("Solana").Rpc)
	// c := solana.NewClient("https://api.mainnet-beta.solana.com")
	// status, e := c.GetSignatureStatus(context.Background(), signature)

	// if e != nil {
	// 	return "error", e
	// }

	// if status == nil {
	// 	return "waiting", nil
	// }

	// if *status.ConfirmationStatus == rpc.CommitmentFinalized || *status.ConfirmationStatus == rpc.CommitmentConfirmed {
	// 	tx, _ := c.GetTransaction(context.Background(), signature)
	// 	if tx == nil {
	// 		return "waiting", nil
	// 	}

	// 	if tx.Meta.Err != nil {
	// 		sys.Logger.Println("交易详情内的错误：", tx.Meta.Err)
	// 		return "error", errors.New("tx.Meta.Err")
	// 	}
	// 	return "success", nil
	// }

	tx, e := c.GetTransaction(context.Background(), signature)

	if e != nil {
		return "error", decimal.NewFromInt(0), e
	}
	if tx == nil {
		return "waiting", decimal.NewFromInt(0), nil
	}
	if tx.Meta.Err != nil {
		sys.Logger.Println("交易详情内的错误：", tx.Meta.Err)
		return "error", decimal.NewFromInt(0), errors.New("tx.Meta.Err")
	}
	userWallet := t.account.PublicKey.String()
	for _, v := range tx.Meta.PostTokenBalances {
		if v.Owner == userWallet && v.Mint == targetToken {
			realAmount, _ := decimal.NewFromString(v.UITokenAmount.Amount)
			return "success", realAmount, nil
		}
	}
	sys.Logger.Println("这里不该出现", signature)
	return "waiting", decimal.NewFromInt(0), nil
}
