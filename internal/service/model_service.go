package service

import (
	"context"
	"fmt"
	"slices"

	"mcfg/internal/exitcode"
	"mcfg/internal/id"
	"mcfg/internal/model"
	"mcfg/internal/validator"
)

var reservedModelEnvKeys = map[string]struct{}{
	"ANTHROPIC_AUTH_TOKEN": {},
	"ANTHROPIC_BASE_URL":   {},
	"ANTHROPIC_MODEL":      {},
}

type ModelService struct {
	store ConfigStore
	clock Clock
	ids   id.Generator
}

type ModelAddInput struct {
	Name        string
	AuthToken   string
	BaseURL     string
	Model       string
	Env         map[string]string
	Description string
}

type ModelEditInput struct {
	Name        *string
	AuthToken   *string
	BaseURL     *string
	Model       *string
	Env         map[string]string
	ReplaceEnv  bool
	ClearEnv    bool
	Description *string
}

func NewModelService(store ConfigStore, clock Clock, gen id.Generator) *ModelService {
	return &ModelService{
		store: store,
		clock: defaultClock(clock),
		ids:   defaultIDGenerator(gen),
	}
}

func (s *ModelService) Add(ctx context.Context, input ModelAddInput) (model.ModelProfile, error) {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return model.ModelProfile{}, err
	}

	if err := validateModelEnv(input.Env); err != nil {
		return model.ModelProfile{}, err
	}

	idValue, err := s.ids.New()
	if err != nil {
		return model.ModelProfile{}, fmt.Errorf("%w: generate model id: %v", exitcode.ErrBusiness, err)
	}
	timestamp := nowRFC3339(s.clock)
	profile := model.ModelProfile{
		ID:          idValue,
		Name:        input.Name,
		Env:         buildModelEnv(input.AuthToken, input.BaseURL, input.Model, input.Env),
		Source:      model.SourceManual,
		Description: input.Description,
		CreatedAt:   timestamp,
		UpdatedAt:   timestamp,
	}
	if err := validator.ValidateModelProfile(profile); err != nil {
		return model.ModelProfile{}, err
	}

	cfg.Models = append(cfg.Models, profile)
	if err := s.store.Save(ctx, cfg); err != nil {
		return model.ModelProfile{}, err
	}
	return profile, nil
}

func (s *ModelService) List(ctx context.Context) ([]model.ModelProfile, error) {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return nil, err
	}
	return cfg.Models, nil
}

func (s *ModelService) Edit(ctx context.Context, prefix string, input ModelEditInput) (model.ModelProfile, error) {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return model.ModelProfile{}, err
	}

	index, err := findModelIndex(cfg.Models, prefix)
	if err != nil {
		return model.ModelProfile{}, err
	}

	current := cfg.Models[index]
	if input.Name != nil {
		current.Name = *input.Name
	}
	if input.Description != nil {
		current.Description = *input.Description
	}

	customEnv := extractCustomModelEnv(current.Env)
	switch {
	case input.ClearEnv:
		customEnv = map[string]string{}
	case input.ReplaceEnv:
		if err := validateModelEnv(input.Env); err != nil {
			return model.ModelProfile{}, err
		}
		customEnv = cloneMap(input.Env)
	}

	token := current.Env["ANTHROPIC_AUTH_TOKEN"]
	if input.AuthToken != nil {
		token = *input.AuthToken
	}
	baseURL := current.Env["ANTHROPIC_BASE_URL"]
	if input.BaseURL != nil {
		baseURL = *input.BaseURL
	}
	modelName := current.Env["ANTHROPIC_MODEL"]
	if input.Model != nil {
		modelName = *input.Model
	}

	current.Env = buildModelEnv(token, baseURL, modelName, customEnv)
	current.UpdatedAt = nowRFC3339(s.clock)
	if err := validator.ValidateModelProfile(current); err != nil {
		return model.ModelProfile{}, err
	}

	cfg.Models[index] = current
	if err := s.store.Save(ctx, cfg); err != nil {
		return model.ModelProfile{}, err
	}
	return current, nil
}

func (s *ModelService) Remove(ctx context.Context, prefix string, force bool) error {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return err
	}

	index, err := findModelIndex(cfg.Models, prefix)
	if err != nil {
		return err
	}
	target := cfg.Models[index]
	if cfg.ClaudeBinding.CurrentModelID == target.ID {
		if !force {
			return fmt.Errorf("%w: model %s is currently bound; use `mcfg model use <other-id>` or `mcfg model remove %s --force`", exitcode.ErrBusiness, target.ID, prefix)
		}
		cfg.ClaudeBinding.CurrentModelID = ""
	}

	cfg.Models = slices.Delete(cfg.Models, index, index+1)
	return s.store.Save(ctx, cfg)
}

func (s *ModelService) Use(ctx context.Context, prefix string) (model.ModelProfile, error) {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return model.ModelProfile{}, err
	}

	index, err := findModelIndex(cfg.Models, prefix)
	if err != nil {
		return model.ModelProfile{}, err
	}
	cfg.ClaudeBinding.CurrentModelID = cfg.Models[index].ID
	if err := s.store.Save(ctx, cfg); err != nil {
		return model.ModelProfile{}, err
	}
	return cfg.Models[index], nil
}

func findModelIndex(items []model.ModelProfile, prefix string) (int, error) {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	matched, err := id.MatchByPrefix(prefix, ids)
	if err != nil {
		return -1, err
	}
	for index, item := range items {
		if item.ID == matched {
			return index, nil
		}
	}
	return -1, fmt.Errorf("%w: model %q not found", exitcode.ErrBusiness, prefix)
}

func validateModelEnv(env map[string]string) error {
	for key := range env {
		if _, reserved := reservedModelEnvKeys[key]; reserved {
			return fmt.Errorf("%w: reserved env key %s must use a dedicated flag", exitcode.ErrParam, key)
		}
	}
	return nil
}

func extractCustomModelEnv(env map[string]string) map[string]string {
	custom := map[string]string{}
	for key, value := range env {
		if _, reserved := reservedModelEnvKeys[key]; reserved {
			continue
		}
		custom[key] = value
	}
	return custom
}

func buildModelEnv(token, baseURL, modelName string, custom map[string]string) map[string]string {
	env := cloneMap(custom)
	env["ANTHROPIC_AUTH_TOKEN"] = token
	env["ANTHROPIC_BASE_URL"] = baseURL
	env["ANTHROPIC_MODEL"] = modelName
	return env
}
