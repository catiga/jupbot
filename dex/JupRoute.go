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

	bin "github.com/gagliardetto/binary"
	solana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"

	"shelfrobot/config"
)

type JupGag struct {
	Jup
	userPk  string
	account *solana.Wallet
}

func (t *JupGag) Init() {
	account, err := solana.WalletFromPrivateKeyBase58(t.userPk)
	if err != nil {
		log.Fatal("wrong pk verified")
	}
	t.account = account
}

func (JupGag) Price(targetToken, vsToken string) (decimal.Decimal, error) {
	api := fmt.Sprint("https://price.jup.ag/v4/price", "?", "ids=", targetToken, "&", "vsToken=", vsToken)

	resp, err := http.Get(api)
	if err != nil {
		log.Printf("Get %s vs %s price failed, for %+v", targetToken, vsToken, err)
		return decimal.NewFromInt(0), err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// Handle error
		log.Println("Error reading response:", err)
		return decimal.NewFromInt(0), err
	}

	jsonData := string(body)

	var result map[string]interface{}
	err = json.Unmarshal([]byte(jsonData), &result)
	if err != nil {
		log.Printf("Error parsing JSON: %v", err)
		return decimal.NewFromInt(0), err
	}

	data := result["data"].(map[string]interface{})
	olen := data[targetToken].(map[string]interface{})
	price := fmt.Sprintf("%f", olen["price"])

	return decimal.NewFromString(price)
}

/*
slippage , 100: 1%
*/
func (JupGag) Quote(input, output string, amountDecimals *big.Int, slippage int) (*SwapResponse, error) {
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
		log.Printf("Error parsing JSON: %v", err)
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

func (JupGag) Tokens() ([]string, error) {
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

func (t JupGag) Swap(input, output string, amountDecimals *big.Int, slippage int) (string, decimal.Decimal, error) {
	quote, e := t.Quote(input, output, amountDecimals, slippage)
	if e != nil {
		return "", decimal.NewFromInt(0), e
	}
	url := "https://quote-api.jup.ag/v6/swap"
	method := "POST"
	account := t.account

	params := make(map[string]interface{})
	params["userPublicKey"] = account.PublicKey().String()
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
		return "", decimal.NewFromInt(0), err
	}

	payload := strings.NewReader(string(json_params))

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		return "", decimal.NewFromInt(0), err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return "", decimal.NewFromInt(0), err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", decimal.NewFromInt(0), err
	}

	outAmount, _ := decimal.NewFromString(quote.OutAmount)
	return string(body), outAmount, nil
}

func (t JupGag) SwapAndSend(input, output string, amountDecimals *big.Int, slippage int) (string, decimal.Decimal, error) {
	transaction, outAmount, err := t.Swap(input, output, amountDecimals, slippage)

	if err != nil {
		log.Println(err)
		return "", outAmount, err
	}

	var swapTxMap map[string]interface{}
	err = json.Unmarshal([]byte(transaction), &swapTxMap)
	if err != nil {
		return "", outAmount, err
	}

	txinfo := swapTxMap["swapTransaction"].(string)
	// txLastValidBlockHeight := uint64(swapTxMap["lastValidBlockHeight"].(float64))

	c := rpc.New(config.Get("Solana").Rpc)

	// blockHash, err := c.GetLatestBlockhashAndContextWithConfig(context.Background(), solana.GetLatestBlockhashConfig{
	// 	Commitment: "finalized",
	// })

	blockHash, err := c.GetLatestBlockhash(context.Background(), rpc.CommitmentConfirmed)

	if err != nil {
		return "", outAmount, err
	}

	// solanaBlock, err := c.GetBlock(context.Background(), blockHash.Context.Slot)
	// if err != nil {
	// 	return "", err
	// }

	// recentBlockSlot := solanaBlock.BlockHeight

	b, _ := base64.StdEncoding.DecodeString(txinfo)
	tx, _ := solana.TransactionFromDecoder(bin.NewBinDecoder(b))

	tx.Message.RecentBlockhash = blockHash.Value.Blockhash

	signers := []solana.PrivateKey{
		t.account.PrivateKey,
	}

	signature, err := tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		for _, signer := range signers {
			if key.Equals(signer.PublicKey()) {
				return &signer
			}
		}
		return nil
	})
	tx.Signatures = []solana.Signature{signature[1]}

	verif := tx.VerifySignatures()
	log.Println(verif)

	if err != nil {
		log.Println("sign error")
		return "", outAmount, err
	}

	sebytestr, _ := tx.ToBase64()
	log.Println(sebytestr)

	transactionSimulate, err := c.SimulateTransaction(context.Background(), tx)
	log.Println(transactionSimulate, err)

	transactionSignature, err := c.SendTransaction(context.TODO(), tx)

	if err != nil {
		// log.Println(txLastValidBlockHeight, recentBlockSlot)
		log.Printf("failed to send transaction, err: %v", err)
		return "", outAmount, err
	}

	return transactionSignature.String(), outAmount, nil
}

func (t JupGag) GetMemTx(signatures []string) (interface{}, error) {

	c := rpc.New(config.Get("Solana").Rpc)
	sig, _ := solana.SignatureFromBase58(signatures[0])
	status, err := c.GetSignatureStatuses(context.Background(), true, sig)

	if err != nil {
		return nil, err
	}

	return status, nil
}
