package errors

type DomainError interface {
	Code() string
	Message() string
	Metadata() map[string]any
}

type GenericDomainError struct {
	code     string
	message  string
	metadata map[string]any
}

type CreateDomainErrorArguments struct {
	Code     *string
	Message  string
	Metadata map[string]any
}

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

func (e *GenericDomainError) Code() string {
	return e.code
}

func (e *GenericDomainError) Message() string {
	return e.message
}

func (e *GenericDomainError) Metadata() map[string]any {
	return e.metadata
}
