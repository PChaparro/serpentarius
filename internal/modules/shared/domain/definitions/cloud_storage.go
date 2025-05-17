package definitions

import "io"

type UploadFileRequest struct {
	FileReader      io.Reader
	FileFolder      string
	FilePath        string
	ContentType     string
	PublicURLPrefix string
}

type FileExistsRequest struct {
	FileFolder string
	FilePath   string
}

type CloudStorage interface {
	UploadFile(request UploadFileRequest) (string, error)
	FileExists(request FileExistsRequest) (bool, error)
}
