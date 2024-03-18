package surround

import (
	"shelfrobot/config"
	"shelfrobot/database"
	"shelfrobot/dex"
	"sync"
	"time"

	"shelfrobot/sys"

	"github.com/shopspring/decimal"
)

var logger = sys.Logger

func OperateSwap(to config.TargetToken) (
	func(basePrice, currentPrice decimal.Decimal) string,
	func(currentPrice, sellPrice decimal.Decimal) string,
	func() bool,
	func() bool,
	func() *AvePair) {

	var allowBuy = true
	var allowAmount decimal.Decimal
	var ongoing = false
	var tokenobject = to
	var targetToken = tokenobject.Ca
	var vsToken = config.GetConfig().Dex.Vstoken

	reset := func() *AvePair {
		pair, err := FilterHot()
		if err != nil {
			logger.Print("尝试更换交易标的错误", err)
			return nil
		}
		allowBuy = true
		allowAmount = decimal.NewFromInt(0)
		ongoing = false
		return pair
	}

	buy := func(basePrice, currentPrice decimal.Decimal) string {
		allowBuy = false
		logger.Println("TX:BUY_FLAG:执行买入并修改为禁止买入状态", allowBuy)

		var examount, _ = decimal.NewFromString(config.GetConfig().Dex.Pay)

		i := dex.GetIns()
		txhash, estimateAmount, e := i.SwapAndSend(vsToken, targetToken, examount.BigInt(), config.GetConfig().Dex.Slippage)

		var txstatus = 0 // 0:ing 1:success 2:error
		if e != nil {
			allowBuy = true
			logger.Println("TX:BUY_ERROR:执行买入交易出错：", e)
			// return txhash
			txstatus = 2
		}
		if len(txhash) == 0 {
			allowBuy = true
			logger.Println("TX:BUY_ERROR:执行买入交易出错：", e)
			// return txhash
			txstatus = 2
		}
		logger.Println("TX:BUY_FLAG:买入交易签名：", txhash)
		if len(txhash) == 0 {
			logger.Println("TX:BUY_ERROR:买入交易HASH未获取到")
			// return txhash
			txstatus = 2
		}

		if txstatus == 2 {
			//存入数据
			database.BuyTx(&database.Transaction{
				Txhash:      txhash,
				TokenCa:     tokenobject.Ca,
				TokenName:   tokenobject.Name,
				TokenAmount: "0",
				EstAmount:   estimateAmount.String(),
				TxPrice:     currentPrice.String(),
				BasePrice:   basePrice.String(),
				Status:      txstatus,
			})
			return txhash
		}

		checkNum := 0
		for {
			if checkNum += 1; checkNum > config.GetConfig().Dex.Timeout {
				logger.Println("TX:BUY_ERROR:交易检查超过阈值，重置状态, 重置次数:", checkNum)
				allowBuy = true
				break
			}

			e := i.GetMemTx(txhash)
			if e != nil {
				if e.Error() == "dropped" {
					//交易失败
					logger.Println("TX:BUY_ERROR:交易失败，重置状态")
					allowBuy = true
					break
				} else {
					continue
				}
			}
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

			time.Sleep(50 * time.Millisecond)
		}
		if !allowBuy {
			logger.Println("TX:BUY_SUCCESS:当前价格 ", currentPrice, " 低于基准价格 ", basePrice, " 买入完成, 预估数量：", estimateAmount, " 实际数量：", allowAmount)
			ongoing = true

			database.BuyTx(&database.Transaction{
				Txhash:      txhash,
				TokenCa:     tokenobject.Ca,
				TokenName:   tokenobject.Name,
				TokenAmount: allowAmount.String(),
				EstAmount:   estimateAmount.String(),
				TxPrice:     currentPrice.String(),
				BasePrice:   basePrice.String(),
				Status:      1,
			})
		} else {
			database.BuyTx(&database.Transaction{
				Txhash:      txhash,
				TokenCa:     tokenobject.Ca,
				TokenName:   tokenobject.Name,
				TokenAmount: "0",
				EstAmount:   estimateAmount.String(),
				TxPrice:     currentPrice.String(),
				BasePrice:   basePrice.String(),
				Status:      2,
			})
		}
		return txhash
	}

	sell := func(currentPrice, sellPrice decimal.Decimal) string {
		if allowBuy {
			return ""
		}
		i := dex.GetIns()
		txhash, _, e := i.SwapAndSend(targetToken, vsToken, allowAmount.BigInt(), config.GetConfig().Dex.Slippage)
		var txstatus = 0
		if e != nil {
			logger.Println("TX:SELL_ERROR:执行卖出交易出错：", e)
			// return txhash
			txstatus = 2
		}
		logger.Println("TX:SELL_FLAG:卖出交易签名：", txhash)
		if len(txhash) == 0 {
			logger.Println("TX:SELL_ERROR:卖出交易HASH未获取到")
			// return txhash
			txstatus = 2
		}

		if txstatus == 2 {
			//存入数据
			database.SellTx(&database.Transaction{
				Txhash:      txhash,
				TokenCa:     tokenobject.Ca,
				TokenName:   tokenobject.Name,
				TokenAmount: allowAmount.String(),
				EstAmount:   allowAmount.String(),
				TxPrice:     sellPrice.String(),
				BasePrice:   currentPrice.String(),
				Status:      txstatus,
			})
			return txhash
		}

		checkNum := 0
		var sellSuccess = false
		for {
			if checkNum += 1; checkNum > config.GetConfig().Dex.Timeout {
				logger.Println("TX:SELL_ERROR:交易检查超过阈值，交易失败，继续等待卖出 ", allowBuy, allowAmount)
				break
			}
			e := i.GetMemTx(txhash)
			if e != nil {
				if e.Error() == "dropped" {
					//交易失败
					logger.Println("TX:SELL_ERROR:交易失败，重置状态")
					break
				} else {
					continue
				}
			}
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
			time.Sleep(50 * time.Millisecond)
		}
		if sellSuccess {
			logger.Println("TX:SELL_SUCCESS:当前价格 ", currentPrice, " 高于售出价格 ", sellPrice, " 卖出完成")
			ongoing = false

			database.SellTx(&database.Transaction{
				Txhash:      txhash,
				TokenCa:     tokenobject.Ca,
				TokenName:   tokenobject.Name,
				TokenAmount: allowAmount.String(),
				EstAmount:   allowAmount.String(),
				TxPrice:     sellPrice.String(),
				BasePrice:   currentPrice.String(),
				Status:      1,
			})
		} else {
			database.SellTx(&database.Transaction{
				Txhash:      txhash,
				TokenCa:     tokenobject.Ca,
				TokenName:   tokenobject.Name,
				TokenAmount: allowAmount.String(),
				EstAmount:   allowAmount.String(),
				TxPrice:     sellPrice.String(),
				BasePrice:   currentPrice.String(),
				Status:      2,
			})
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

const usdc = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

func MakeWatch(tokenobject config.TargetToken) {

	var buy, sell, status, ongoing, reset = OperateSwap(tokenobject)

	targetToken := tokenobject.Ca
	basePrice := decimal.NewFromFloat(tokenobject.Initprice)
	baseLinePrice := basePrice

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
					buyHash = buy(basePrice, price)
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
				var uprise = int64(config.GetConfig().Dex.Increase)
				sellPrice := basePrice.Mul(decimal.NewFromInt(uprise)).Div(decimal.NewFromInt(100))
				if price.Cmp(sellPrice) != -1 {
					sellHash = sell(price, sellPrice)
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
					if tplRange > 0 && (time.Now().UnixMilli()-lastBuyTs)/1000 > tplRange*60 {
						logger.Println("TX:SYSCONF:超过交易频率阈值未触发售出，启动止损售出")
						sell(price, sellPrice)
						if !ongoing() {
							logger.Println("TX:SYSCONF:开始启动重置")
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
