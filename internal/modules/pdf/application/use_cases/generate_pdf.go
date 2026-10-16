package use_cases

import (
	"encoding/json"
	"fmt"

	"github.com/PChaparro/serpentarius/internal/modules/pdf/domain/definitions"
	"github.com/PChaparro/serpentarius/internal/modules/pdf/domain/dto"
	sharedDefinitions "github.com/PChaparro/serpentarius/internal/modules/shared/domain/definitions"
	sharedUtilities "github.com/PChaparro/serpentarius/internal/modules/shared/utilities"
)

// renderHashMetadataKey is the object metadata key holding the hash of the
// request that produced the stored render. S3 user-metadata keys must stay
// lowercase ASCII.
const renderHashMetadataKey = "render-hash"

// pdfRenderCachePrefix is the prefix for PDF render cache keys.
const pdfRenderCachePrefix = "pdf:render"

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

	// Build the namespaced cache key
	redisKey := pdfRenderCachePrefix + ":" + hash
	if request.Cache.Namespace != "" {
		redisKey = pdfRenderCachePrefix + ":" + request.Cache.Namespace + ":" + hash
	}

	// Check if the key is already cached
	cachedKey, err := u.PathCacheStorage.Get(redisKey)
	if err != nil {
		return nil, fmt.Errorf("error checking cache for key: %w", err)
	}

	// If the key is cached, verify the stored object is the same render before returning it
	if cachedKey != nil {
		key := *cachedKey

		// Stat the file in cloud storage to check existence and identity
		stat, err := u.CloudStorage.StatFile(sharedDefinitions.FileExistsRequest{
			Bucket: request.Output.Bucket,
			Key:    key,
		})
		if err != nil {
			return nil, fmt.Errorf("error stating file in cloud storage: %w", err)
		}

		// A real hit requires the stored render hash to match the current request hash
		if stat.Exists && stat.Metadata[renderHashMetadataKey] == hash {
			sharedUtilities.GetLogger().
				WithField("bucket", request.Output.Bucket).
				WithField("key", key).
				Info("Cache HIT for key (render hash matches stored object)")

			return &dto.PDFGenerationResult{
				Bucket: request.Output.Bucket,
				Key:    key,
			}, nil
		}

		// The cached entry is stale (object missing, identity mismatch or
		// missing metadata): remove it and fall through to re-render
		err = u.PathCacheStorage.Delete(redisKey)
		if err != nil {
			return nil, fmt.Errorf("error deleting invalid key from cache: %w", err)
		}
	}

	// Generate the PDF
	pdfReader, err := u.PDFGenerator.GeneratePDF(request)
	if err != nil {
		return nil, err
	}

	// Upload the PDF to cloud storage, tagging it with the request hash
	// so later cache hits can verify the stored object identity
	uploadRequest := sharedDefinitions.UploadFileRequest{
		FileReader:  pdfReader,
		Bucket:      request.Output.Bucket,
		Key:         request.Output.Key,
		ContentType: "application/pdf",
		Metadata: map[string]string{
			renderHashMetadataKey: hash,
		},
	}
	key, err := u.CloudStorage.UploadFile(uploadRequest)
	if err != nil {
		return nil, fmt.Errorf("error uploading file to cloud storage: %w", err)
	}

	// Cache the key with the generated hash as the key
	cacheRequest := sharedDefinitions.SetPathCacheRequest{
		Key:        redisKey,
		Value:      key,
		Expiration: request.Cache.TTL,
	}

	err = u.PathCacheStorage.Set(cacheRequest)
	if err != nil {
		return nil, fmt.Errorf("error setting cache for key: %w", err)
	}

	// Return the file reference of the uploaded PDF
	return &dto.PDFGenerationResult{
		Bucket: request.Output.Bucket,
		Key:    key,
	}, nil
}
