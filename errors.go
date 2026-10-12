package telbiz

import "errors"

var (
	ErrValidation        = errors.New("telbiz: invalid input")
	ErrClosed            = errors.New("telbiz: client closed")
	ErrAuthentication    = errors.New("telbiz: authentication failed")
	ErrProvider          = errors.New("telbiz: provider rejected request")
	ErrTransport         = errors.New("telbiz: HTTP request failed")
	ErrMalformedResponse = errors.New("telbiz: malformed response")
)

// Error is inspectable with errors.As; Kind is inspectable with errors.Is.
// Provider text fields may echo sensitive inputs. Error() omits those fields,
// provider codes, bodies, URLs and underlying error text.
type Error struct {
	Kind           error
	Operation      string
	Field          string
	HTTPStatus     int
	Code           string
	Message        string
	Detail         string
	OutcomeUnknown bool
	cause          error
}

func (e *Error) Error() string {
	if e.Kind == nil {
		return "telbiz: operation failed"
	}
	return e.Kind.Error()
}

// Is matches the SDK error category; Unwrap preserves context/transport causes.
func (e *Error) Is(target error) bool { return target == e.Kind }
func (e *Error) Unwrap() error        { return e.cause }

func validation(op, field string) error {
	return &Error{Kind: ErrValidation, Operation: op, Field: field}
}

func authenticationError(err error) error {
	e := &Error{Kind: ErrAuthentication, Operation: "authenticate", cause: err}
	var inner *Error
	if errors.As(err, &inner) {
		e.HTTPStatus, e.Code, e.Message, e.Detail = inner.HTTPStatus, inner.Code, inner.Message, inner.Detail
	}
	return e
}
