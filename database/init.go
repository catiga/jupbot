package database

type Transaction struct {
	Id          uint64 `gorm:"primaryKey"`                                               // 对应 INTEGER PRIMARY KEY
	Txhash      string `gorm:"not null"`                                                 // 对应 TEXT NOT NULL
	Type        string `gorm:"type:varchar(100);check:type IN ('buy', 'sell');not null"` // 对应 TEXT CHECK(Type IN ('buy', 'sell'))
	Chain       string // 对应 TEXT NOT NULL
	TxChannel   string `gorm:"not null"` // 对应 TEXT NOT NULL
	TokenCa     string `gorm:"not null"` // 对应 TEXT NOT NULL
	TokenName   string `gorm:"not null"`
	TokenAmount string `gorm:"not null"` // 对应 TEXT NOT NULL
	EstAmount   string `gorm:"not null"`
	TxPrice     string `gorm:"not null"` // 对应 TEXT NOT NULL
	BasePrice   string
	TxTime      string `gorm:"not null"`
	Status      int    `gorm:"not null"`
}

// const sqlStmt = `
// CREATE TABLE IF NOT EXISTS transactions (
//     Id       INTEGER PRIMARY KEY, -- 使用 INTEGER 来匹配 Go 中的 uint64
//     Txhash   TEXT NOT NULL,
//     Type     TEXT CHECK(Type IN ('buy', 'sell')), -- 确保 Type 只能是 'buy' 或 'sell'
//     ChainId  TEXT NOT NULL,
//     TxChan   TEXT NOT NULL,
//     TokenCa  TEXT NOT NULL,
//     TxAmount TEXT NOT NULL,
// 	EstAmount TEXT NOT NULL,
//     TxPrice  TEXT NOT NULL,
//     TxTime  TEXT NOT NULL,
// 	INTEGER NOT NULL
// );
// `
