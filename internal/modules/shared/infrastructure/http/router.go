package http

import (
	"log"

	pdfHttp "github.com/PChaparro/serpentarius/internal/modules/pdf/infrastructure/http"
	"github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure"
	sharedMiddlewares "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure/http/middlewares"
	"github.com/gin-gonic/gin"
)

// moduleRegistries contains all routers to be registered
var moduleRegistries = []RouterRegistry{
	&pdfHttp.PDFRouter{}, // PDF module routes
}

// RouterRegistry registers routes of all modules
func RegisterRoutes() {
	// Set Gin mode based on the environment
	if infrastructure.GetEnvironment().Environment == infrastructure.ENVIRONMENT_PRODUCTION {
		gin.SetMode(gin.ReleaseMode)
	}

	// Start the router
	router := gin.Default()

	// Register global middlewares
	router.Use(sharedMiddlewares.ErrorHandlerMiddleware())

	// Register all routes
	apiV1 := router.Group("/api/v1")
	for _, registry := range moduleRegistries {
		registry.RegisterRoutes(apiV1)
	}

	if err := router.Run(":3000"); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
