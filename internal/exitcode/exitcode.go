package exitcode

import (
	"errors"
	"strings"
)

const (
	Success      = 0
	Business     = 1
	LockConflict = 2
	IO           = 3
	Param        = 4
)

var (
	ErrBusiness = errors.New("business error")
	ErrLock     = errors.New("lock conflict")
	ErrIO       = errors.New("io error")
	ErrParam    = errors.New("parameter error")
)

func FromError(err error) int {
	if err == nil {
		return Success
	}

	switch {
	case errors.Is(err, ErrParam):
		return Param
	case errors.Is(err, ErrIO):
		return IO
	case errors.Is(err, ErrLock):
		return LockConflict
	case looksLikeParamError(err.Error()):
		return Param
	default:
		return Business
	}
}

func looksLikeParamError(message string) bool {
	return strings.Contains(message, "required flag") ||
		strings.Contains(message, "unknown flag") ||
		strings.Contains(message, "accepts ") ||
		strings.Contains(message, "invalid argument") ||
		strings.Contains(message, "argument ") ||
		strings.Contains(message, "requires at least") ||
		strings.Contains(message, "requires at most") ||
		strings.Contains(message, "mutually exclusive")
}
