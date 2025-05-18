package definitions

import "io"

// UploadFileRequest represents the request for uploading a file to cloud storage.
type UploadFileRequest struct {
	FileReader      io.Reader
	FileFolder      string
	FilePath        string
	ContentType     string
	PublicURLPrefix string
}

// FileExistsRequest represents the request for checking if a file exists in cloud storage.
type FileExistsRequest struct {
	FileFolder string
	FilePath   string
}

// CloudStorage is an interface for cloud storage operations.
type CloudStorage interface {
	UploadFile(request UploadFileRequest) (string, error)
	FileExists(request FileExistsRequest) (bool, error)
}
