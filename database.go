package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func initDatabase() {

	var err error

	db, err = sql.Open("sqlite3", "asebe_fabrics.db")

	if err != nil {
		log.Fatal("Could not open database:", err)
	}

	err = db.Ping()

	if err != nil {
		log.Fatal("Could not connect to database:", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS products (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			price REAL NOT NULL,
			quantity INTEGER NOT NULL,
			image TEXT
		)
	`)

	if err != nil {
		log.Fatal("Could not create products table:", err)
	}

	log.Println("ASEBE FABRICS database connected")
	log.Println("Products table ready")
}
