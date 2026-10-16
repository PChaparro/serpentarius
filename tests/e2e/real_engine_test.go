package e2e

import (
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/PChaparro/serpentarius/internal/modules/pdf/application/use_cases"
	"github.com/PChaparro/serpentarius/internal/modules/pdf/infrastructure/http/controllers"
	"github.com/PChaparro/serpentarius/internal/modules/pdf/infrastructure/http/requests"
	pdfImplementations "github.com/PChaparro/serpentarius/internal/modules/pdf/infrastructure/implementations"
	sharedMiddlewares "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure/http/middlewares"
	sharedImplementations "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure/implementations"
	"github.com/gin-gonic/gin"
)

// NewRealEngine builds the production-shaped engine for real renders: error
// middleware and POST /api/v1/pdf behind auth and validation, with the
// controller wired to the rod generator, S3 storage, Redis cache and xxhash.
func NewRealEngine(t *testing.T) *gin.Engine {
	t.Helper()

	requireChromium(t)

	useCase := use_cases.GeneratePDFUseCase{
		PDFGenerator:     pdfImplementations.NewPDFGeneratorRod(),
		CloudStorage:     sharedImplementations.NewS3CloudStorage(),
		PathCacheStorage: sharedImplementations.NewRedisCacheStorage(),
		HashGenerator:    sharedImplementations.GetXxHashGenerator(),
	}
	controller := &controllers.GeneratePDFController{UseCase: useCase}

	engine := gin.Default()
	engine.Use(sharedMiddlewares.ErrorHandlerMiddleware())
	apiV1 := engine.Group("/api/v1")
	pdfGroup := apiV1.Group("/pdf")
	pdfGroup.POST(
		"",
		sharedMiddlewares.AuthMiddleware(),
		sharedMiddlewares.RequestValidationMiddleware(requests.GeneratePDFRequest{}),
		controller.Handle,
	)

	return engine
}

var (
	sharedRealServerOnce sync.Once
	sharedRealServer     *httptest.Server
)

// sharedRealServerURL starts the shared real engine once per run and returns
// its URL.
func sharedRealServerURL(t *testing.T) string {
	t.Helper()

	sharedRealServerOnce.Do(func() {
		sharedRealServer = httptest.NewServer(NewRealEngine(t))
		fmt.Printf("Shared real engine ready at %s\n", sharedRealServer.URL)
	})

	return sharedRealServer.URL
}

// closeSharedRealServer stops the shared real engine at the end of the run.
func closeSharedRealServer() {
	if sharedRealServer != nil {
		sharedRealServer.Close()
	}
}
