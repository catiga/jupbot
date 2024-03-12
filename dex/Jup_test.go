package dex

import (
	"testing"
)

func TestQuote(t *testing.T) {

	/*
		success transaction:
		https://solscan.io/tx/KHtvmFFD8zAWDCdHgQsunSLHBj9JCNKuRih5LxCmCFUtLabU7BQ4va5CMQvAp7boxyHY9rM54vmnmZj5nU6VoyC
		https://explorer.solana.com/tx/2WgLwD7AxfbQfVM3oNQA9UCHb67y2KduNosAKRPtM7ogJXyPrEf2xdNqkHoY7pSR1LVnSVrjPkJtyf7gMXZ5ehT4
	*/

	i := GetIns()
	// v, e := i.Quote("So11111111111111111111111111111111111111112", "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263", big.NewInt(1000000), 100)

	// fmt.Println(v, e)

	i.Price("DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263", "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")

	i.IsTxSuccess("G3q2zUkuxDCXMnhdBPujjPHPw9UTMDbXqzcc2UHM3jiy", "W2979nva1y6o3UTs3LtpF7s42fkBXUdWB45X5CWih8vnxsFSSUFuWSmcnWXtrygYgN3Vc8ksgq6sLHyUHfNXAaJ")

	// txhash, amount, err := i.SwapAndSend("So11111111111111111111111111111111111111112", "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263", big.NewInt(1000000), 100)

	// if err != nil {
	// 	log.Fatal(err, amount)
	// }

	// log.Println(txhash)

	// status, err := i.GetMemTx([]string{txhash})
	// log.Println(status, err)
}
