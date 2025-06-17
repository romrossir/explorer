package main

import (
	"component-service/api"
	"component-service/cache" // Added
	"component-service/db"
	"component-service/store" // Added
	"log"
	"net/http"
	"os"
)

func main() {
	// Load environment variables or configuration if any
	// Example: godotenv.Load() if using .env file

	// Initialize database connection
	db.InitDB() // This function should handle database connection details and pooling
	log.Println("Database initialized.")

	// Initialize the component cache
	// The ComponentStore is needed by InitGlobalCache to fetch initial data.
	cs := &store.ComponentStore{} // Create an instance that satisfies store.ComponentStoreInterface
	if err := cache.InitGlobalCache(cs); err != nil {
		// If cache initialization fails, it might be critical for the application.
		// Depending on requirements, you might allow the app to run with a disabled cache
		// or treat this as a fatal error. Here, we treat it as fatal.
		log.Fatalf("Failed to initialize component cache: %v", err)
	}
	log.Println("Component cache initialized.")

	// Setup HTTP routing
	// ComponentsHandler will use the store (and implicitly the cache through store methods)
	http.HandleFunc("/components/", api.ComponentsHandler) // Handles /components/ and /components/{id}

	// Optional: Root handler for service health check or info
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("Component service is running."))
	})

	// Start the HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port if not specified
	}
	log.Printf("Server starting on port %s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
