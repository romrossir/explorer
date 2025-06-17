package main

import (
	"component-service/api"
	"component-service/db"
	"log"
	"net/http"
	"os"
)

func main() {
	// Load environment variables (e.g., from a .env file or system environment)
	// For simplicity, we assume they are set. In a real app, use something like godotenv.
	// Example:
	// err := godotenv.Load()
	// if err != nil {
	//     log.Println("Warning: .env file not found, relying on system environment variables.")
	// }


	// Initialize database connection
	// DB connection details are expected as environment variables (see db/db.go)
	// Ensure you have set: DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME
	// e.g., export DB_HOST=localhost DB_PORT=5432 DB_USER=youruser DB_PASSWORD=yourpass DB_NAME=components_db
	db.InitDB()
	log.Println("Database initialized.")

	// Setup HTTP routing
	// All requests to /components/* will be handled by ComponentsHandler
	http.HandleFunc("/components/", api.ComponentsHandler)
	// A root handler for basic check
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w,r)
			return
		}
		w.Write([]byte("Component service is running."))
	})


	// Determine port for HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Starting server on port %s...", port)
	// Start the server
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Could not start server: %s\n", err.Error())
	}
}
