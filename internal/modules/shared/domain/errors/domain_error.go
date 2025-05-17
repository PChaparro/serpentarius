package errors

// DomainError is an interface that represents a domain error in the application.
type DomainError interface {
	Code() string
	Message() string
	Metadata() map[string]any
}

// GenericDomainError is a struct that implements the DomainError interface.
type GenericDomainError struct {
	code     string
	message  string
	metadata map[string]any
}

// CreateDomainErrorArguments is a struct that holds the arguments for creating a domain error.
type CreateDomainErrorArguments struct {
	Code     *string
	Message  string
	Metadata map[string]any
}

// NewGenericDomainError creates a new instance of GenericDomainError with the provided arguments.
func NewGenericDomainError(args CreateDomainErrorArguments) DomainError {
	errorCode := "ERROR"

	if args.Code != nil {
		errorCode = *args.Code
	}

	return &GenericDomainError{
		code:     errorCode,
		message:  args.Message,
		metadata: args.Metadata,
	}
}

// Code returns the error code of the domain error.
func (e *GenericDomainError) Code() string {
	return e.code
}

// Message returns the error message of the domain error.
func (e *GenericDomainError) Message() string {
	return e.message
}

// Metadata returns the metadata of the domain error.
func (e *GenericDomainError) Metadata() map[string]any {
	return e.metadata
}
