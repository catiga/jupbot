package main

import (
	"flag"
	"log"
	"shelfrobot/config"
	_ "shelfrobot/database"
	"shelfrobot/dex"
	"shelfrobot/surround"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"shelfrobot/sys"
)

const usdc = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

var buy, sell, status, ongoing, reset = operateSwap()
var logger = sys.Logger

func main() {
	defer sys.LogFile.Close()
	targetTokenPtr := flag.String("o", "", "The target token")
	// vsTokenPtr := flag.String("s", "", "The versus token")
	basePriceStr := flag.String("p", "", "The base price to U")

	flag.Parse()

	if *targetTokenPtr == "" {
		logger.Fatal("must provide both target token and versus token")
	}

	baseLinePrice, err := decimal.NewFromString(*basePriceStr)
	basePrice := decimal.NewFromInt(0)
	if err != nil {
		// logger.Fatal("please refer a regula baseline price")
		logger.Println("TX:SYSCONF:未设置限价，将直接触发买入")
	} else {
		if baseLinePrice.Cmp(decimal.NewFromInt(0)) != 1 {
			logger.Fatal("please refer a regula baseline price")
		}
		basePrice = baseLinePrice
	}

	var targetToken = *targetTokenPtr

	pChan := make(chan decimal.Decimal)

	logger.Println("TX:SYSCONF:目标token ", targetToken, " 基准价格 ", basePrice.String())
	var round = false
	var lastBuyTs = int64(0)
	var mutex sync.Mutex

	go func() {
		for {
			if !round {
				mutex.Lock()
				round = true
				mutex.Unlock()
				i := dex.GetIns()
				p, err := i.Price(targetToken, usdc)

				if err == nil {
					pChan <- p
				}
			}
			time.Sleep(1 * time.Second)
		}
	}()

	var buyHash, sellHash string
	var buyStageDiff = 0
	var lastPrintSellTs = int64(0)
	for {
		select {
		case price := <-pChan:
			if !ongoing() {
				if basePrice.Cmp(decimal.NewFromInt(0)) == 0 {
					basePrice = price
				}
				logger.Println("TX:SYSCONF:当前价格 ", price, " 基准价格 ", basePrice, " 浮动超过 ", price.Sub(basePrice).Abs().Div(price).StringFixed(4), "%", " 交易控制状态：", status())
			}

			if status() {
				if price.Cmp(basePrice) != 1 {
					buyHash = buy(targetToken, config.GetConfig().Dex.Vstoken, basePrice, price)
					if ongoing() {
						basePrice = price
						logger.Println("TX:SYSCONF:修改下次判断交易的基础价格为：", basePrice, " 原基线价格为：", baseLinePrice)
						lastBuyTs = time.Now().UnixMilli()
					}
				} else {
					buyStageDiff += 1
					if buyStageDiff > 100 {
						if basePrice.Sub(price).Abs().Div(basePrice).Cmp(decimal.NewFromFloat32(0.06)) == 1 {
							var oldBasePrice = basePrice
							basePrice = price.Mul(decimal.NewFromInt(95)).Div(decimal.NewFromInt(100))
							logger.Println("TX:SYSCONF:超过多次询价不符合设置限价，重新设定基础限价。原标准价格：", oldBasePrice, " 新标准价格：", basePrice)
							buyStageDiff = 0
						}
					}
				}
			} else {
				const uprise = 105
				sellPrice := basePrice.Mul(decimal.NewFromInt(uprise)).Div(decimal.NewFromInt(100))
				if price.Cmp(sellPrice) != -1 {
					sellHash = sell(targetToken, config.GetConfig().Dex.Vstoken, price, sellPrice)
					if !ongoing() {
						logger.Println("")
						logger.Println("TX:ROUNT:结束完一轮：")
						logger.Println("TX:ROUNT:买入hash：", buyHash)
						logger.Println("TX:ROUNT:卖出hash：", sellHash)
						logger.Println("")
					}
				} else {
					if (time.Now().UnixMilli()-lastPrintSellTs)/1000 > 5*60 {
						logger.Println("TX:SYSCONF:当前价格 ", price, " 基准价格 ", basePrice, " 期望售出 ", sellPrice, " 等待卖出")
					}
					tplRange := int64(config.GetConfig().Sys.Tpl)
					if (time.Now().UnixMilli()-lastBuyTs)/1000 > tplRange*60 {
						log.Println("TX:SYSCONF:超过交易频率阈值未触发售出，启动止损售出")
						sell(targetToken, config.GetConfig().Dex.Vstoken, price, sellPrice)
						if !ongoing() {
							log.Println("TX:SYSCONF:开始启动重置")
							pair := reset()
							if pair != nil {
								targetToken = pair.Token0Address
								basePrice = decimal.NewFromFloat(pair.CurrentPriceUsd)
							}
						}
					}
				}
			}
			round = false
		default:
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func operateSwap() (
	func(targetToken, vsToken string, basePrice, currentPrice decimal.Decimal) string,
	func(targetToken, vsToken string, currentPrice, sellPrice decimal.Decimal) string,
	func() bool,
	func() bool,
	func() *surround.AvePair) {

	var allowBuy = true
	var allowAmount decimal.Decimal
	var ongoing = false

	reset := func() *surround.AvePair {
		pair, err := surround.FilterHot()
		if err != nil {
			log.Print("尝试更换交易标的错误", err)
			return nil
		}
		allowBuy = true
		allowAmount = decimal.NewFromInt(0)
		ongoing = false
		return pair
	}

	buy := func(targetToken, vsToken string, basePrice, currentPrice decimal.Decimal) string {
		allowBuy = false
		logger.Println("TX:BUY_FLAG:执行买入并修改为禁止买入状态", allowBuy)

		var examount, _ = decimal.NewFromString(config.GetConfig().Dex.Pay)

		i := dex.GetIns()
		txhash, estimateAmount, e := i.SwapAndSend(vsToken, targetToken, examount.BigInt(), config.GetConfig().Dex.Slippage)

		if e != nil {
			allowBuy = true
			logger.Println("TX:BUY_ERROR:执行买入交易出错：", e)
			return txhash
		}
		if len(txhash) == 0 {
			allowBuy = true
			logger.Println("TX:BUY_ERROR:执行买入交易出错：", e)
			return txhash
		}
		logger.Println("TX:BUY_FLAG:买入交易签名：", txhash)
		if len(txhash) == 0 {
			logger.Println("TX:BUY_ERROR:买入交易HASH未获取到")
			return txhash
		}

		checkNum := 0
		for {
			result, realAmount, e := i.IsTxSuccess(targetToken, txhash)
			if e != nil {
				allowBuy = true
				logger.Println("TX:BUY_ERROR:交易确认出错：", e)
				break
			}
			if result == "success" {
				allowAmount = realAmount
				break
			}

			if checkNum += 1; checkNum > config.GetConfig().Dex.Timeout {
				logger.Println("TX:BUY_ERROR:交易检查超过阈值，交易失败，重置状态")
				allowBuy = true
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if !allowBuy {
			logger.Println("TX:BUY_SUCCESS:当前价格 ", currentPrice, " 低于基准价格 ", basePrice, " 买入完成, 预估数量：", estimateAmount, " 实际数量：", allowAmount)
			ongoing = true
		}
		return txhash
	}

	sell := func(targetToken, vsToken string, currentPrice, sellPrice decimal.Decimal) string {
		if allowBuy {
			return ""
		}
		i := dex.GetIns()
		txhash, _, e := i.SwapAndSend(targetToken, vsToken, allowAmount.BigInt(), config.GetConfig().Dex.Slippage)
		if e != nil {
			logger.Println("TX:SELL_ERROR:执行卖出交易出错：", e)
			return txhash
		}
		logger.Println("TX:SELL_FLAG:卖出交易签名：", txhash)
		if len(txhash) == 0 {
			logger.Println("TX:SELL_ERROR:卖出交易HASH未获取到")
			return txhash
		}

		checkNum := 0
		var sellSuccess = false
		for {
			result, _, e := i.IsTxSuccess(targetToken, txhash)
			if e != nil {
				logger.Println("TX:SELL_ERROR:卖出交易确认出错：", e)
				break
			}
			if result == "success" {
				allowBuy = true
				allowAmount = decimal.NewFromInt(0)
				sellSuccess = true
				break
			}

			if checkNum += 1; checkNum > config.GetConfig().Dex.Timeout {
				logger.Println("TX:SELL_ERROR:交易检查超过阈值，交易失败，继续等待卖出 ", allowBuy, allowAmount)
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if sellSuccess {
			logger.Println("TX:SELL_SUCCESS:当前价格 ", currentPrice, " 高于售出价格 ", sellPrice, " 卖出完成")
			ongoing = false
		}
		return txhash
	}

	status := func() bool {
		return allowBuy
	}

	ongo := func() bool {
		return ongoing
	}

	return buy, sell, status, ongo, reset
}
