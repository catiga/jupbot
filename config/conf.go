package config

import (
	"log"
	"os"

	system "shelfrobot/sys"

	"github.com/spf13/viper"
)

type Database struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

type Chain struct {
	Rpc    string `yaml:"rpc"`
	Name   string `yaml:"name"`
	Access string `yaml:"key"`
}

type Log struct {
	Path string `yaml:"path"`
	Name string `yaml:"name"`
}

type Wallet struct {
	Pk string `yaml:"pk"`
}

type Dex struct {
	Pay      string `yaml:"pk"`
	Slippage int    `yaml:"slippage"`
	Increase int
	Timeout  int
	Vstoken  string
}

type Sys struct {
	Avekey  string
	Aveauth string
	Tpl     int
}

type Config struct {
	Database Database
	Chain    Chain
	Log      Log
	Wallet   Wallet
	Dex      Dex
	Sys      Sys
}

var addChain, removeChain, getChain = chainFactory()
var systemConfig = &Config{}

func GetConfig() Config {
	return *systemConfig
}

func init() {
	confFilePath := "../config/dev.yml"

	if configFilePathFromEnv := os.Getenv("DALINK_GO_CONFIG_PATH"); configFilePathFromEnv != "" {
		confFilePath = configFilePathFromEnv
	}

	viper.SetConfigFile(confFilePath)

	viper.SetConfigType("yml")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("无法读取配置文件：%s", err)
	}

	err = viper.Unmarshal(systemConfig)
	if err != nil {
		log.Fatalf("无法解析配置：%s", err)
	}

	system.LogFile, err = os.OpenFile(systemConfig.Log.Path+systemConfig.Log.Name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	// defer logFile.Close()

	system.Logger = log.New(system.LogFile, "prefix: ", log.LstdFlags)

	addChain(systemConfig.Chain)

	system.Logger.Printf("initing default %s chain config", "Solana")
}

func GetSqlLite() Database {
	return systemConfig.Database
}

func Add(chainName, chainRpc string, chainId int) bool {
	log.Printf("Initing %s config...", chainName)
	return addChain(Chain{
		Rpc:    chainRpc,
		Name:   chainName,
		Access: "",
	})
}

func Remove(chainName string) {
	removeChain(chainName)
}

func Get(chainName string) Chain {
	return getChain(chainName)
}

func chainFactory() (func(Chain) bool, func(string), func(string) Chain) {
	chainList := make(map[string]Chain)

	addChain := func(c Chain) bool {
		if len(c.Name) == 0 {
			return false
		}
		chainList[c.Name] = c
		return true
	}

	removeChain := func(name string) {
		_, exist := chainList[name]
		if exist {
			delete(chainList, name)
		}
	}

	getChain := func(name string) Chain {
		return chainList[name]
	}

	return addChain, removeChain, getChain
}
