package database

import (
	"database/sql"
	"log"
	"os"

	"shelfrobot/config"

	_ "github.com/mattn/go-sqlite3" // SQLite驱动程序
)

var sqldb *sql.DB

func initializeDB(filePath string) (*sql.DB, error) {
	// 检查数据库文件是否存在
	firstTimeSetup := false
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		firstTimeSetup = true
	}

	log.Println("db path:", filePath)

	// 打开数据库连接（如果文件不存在，SQLite会自动创建）
	db, err := sql.Open("sqlite3", filePath)
	if err != nil {
		return nil, err
	}

	// 如果是首次设置，创建表
	if firstTimeSetup {
		sqlStmt := `
        CREATE TABLE IF NOT EXISTS foo (
            id INTEGER NOT NULL PRIMARY KEY,
            name TEXT
        );
        `
		_, err = db.Exec(sqlStmt)
		if err != nil {
			return nil, err
		}
		log.Println("Database and tables created.")
	}

	return db, nil
}

func init() {
	sqlConfig := config.GetSqlLite()
	sqldb, err := initializeDB(sqlConfig.Path + sqlConfig.Name)

	if err != nil {
		log.Fatal(err)
	}
	defer sqldb.Close()

	// 从此处开始你的数据库操作，例如插入数据、查询等
	// ...
}

func GetDB() *sql.DB {
	return sqldb
}
