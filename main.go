package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"shelfrobot/config"
	_ "shelfrobot/database"
	"shelfrobot/surround"
	"strings"
	"sync"

	"shelfrobot/cmd"
	"shelfrobot/sys"
)

var logger = sys.Logger
var wg sync.WaitGroup

func handleConnection(conn net.Conn) {
	fmt.Println("New client connected:", conn.RemoteAddr().String())
	defer conn.Close()

	reader := bufio.NewReader(conn)
	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			logger.Println("Client disconnected:", conn.RemoteAddr().String())
			break
		}

		message = message[:len(message)-1] // 去除换行符
		message = strings.TrimRight(message, "\r\n")
		logger.Printf("Received command from %s: %s\n", conn.RemoteAddr().String(), message)
		if len(message) > 0 {
			if message == "exit" {
				logger.Println("Exiting Connection.")
				_ = conn.Close()
			} else {
				router := cmd.NewCommandRouter()
				router.ParseCommands(message)
				router.Route(conn, message)
			}
		}
	}
}

func main() {
	defer sys.LogFile.Close()

	portInt := flag.Int("p", 9501, "The Console Port")
	flag.Parse()

	listenAddr := fmt.Sprintf("0.0.0.0:%d", *portInt)
	server, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logger.Fatal("system error:", err)
	}
	defer server.Close()

	logger.Printf("Bot starting, and the console port is %d", *portInt)
	for _, v := range config.GetDexes() {
		go surround.WatchDex(v)
	}

	for {
		conn, err := server.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		wg.Add(1)
		go handleConnection(conn)
	}
}
