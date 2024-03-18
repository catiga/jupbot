package database

import (
	"log"
	"os"
	"time"

	"shelfrobot/config"
	"shelfrobot/sys"

	_ "github.com/mattn/go-sqlite3" // SQLite驱动程序
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var sqldb *gorm.DB
var logger = sys.Logger

func initializeDB(filePath string) (*gorm.DB, error) {
	firstTimeSetup := false
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		firstTimeSetup = true
	}

	logger.Println("db path:", filePath)

	db, err := gorm.Open(sqlite.Open(filePath), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database", err)
	}

	// 如果是首次设置，创建表
	if firstTimeSetup {
		db.AutoMigrate(&Transaction{})
		logger.Println("Database and tables created.")
	}

	return db, nil
}

func init() {
	sqlConfig := config.GetSqlLite()
	tmpdb, err := initializeDB(sqlConfig.Path + sqlConfig.Name)

	if err != nil {
		logger.Fatal(err)
	}
	sqldb = tmpdb
}

func GetDB() *gorm.DB {
	return sqldb
}

func BuyTx(tx *Transaction) {
	loc, _ := time.LoadLocation("Asia/Shanghai") // 加载UTC+8时区
	now := time.Now().In(loc)
	formattedTime := now.Format("2006-01-02 15:04:05")

	tx.TxTime = formattedTime
	tx.Type = "buy"
	tx.Chain = "solana"
	tx.TxChannel = "jup"

	sqldb.Create(tx)
}

func SellTx(tx *Transaction) {
	loc, _ := time.LoadLocation("Asia/Shanghai") // 加载UTC+8时区
	now := time.Now().In(loc)
	formattedTime := now.Format("2006-01-02 15:04:05")

	tx.TxTime = formattedTime
	tx.Type = "sell"
	tx.Chain = "solana"
	tx.TxChannel = "jup"

	sqldb.Create(tx)
}
