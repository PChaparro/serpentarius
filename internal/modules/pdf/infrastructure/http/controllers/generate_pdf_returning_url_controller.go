package controllers

import (
	"net/http"

	"github.com/PChaparro/serpentarius/internal/modules/pdf/application/use_cases"
	"github.com/PChaparro/serpentarius/internal/modules/pdf/infrastructure/http/requests"
	sharedMiddlewares "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure/http/middlewares"
	"github.com/gin-gonic/gin"
)

// GeneratePDFReturningURLController handles the generation of a PDF and returns its public URL.
type GeneratePDFReturningURLController struct {
	UseCase use_cases.GeneratePDFReturningURLUseCase
}

// Handle processes the request to generate a PDF and return its public URL.
func (controller *GeneratePDFReturningURLController) Handle(c *gin.Context) {
	// Get validated request from context
	req := sharedMiddlewares.GetValidatedRequest(c).(*requests.GeneratePDFReturningURLRequest)

	// Convert request to DTO
	dto := req.ToDTO()

	// Call the use case
	url, err := controller.UseCase.Execute(dto)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "PDF generated successfully",
		"url":     url,
	})
}
