package db

import (
	"database/sql"
	"log"
	"tenders/internal/config"

	_ "github.com/lib/pq"
)

func NewPostgresDB(cfg *config.PostgresConfig) (*sql.DB, error) {
	log.Println("Connecting db on: ", cfg.Conn)
	db, err := sql.Open("postgres", cfg.Conn)

	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}
