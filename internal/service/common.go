package service

import (
	"context"
	"time"

	"mcfg/internal/id"
	"mcfg/internal/model"
)

// ConfigStore 定义服务层依赖的配置持久化接口。
type ConfigStore interface {
	Load(context.Context) (model.ConfigRoot, error)
	Save(context.Context, model.ConfigRoot) error
}

// Clock 定义服务层依赖的时间来源接口。
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now().UTC()
}

func nowRFC3339(clock Clock) string {
	return clock.Now().Format(time.RFC3339)
}

func defaultClock(clock Clock) Clock {
	if clock != nil {
		return clock
	}
	return realClock{}
}

func defaultIDGenerator(gen id.Generator) id.Generator {
	if gen != nil {
		return gen
	}
	return id.ULIDGenerator{}
}

func cloneMap(input map[string]string) map[string]string {
	// 统一返回非 nil map，减少业务层额外的空值判断。
	if len(input) == 0 {
		return map[string]string{}
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
