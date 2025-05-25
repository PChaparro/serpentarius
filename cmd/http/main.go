package main

import (
	"log"

	"github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure/http"
)

func main() {
	router := http.RegisterRoutes()

	if err := router.Run(":3000"); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
