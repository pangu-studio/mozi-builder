package apierror

import "fmt"

type Error struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

type Envelope struct {
	Error Error `json:"error"`
}

func New(code, message string, details any) *Error {
	return &Error{Code: code, Message: message, Details: details}
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func Wrap(err *Error, requestID string) Envelope {
	copy := *err
	copy.RequestID = requestID
	return Envelope{Error: copy}
}
