package use_cases

import (
	"encoding/json"
	"fmt"

	"github.com/PChaparro/serpentarius/internal/modules/pdf/domain/definitions"
	"github.com/PChaparro/serpentarius/internal/modules/pdf/domain/dto"
	sharedDefinitions "github.com/PChaparro/serpentarius/internal/modules/shared/domain/definitions"
	sharedInfrastructure "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure"
)

// GeneratePDFReturningURLUseCase is the use case for generating a PDF and returning its public URL.
type GeneratePDFReturningURLUseCase struct {
	// PDFGenerator is the interface for generating PDFs
	PDFGenerator definitions.PDFGenerator
	// CloudStorage is the interface for cloud storage operations
	CloudStorage sharedDefinitions.CloudStorage
	// URLCacheStorage is the interface for URL cache storage operations
	URLCacheStorage sharedDefinitions.UrlCacheStorage
	// HashGenerator is the interface for generating hashes
	HashGenerator sharedDefinitions.HashGenerator
	// Fetcher is the interface for fetching content from URLs
	Fetcher sharedDefinitions.Fetcher
}

// Execute generates a PDF based on the provided request and returns the URL of the generated PDF.
func (u *GeneratePDFReturningURLUseCase) Execute(
	request *dto.PDFGenerationDTO,
) (string, error) {
	// Stringify the request to generate the cache key from it
	stringifiedRequest, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("error stringifying request to generate cache key: %w", err)
	}

	// Generate a hash from the stringified request to use as a cache key
	hash, err := u.HashGenerator.GenerateHash(string(stringifiedRequest))
	if err != nil {
		return "", fmt.Errorf("error generating hash for cache key: %w", err)
	}

	// Check if the URL is already cached
	cachedURL, err := u.URLCacheStorage.Get(hash)
	if err != nil {
		return "", fmt.Errorf("error checking cache for URL: %w", err)
	}

	// If the URL is cached, check if it is still valid and return it
	if cachedURL != nil {
		url := *cachedURL

		// Check if the URL is still valid
		_, err := u.Fetcher.Get(sharedDefinitions.GetRequest{
			URL: url,
		})
		if err == nil {
			sharedInfrastructure.GetLogger().
				WithField("url", url).
				Info("Cache HIT for URL")

			return url, nil
		}

		// If the URL is not valid, remove it from the cache
		err = u.URLCacheStorage.Delete(hash)
		if err != nil {
			return "", fmt.Errorf("error deleting invalid URL from cache: %w", err)
		}
	}

	// Generate the PDF
	pdfReader, err := u.PDFGenerator.GeneratePDF(request)
	if err != nil {
		return "", err
	}

	// Upload the PDF to cloud storage
	uploadRequest := sharedDefinitions.UploadFileRequest{
		FileReader:      pdfReader,
		FileFolder:      request.Config.Directory,
		FilePath:        request.Config.FileName,
		ContentType:     "application/pdf",
		PublicURLPrefix: request.Config.PublicURLPrefix,
	}
	url, err := u.CloudStorage.UploadFile(uploadRequest)
	if err != nil {
		return "", fmt.Errorf("error uploading file to cloud storage: %w", err)
	}

	// Cache the URL with the generated hash as the key
	cacheRequest := sharedDefinitions.SetURLCacheRequest{
		Key:        hash,
		Value:      url,
		Expiration: *request.Config.Expiration,
	}

	err = u.URLCacheStorage.Set(cacheRequest)
	if err != nil {
		return "", fmt.Errorf("error setting cache for URL: %w", err)
	}

	// Return the public URL of the uploaded PDF
	return url, nil
}
