package main

import (
	"fmt"
	"log"

	"github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure"
	"github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure/http"
)

func main() {
	router := http.RegisterRoutes()

	port := infrastructure.GetEnvironment().Port
	if err := router.Run(fmt.Sprintf(":%d", port)); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
