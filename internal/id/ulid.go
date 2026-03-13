package id

import (
	"crypto/rand"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"

	"mcfg/internal/exitcode"
)

// Generator 定义生成字符串 ID 的接口。
type Generator interface {
	New() (string, error)
}

// ULIDGenerator 使用 ULID 生成全局唯一标识。
type ULIDGenerator struct{}

var (
	entropyMu sync.Mutex
	entropy   = ulid.Monotonic(rand.Reader, 0)
)

// New 生成一个按时间有序的 ULID 字符串。
func (ULIDGenerator) New() (string, error) {
	entropyMu.Lock()
	defer entropyMu.Unlock()

	return ulid.MustNew(ulid.Timestamp(time.Now().UTC()), entropy).String(), nil
}

// MatchByPrefix 根据前缀在候选 ID 列表中解析唯一匹配项。
func MatchByPrefix(prefix string, ids []string) (string, error) {
	if len(prefix) < 8 {
		return "", fmt.Errorf("%w: id prefix must be at least 8 characters", exitcode.ErrParam)
	}

	if slices.Contains(ids, prefix) {
		return prefix, nil
	}

	matches := make([]string, 0, 1)
	for _, candidate := range ids {
		if strings.HasPrefix(candidate, prefix) {
			matches = append(matches, candidate)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("%w: id %q not found", exitcode.ErrBusiness, prefix)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("%w: id prefix %q is ambiguous", exitcode.ErrBusiness, prefix)
	}
}
