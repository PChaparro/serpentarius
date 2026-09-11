package http

import (
	"github.com/PChaparro/serpentarius/internal/modules/pdf/application/use_cases"
	"github.com/PChaparro/serpentarius/internal/modules/pdf/infrastructure/http/controllers"
	"github.com/PChaparro/serpentarius/internal/modules/pdf/infrastructure/http/requests"
	"github.com/PChaparro/serpentarius/internal/modules/pdf/infrastructure/implementations"
	sharedMiddlewares "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure/http/middlewares"
	sharedImplementations "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure/implementations"
	"github.com/gin-gonic/gin"
)

// PDFRouter handles the routing for the PDF module
type PDFRouter struct{}

// RegisterRoutes implements the RouterRegistry interface to register all routes for the PDF module
func (pr *PDFRouter) RegisterRoutes(r *gin.RouterGroup) {
	// Register the PDF routes
	pdfGroup := r.Group("/pdf")

	// Generate PDF and return its file reference
	generatePDFUseCase := use_cases.GeneratePDFUseCase{
		PDFGenerator:     implementations.GetPDFGeneratorRod(),
		CloudStorage:     sharedImplementations.GetS3CloudStorage(),
		PathCacheStorage: sharedImplementations.GetRedisCacheStorage(),
		HashGenerator:    sharedImplementations.GetXxHashGenerator(),
	}
	generatePDFController := &controllers.GeneratePDFController{
		UseCase: generatePDFUseCase,
	}
	pdfGroup.POST(
		// Creates a PDF and returns its file reference (bucket/key).
		"",
		sharedMiddlewares.AuthMiddleware(),
		sharedMiddlewares.RequestValidationMiddleware(requests.GeneratePDFRequest{}),
		generatePDFController.Handle,
	)
}
