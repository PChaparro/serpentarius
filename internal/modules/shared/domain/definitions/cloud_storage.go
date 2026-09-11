package definitions

import "io"

// UploadFileRequest represents the request for uploading a file to cloud storage.
type UploadFileRequest struct {
	FileReader  io.Reader
	Bucket      string
	Key         string
	ContentType string
	Metadata    map[string]string
}

// FileExistsRequest represents the request for checking if a file exists in cloud storage.
type FileExistsRequest struct {
	Bucket string
	Key    string
}

// StatFileResult represents the result of stating a file in cloud storage.
type StatFileResult struct {
	Exists   bool
	Metadata map[string]string
}

// CloudStorage is an interface for cloud storage operations.
type CloudStorage interface {
	UploadFile(request UploadFileRequest) (string, error)
	StatFile(request FileExistsRequest) (StatFileResult, error)
}
