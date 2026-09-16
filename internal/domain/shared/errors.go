package shared

import (
	"errors"
	"fmt"
)

type Kind uint8

const (
	KindInternal Kind = iota
	KindInvalid
	KindNotFound
	KindConflict
	KindUnprocessable
	KindUnauthenticated
)

type Error struct {
	Kind Kind
	Code string
	Msg  string
	err  error
}

func (e *Error) Error() string {
	if e.err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Msg, e.err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Msg)
}

func (e *Error) Unwrap() error { return e.err }

func newError(kind Kind, code, format string, args ...any) *Error {
	return &Error{Kind: kind, Code: code, Msg: fmt.Sprintf(format, args...)}
}

func Invalid(code, format string, args ...any) *Error {
	return newError(KindInvalid, code, format, args...)
}

func NotFound(code, format string, args ...any) *Error {
	return newError(KindNotFound, code, format, args...)
}

func Conflict(code, format string, args ...any) *Error {
	return newError(KindConflict, code, format, args...)
}

func Unprocessable(code, format string, args ...any) *Error {
	return newError(KindUnprocessable, code, format, args...)
}

func Internal(code string, err error) *Error {
	return &Error{Kind: KindInternal, Code: code, Msg: "internal error", err: err}
}

func Unauthenticated(code, format string, args ...any) *Error {
	return newError(KindUnauthenticated, code, format, args...)
}

func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindInternal
}

func CodeOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return "internal_error"
}

func MessageOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Msg
	}
	return "internal error"
}
