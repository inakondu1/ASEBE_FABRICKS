package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func initDatabase() {

	var err error

	// Connect to database
	db, err = sql.Open("sqlite3", "asebe_fabrics.db")

	if err != nil {
		log.Fatal("Could not open database:", err)
	}

	// Test database connection
	err = db.Ping()

	if err != nil {
		log.Fatal("Could not connect to database:", err)
	}

	// Create products table
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

	// Check if products already exist
	var count int

	err = db.QueryRow(
		"SELECT COUNT(*) FROM products",
	).Scan(&count)

	if err != nil {
		log.Fatal("Could not check products:", err)
	}

	// Add starter fabrics only when database is empty
	if count == 0 {

		_, err = db.Exec(`
			INSERT INTO products
			(name, description, price, quantity, image)
			VALUES
			(
				'Royal Ankara',
				'Beautiful premium Ankara fabric with vibrant African patterns.',
				25000,
				10,
				''
			),
			(
				'Classic Lace',
				'Elegant lace fabric suitable for weddings and special occasions.',
				35000,
				8,
				''
			),
			(
				'African Print',
				'High-quality African print fabric with beautiful traditional designs.',
				22000,
				15,
				''
			),
			(
				'Premium Senator',
				'Premium senator fabric with a smooth and luxurious finish.',
				40000,
				6,
				''
			),
			(
				'Elegant George',
				'Beautiful George fabric perfect for elegant traditional outfits.',
				55000,
				5,
				''
			),
			(
				'Luxury Silk',
				'Soft and luxurious silk fabric for beautiful and stylish outfits.',
				30000,
				12,
				''
			)
		`)

		if err != nil {
			log.Fatal("Could not add starter fabrics:", err)
		}

		log.Println("Starter fabrics added successfully")
	}

	        // Create customers table
        _, err = db.Exec(`
                CREATE TABLE IF NOT EXISTS customers (
                        id INTEGER PRIMARY KEY AUTOINCREMENT,
                        full_name TEXT NOT NULL,
                        phone TEXT NOT NULL UNIQUE,
                        password TEXT NOT NULL,
                        address TEXT,
                        created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
                )
        `)

        if err != nil {
                log.Fatal("Could not create customers table:", err)
        }

        log.Println("Customers table ready")

	log.Println("ASEBE FABRICS database connected")
	log.Println("Products table ready")
}
