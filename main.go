package main

import (
	"flag"
	"shelfrobot/config"
	_ "shelfrobot/database"
	"shelfrobot/dex"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"shelfrobot/sys"
)

const usdc = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

var buy, sell, status, ongoing = operateSwap()
var logger = sys.Logger

func main() {
	defer sys.LogFile.Close()
	targetTokenPtr := flag.String("o", "", "The target token")
	vsTokenPtr := flag.String("s", "", "The versus token")
	basePriceStr := flag.String("p", "", "The base price to U")

	flag.Parse()

	if *targetTokenPtr == "" || *vsTokenPtr == "" {
		logger.Fatal("must provide both target token and versus token")
	}

	baseLinePrice, err := decimal.NewFromString(*basePriceStr)
	if err != nil {
		logger.Fatal("please refer a regula baseline price")
	}
	if baseLinePrice.Cmp(decimal.NewFromInt(0)) != 1 {
		logger.Fatal("please refer a regula baseline price")
	}
	basePrice := baseLinePrice

	pChan := make(chan decimal.Decimal)

	logger.Println("目标token ", *targetTokenPtr, " 基准价格 ", *basePriceStr)
	var round = false
	var mutex sync.Mutex

	go func() {
		for {
			if !round {
				mutex.Lock()
				round = true
				mutex.Unlock()
				i := dex.GetIns()
				p, err := i.Price(*targetTokenPtr, usdc)

				if err == nil {
					pChan <- p
				}
			}
		}
	}()

	var buyHash, sellHash string
	var buyStageDiff = 0
	for {
		select {
		case price := <-pChan:
			// if price.Sub(lastPrice).Abs().Div(price).Cmp(rate) != -1 {
			// 	log.Println("当前价格 ", price, " 基准价格 ", basePrice, " 浮动超过 3%")
			// }
			if !ongoing() {
				logger.Println("当前价格 ", price, " 基准价格 ", basePrice, " 浮动超过 ", price.Sub(basePrice).Abs().Div(price), "%", " 交易控制状态：", status())
			}

			if status() {
				if price.Cmp(basePrice) != 1 {
					// log.Println("当前价格 ", price, " 低于基准价格 ", basePrice, " 执行买入")
					buyHash = buy(*targetTokenPtr, *vsTokenPtr, basePrice, price)
					if ongoing() {
						//说明买入完成, 修改基础基准交易价格
						basePrice = price
						logger.Println("修改下次判断交易的基础价格为：", basePrice, " 原基线价格为：", baseLinePrice)
					}
				} else {
					buyStageDiff += 1
					if buyStageDiff > 100 {
						if basePrice.Sub(price).Abs().Div(basePrice).Cmp(decimal.NewFromFloat32(0.06)) == 1 {
							var oldBasePrice = basePrice
							basePrice = price.Mul(decimal.NewFromInt(95)).Div(decimal.NewFromInt(100))
							logger.Println("超过多次询价不符合设置限价，重新设定基础限价。原标准价格：", oldBasePrice, " 新标准价格：", basePrice)
							buyStageDiff = 0
						}
					}
				}
				// else {
				// 	log.Println("价格不符合条件", price, basePrice)
				// }
			} else {
				const uprise = 102
				sellPrice := basePrice.Mul(decimal.NewFromInt(uprise)).Div(decimal.NewFromInt(100))
				// logger.Println("上次购买价格：", basePrice, " 拟售出价格 ", sellPrice, " 当前价格 ", price)
				if price.Cmp(sellPrice) != -1 {
					// log.Println("当前价格 ", price, " 高于售出价格 ", sellPrice, " 执行卖出")
					sellHash = sell(*targetTokenPtr, *vsTokenPtr, price, sellPrice)
					if !ongoing() {
						logger.Println("")
						logger.Println("结束完一轮：")
						logger.Println("买入hash：", buyHash)
						logger.Println("卖出hash：", sellHash)
						logger.Println("")
					}

					// if !ongoing() {
					// 	//说明卖出完成
					// 	basePrice = sellPrice
					// }
				}
				// else {
				// 	log.Println("当前价格 ", price, " 拟售出价格 ", sellPrice, " 等待卖出")
				// }
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
	func() bool) {

	var allowBuy = true
	var allowAmount decimal.Decimal
	var ongoing = false

	buy := func(targetToken, vsToken string, basePrice, currentPrice decimal.Decimal) string {
		allowBuy = false
		logger.Println("执行买入并修改为禁止买入状态", allowBuy)

		var examount, _ = decimal.NewFromString(config.GetConfig().Wallet.Pay)

		i := dex.GetIns()
		txhash, estimateAmount, e := i.SwapAndSend(vsToken, targetToken, examount.BigInt(), 100)

		if e != nil {
			allowBuy = true
			logger.Println("执行买入交易出错：", e)
			return txhash
		}
		if len(txhash) == 0 {
			allowBuy = true
			logger.Println("执行买入交易出错：", e)
			return txhash
		}
		logger.Println("买入交易签名：", txhash)

		checkNum := 0
		// var realAmount = decimal.NewFromInt(0)
		for {
			result, realAmount, e := i.IsTxSuccess(targetToken, txhash)
			if e != nil {
				allowBuy = true
				logger.Println("交易确认出错：", e)
				break
			}
			if result == "success" {
				allowAmount = realAmount
				break
			}

			if checkNum += 1; checkNum > 90 {
				logger.Println("交易检查超过阈值，交易失败，重置状态")
				allowBuy = true
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		if !allowBuy {
			logger.Println("当前价格 ", currentPrice, " 低于基准价格 ", basePrice, " 买入完成, 预估数量：", estimateAmount, " 实际数量：", allowAmount)
			ongoing = true
		}
		return txhash
	}

	sell := func(targetToken, vsToken string, currentPrice, sellPrice decimal.Decimal) string {
		if allowBuy {
			return ""
		}
		i := dex.GetIns()
		txhash, _, e := i.SwapAndSend(targetToken, vsToken, allowAmount.BigInt(), 100)
		if e != nil {
			logger.Println("执行卖出交易出错：", e)
			return txhash
		}
		logger.Println("卖出交易签名：", txhash)
		if len(txhash) == 0 {
			logger.Println("卖出交易hash未获取到")
			return txhash
		}

		checkNum := 0
		var sellSuccess = false
		for {
			result, _, e := i.IsTxSuccess(targetToken, txhash)
			if e != nil {
				// allowBuy = true
				logger.Println("卖出交易确认出错：", e)
				break
			}
			if result == "success" {
				allowBuy = true
				allowAmount = decimal.NewFromInt(0)
				sellSuccess = true
				break
			}

			if checkNum += 1; checkNum > 90 {
				logger.Println("交易检查超过阈值，交易失败，继续等待卖出 ", allowBuy, allowAmount)
				// allowBuy = true
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		if sellSuccess {
			logger.Println("当前价格 ", currentPrice, " 高于售出价格 ", sellPrice, " 卖出完成")
			ongoing = false
		}
		return txhash

		// if !ongoing {
		// 	logger.Fatal("执行完一个循环，退出程序，检查数据")
		// }
	}

	status := func() bool {
		return allowBuy
	}

	ongo := func() bool {
		return ongoing
	}

	return buy, sell, status, ongo
}
