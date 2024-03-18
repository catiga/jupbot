package cmd

import (
	"fmt"
	"net"
	"shelfrobot/database"
	"strconv"
	"strings"
)

type CommandHandler func(conn net.Conn, args []string)

type CommandRouter struct {
	handlers map[string]CommandHandler
}

func NewCommandRouter() *CommandRouter {
	return &CommandRouter{handlers: make(map[string]CommandHandler)}
}

func (r *CommandRouter) register(command string, handler CommandHandler) {
	r.handlers[command] = handler
}

/*
showtxs [10]
showcatxs [ca] [10]
showcabuytxs [ca] [10]
showcaselltxs [ca] [10]
gettx [txhash]
*/
func (r *CommandRouter) ParseCommands(command string) {
	if strings.HasPrefix(command, "showtxs") {
		r.register("showtxs", showTxHandler)
	} else if strings.HasPrefix(command, "showcatxs") {
		r.register("showcatxs", showCaTxsHandler)
	} else if strings.HasPrefix(command, "gettx") {
		r.register("gettx", getTxHandler)
	}
}

func (r *CommandRouter) Route(conn net.Conn, command string) {
	args := strings.Fields(command) // 使用Fields处理多余空格
	if len(args) == 0 {
		return // 空命令
	}

	cmd := args[0]
	handler, ok := r.handlers[cmd]
	if !ok {
		// 如果没有找到命令，可以发送错误信息给客户端
		conn.Write([]byte("Unknown command\n"))
		return
	}

	// 调用处理函数，传入参数
	handler(conn, args[1:]) // 传递除命令外的参数
}

func showTxHandler(conn net.Conn, args []string) {
	count := int(10)
	if len(args) > 0 {
		cmdcount, err := strconv.ParseInt(args[0], 10, 64)
		if err == nil && cmdcount > 0 && cmdcount <= 100 {
			count = int(cmdcount)
		}
	}

	var result []database.Transaction
	database.GetDB().Model(&database.Transaction{}).Limit(count).Find(&result)

	var sb strings.Builder
	sb.WriteString("TX, Type, CA, Name, Amount, TxPrice, BasePrice, Time, Status\n")
	if len(result) > 0 {
		for _, tx := range result {
			txhash := tx.Txhash
			if len(txhash) == 0 {
				txhash = "***********"
			}
			sb.WriteString(fmt.Sprintf("%s, %s, %s, %s, %s, %s, %s, %s, %d \n", txhash, tx.Type, tx.TokenCa, tx.TokenName, tx.TokenAmount, tx.TxPrice, tx.BasePrice, tx.TxTime, tx.Status))
		}
	}

	conn.Write([]byte(sb.String()))
}

func showCaTxsHandler(conn net.Conn, args []string) {
	var sb strings.Builder
	sb.WriteString("TX, Type, CA, Name, Amount, TxPrice, BasePrice, Time, Status \n")

	if len(args) == 2 {
		ca := args[0]
		count := int(10)
		if len(args) > 0 {
			cmdcount, err := strconv.ParseInt(args[1], 10, 64)
			if err == nil && cmdcount > 0 && cmdcount <= 100 {
				count = int(cmdcount)
			}
		}

		var result []database.Transaction
		database.GetDB().Model(&database.Transaction{}).Where("token_ca=?", ca).Limit(count).Find(&result)

		if len(result) > 0 {
			for _, tx := range result {
				txhash := tx.Txhash
				if len(txhash) == 0 {
					txhash = "***********"
				}
				sb.WriteString(fmt.Sprintf("%s, %s, %s, %s, %s, %s, %s, %s, %d \n", txhash, tx.Type, tx.TokenCa, tx.TokenName, tx.TokenAmount, tx.TxPrice, tx.BasePrice, tx.TxTime, tx.Status))
			}
		}
	}
	conn.Write([]byte(sb.String()))
}

func getTxHandler(conn net.Conn, args []string) {
	var sb strings.Builder
	sb.WriteString("TX, Type, CA, Name, Amount, TxPrice, BasePrice, Time, Status\n")

	if len(args) == 1 {
		hash := args[0]

		var result []database.Transaction
		database.GetDB().Model(&database.Transaction{}).Where("txhash=?", hash).Limit(10).Find(&result)

		if len(result) > 0 {
			for _, tx := range result {
				txhash := tx.Txhash
				if len(txhash) == 0 {
					txhash = "***********"
				}
				sb.WriteString(fmt.Sprintf("%s, %s, %s, %s, %s, %s, %s, %s, %d \n", txhash, tx.Type, tx.TokenCa, tx.TokenName, tx.TokenAmount, tx.TxPrice, tx.BasePrice, tx.TxTime, tx.Status))
			}
		}
	}
	conn.Write([]byte(sb.String()))
}
