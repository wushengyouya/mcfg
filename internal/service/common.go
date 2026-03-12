package service

import (
	"context"
	"time"

	"mcfg/internal/id"
	"mcfg/internal/model"
)

type ConfigStore interface {
	Load(context.Context) (model.ConfigRoot, error)
	Save(context.Context, model.ConfigRoot) error
}

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
	if len(input) == 0 {
		return map[string]string{}
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
