package auth

import "fmt"

type Error struct {
	Code    string
	Message string
	Status  int
}

func (e *Error) Error() string {
	return e.Message
}

func ErrInvalidInput(msg string) error {
	return &Error{Code: "invalid_input", Message: msg, Status: 400}
}

func ErrUnauthorized(msg string) error {
	return &Error{Code: "unauthorized", Message: msg, Status: 401}
}

func ErrForbidden(msg string) error {
	return &Error{Code: "forbidden", Message: msg, Status: 403}
}

func ErrNotFound(msg string) error {
	return &Error{Code: "not_found", Message: msg, Status: 404}
}

func ErrConflict(msg string) error {
	return &Error{Code: "conflict", Message: msg, Status: 409}
}

func AsError(err error) (*Error, bool) {
	if err == nil {
		return nil, false
	}
	if e, ok := err.(*Error); ok {
		return e, true
	}
	return nil, false
}

func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}
