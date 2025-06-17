package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq" // PostgreSQL driver
)

var DB *sql.DB

// InitDB initializes the database connection.
// It expects database connection details from environment variables:
// DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE
func InitDB() {
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbSSLMode := os.Getenv("DB_SSLMODE")

	if dbHost == "" || dbPort == "" || dbUser == "" || dbName == "" {
		log.Fatal("Database environment variables (DB_HOST, DB_PORT, DB_USER, DB_NAME) are required.")
	}

	if dbSSLMode == "" {
		dbSSLMode = "disable" // Default SSL mode
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		dbHost, dbPort, dbUser, dbPassword, dbName, dbSSLMode)

	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Error opening database connection: %v", err)
	}

	err = DB.Ping()
	if err != nil {
		log.Fatalf("Error pinging database: %v. Please ensure PostgreSQL is running and accessible, and the connection details are correct.", err)
	}

	log.Println("Successfully connected to the PostgreSQL database!")

	// Optional: You can execute the schema.sql here if you want to ensure tables are created
	// This is useful for development but might be handled by migrations in production.
	// Example:
	// schemaBytes, err := os.ReadFile("db/schema.sql")
	// if err != nil {
	//     log.Fatalf("Error reading schema.sql: %v", err)
	// }
	// _, err = DB.Exec(string(schemaBytes))
	// if err != nil {
	//     log.Fatalf("Error executing schema.sql: %v", err)
	// }
	// log.Println("Database schema applied successfully.")
}

// GetDB returns the active database connection.
func GetDB() *sql.DB {
	if DB == nil {
		// This case should ideally not happen if InitDB is called at application start.
		// Consider how to handle this based on your application's lifecycle.
		// For now, we'll log a fatal error.
		log.Fatal("Database connection is not initialized. Call InitDB first.")
	}
	return DB
}
