package exitcode

import (
	"errors"
	"strings"
)

const (
	// Success 表示命令执行成功。
	Success = 0
	// Business 表示业务层错误。
	Business = 1
	// LockConflict 表示锁冲突错误。
	LockConflict = 2
	// IO 表示文件或系统 IO 错误。
	IO = 3
	// Param 表示参数校验错误。
	Param = 4
)

var (
	// ErrBusiness 标记业务层错误。
	ErrBusiness = errors.New("business error")
	// ErrLock 标记锁冲突错误。
	ErrLock = errors.New("lock conflict")
	// ErrIO 标记 IO 错误。
	ErrIO = errors.New("io error")
	// ErrParam 标记参数错误。
	ErrParam = errors.New("parameter error")
)

// FromError 根据错误类型返回对应的进程退出码。
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
