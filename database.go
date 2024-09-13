package main

import (
    "database/sql"
    "fmt"
    "log"
    "time"
    _ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

func initDatabase(config *Config) {
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		config.Database.User,
		config.Database.Password,
		config.Database.Host,
		config.Database.Port,
		config.Database.Name)

	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour)

	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	err = createTables()
	if err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}

	log.Println("Database initialized successfully")
}

func createTables() error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS Files (
		Id VARCHAR(36) PRIMARY KEY,
		FileName VARCHAR(255) NOT NULL,
		FileType VARCHAR(100) NOT NULL,
		IV BLOB NOT NULL,
		EncryptionKey BLOB NOT NULL,
		CreatedAt TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`)
	if err != nil {
		log.Printf("Error creating Files table: %v", err)
		return err
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS ShortLinks (
		ShortCode VARCHAR(10) PRIMARY KEY,
		OriginalUrl TEXT NOT NULL,
		CreatedAt TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`)
	if err != nil {
		return err
	}

	// 检查表是否成功创建
	tables := []string{"Files", "ShortLinks"}
	for _, table := range tables {
		var tableName string
		query := fmt.Sprintf("SHOW TABLES LIKE '%s'", table)
		err := db.QueryRow(query).Scan(&tableName)
		if err != nil {
			if err == sql.ErrNoRows {
				log.Printf("Table %s does not exist", table)
			} else {
				log.Printf("Error checking if table %s exists: %v", table, err)
				return err
			}
		} else {
			log.Printf("Table %s exists", table)
		}
	}

	return nil
}