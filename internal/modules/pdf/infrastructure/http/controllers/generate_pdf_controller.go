package controllers

import (
	"net/http"

	"github.com/PChaparro/serpentarius/internal/modules/pdf/application/use_cases"
	"github.com/PChaparro/serpentarius/internal/modules/pdf/infrastructure/http/requests"
	sharedMiddlewares "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure/http/middlewares"
	"github.com/gin-gonic/gin"
)

// GeneratePDFController handles the generation of a PDF and returns its file reference.
type GeneratePDFController struct {
	UseCase use_cases.GeneratePDFUseCase
}

// Handle processes the request to generate a PDF and return its file reference.
func (controller *GeneratePDFController) Handle(c *gin.Context) {
	// Get validated request from context
	req := sharedMiddlewares.GetValidatedRequest(c).(*requests.GeneratePDFRequest)

	// Convert request to DTO
	dto := req.ToDTO()

	// Call the use case
	result, err := controller.UseCase.Execute(dto)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "PDF generated successfully",
		"bucket":  result.Bucket,
		"key":     result.Key,
	})
}
