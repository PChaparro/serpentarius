package use_cases

import (
	"encoding/json"
	"fmt"

	"github.com/PChaparro/serpentarius/internal/modules/pdf/domain/definitions"
	"github.com/PChaparro/serpentarius/internal/modules/pdf/domain/dto"
	sharedDefinitions "github.com/PChaparro/serpentarius/internal/modules/shared/domain/definitions"
	sharedUtilities "github.com/PChaparro/serpentarius/internal/modules/shared/utilities"
)

// GeneratePDFUseCase is the use case for generating a PDF and returning its file reference.
type GeneratePDFUseCase struct {
	// PDFGenerator is the interface for generating PDFs
	PDFGenerator definitions.PDFGenerator
	// CloudStorage is the interface for cloud storage operations
	CloudStorage sharedDefinitions.CloudStorage
	// PathCacheStorage is the interface for path cache storage operations
	PathCacheStorage sharedDefinitions.PathCacheStorage
	// HashGenerator is the interface for generating hashes
	HashGenerator sharedDefinitions.HashGenerator
}

// Execute generates a PDF based on the provided request and returns the file reference of the generated PDF.
func (u *GeneratePDFUseCase) Execute(
	request *dto.PDFGenerationDTO,
) (*dto.PDFGenerationResult, error) {
	// Stringify the request to generate the cache key from it
	stringifiedRequest, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error stringifying request to generate cache key: %w", err)
	}

	// Generate a hash from the stringified request to use as a cache key
	hash, err := u.HashGenerator.GenerateHash(string(stringifiedRequest))
	if err != nil {
		return nil, fmt.Errorf("error generating hash for cache key: %w", err)
	}

	// Check if the key is already cached
	cachedKey, err := u.PathCacheStorage.Get(hash)
	if err != nil {
		return nil, fmt.Errorf("error checking cache for key: %w", err)
	}

	// If the key is cached, check if the file exists in cloud storage and return it
	if cachedKey != nil {
		key := *cachedKey

		// Check if the file exists in cloud storage
		fileExists, err := u.CloudStorage.FileExists(sharedDefinitions.FileExistsRequest{
			Bucket: request.Config.Bucket,
			Key:    key,
		})
		if err != nil {
			return nil, fmt.Errorf("error checking file existence in cloud storage: %w", err)
		}

		// If the file exists, return the cached file reference
		if fileExists {
			sharedUtilities.GetLogger().
				WithField("bucket", request.Config.Bucket).
				WithField("key", key).
				Info("Cache HIT for key (file exists in cloud storage)")

			return &dto.PDFGenerationResult{
				Bucket: request.Config.Bucket,
				Key:    key,
			}, nil
		}

		// If the file does not exist, remove it from the cache
		err = u.PathCacheStorage.Delete(hash)
		if err != nil {
			return nil, fmt.Errorf("error deleting invalid key from cache: %w", err)
		}
	}

	// Generate the PDF
	pdfReader, err := u.PDFGenerator.GeneratePDF(request)
	if err != nil {
		return nil, err
	}

	// Upload the PDF to cloud storage
	uploadRequest := sharedDefinitions.UploadFileRequest{
		FileReader:  pdfReader,
		Bucket:      request.Config.Bucket,
		Key:         request.Config.Key,
		ContentType: "application/pdf",
	}
	key, err := u.CloudStorage.UploadFile(uploadRequest)
	if err != nil {
		return nil, fmt.Errorf("error uploading file to cloud storage: %w", err)
	}

	// Cache the key with the generated hash as the key
	cacheRequest := sharedDefinitions.SetPathCacheRequest{
		Key:        hash,
		Value:      key,
		Expiration: *request.Config.Expiration,
	}

	err = u.PathCacheStorage.Set(cacheRequest)
	if err != nil {
		return nil, fmt.Errorf("error setting cache for key: %w", err)
	}

	// Return the file reference of the uploaded PDF
	return &dto.PDFGenerationResult{
		Bucket: request.Config.Bucket,
		Key:    key,
	}, nil
}
