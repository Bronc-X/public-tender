package db

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
	"os"
)

var DB *sqlx.DB

func InitDB(dbPath string) (*sqlx.DB, error) {
	if dbPath == "" {
		dbPath = "../backend/data/app.db" // Default path for development
	}

	fmt.Printf("Connecting to SQLite database: %s\n", dbPath)
	
	// Check if file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("database file does not exist: %s", dbPath)
	}

	db, err := sqlx.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	// 限制为单连接，完美解决多个文件并发上传时出现的 SQLite database is locked 报错
	db.SetMaxOpenConns(1)

	// 全局 Unsafe：表结构演进后 SELECT * 多出的列不会导致扫描失败（与逐处 .Unsafe() 叠加无害）
	unsafeDB := db.Unsafe()
	DB = unsafeDB
	return unsafeDB, nil
}
