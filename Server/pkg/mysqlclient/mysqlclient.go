package mysqlclient

import (
	"fmt"
	"log"
	"time"

	"github.com/dprince-03/Bibliotheca/internal/config"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func ConnectMySqlClient(cfg *config.Config) (*sqlx.DB, error) {
	dataSourceName := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=Local",
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
	)

	db, err := sqlx.Connect("mysql", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("Error connecting to Database : %v", err)
	}

	db.SetMaxOpenConns(25)                 // max open connections to DB
	db.SetMaxIdleConns(10)                 // max idle connections kept in pool
	db.SetConnMaxLifetime(5 * time.Minute) // recycle connections every 5 min
	db.SetConnMaxIdleTime(2 * time.Minute) // close idle connections after 2 min

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("Error pinging database: %v", err)
	}

	log.Println("Database connected successfully !!!")
	return db, nil
}
