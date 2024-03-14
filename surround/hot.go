package surround

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"shelfrobot/config"
)

const uri = "https://api.fgsasd.org/v1api/v3/tokens/token_list?chain=solana&category=trending&pageSize=10&sort=&direction=desc&group=0"

type AveRes struct {
	Status     int    `json:"status"`
	Msg        string `json:"msg"`
	EncodeData string `json:"encode_data"`
	DataType   int    `json:"data_type"`
}

type AvePair struct {
	Pair            string
	Token0Address   string `json:"token0_address"`
	Token0Symbol    string `json:"token0_symbol"`
	Token1Address   string `json:"token1_address"`
	Token1Symbol    string `json:"token1_symbol"`
	Chain           string
	CurrentPriceUsd float64
	PriceChange24h  float64 `json:"price_change_24h"`
	TxVolumeU24h    float64 `json:"tx_volume_u_24h"`
	TxCount24h      int     `json:"tx_count_24h"`
	Holders         int
	Liquidity       float64 `json:"liguidity"`
	MarketCap       float64 `json:"market_cap"`
	Amm             string
	BuyTax          float64
	SellTax         float64
}

func FilterHot() (*AvePair, error) {
	result, err := LoadHotPairs()

	if err != nil {
		return nil, err
	}

	for _, v := range result {
		if v.Token1Address == "So11111111111111111111111111111111111111112" {
			return &v, nil
		}
	}
	return nil, errors.New("notfound")
}

func LoadHotPairs() ([]AvePair, error) {
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, uri, nil)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	req.Header.Add("Signature", config.GetConfig().Sys.Avekey)
	req.Header.Add("X-Auth", config.GetConfig().Sys.Aveauth)

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

	var result AveRes
	err = json.Unmarshal(body, &result)

	if err != nil {
		return nil, err
	}

	if result.Status != 1 {
		//success
		return nil, errors.New(result.Msg)
	}

	encodeData, err := base64.StdEncoding.DecodeString(result.EncodeData)
	if err != nil {
		log.Println("ave转base64错误", encodeData, err)
		return nil, errors.New("reset_uri_error")
	}
	encodeDataJson, err := url.QueryUnescape(string(encodeData))
	if err != nil {
		log.Println("ave转码错误", encodeData, err)
		return nil, errors.New("reset_uri_error")
	}

	var revd []AvePair
	err = json.Unmarshal([]byte(encodeDataJson), &revd)
	if err != nil {
		log.Println("ave转json错误", encodeDataJson, err)
		return nil, errors.New("reset_uri_error")
	}

	return revd, nil
}
