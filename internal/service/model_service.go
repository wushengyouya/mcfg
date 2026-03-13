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

// ModelService 负责模型配置的增删改查与绑定切换。
type ModelService struct {
	store ConfigStore
	clock Clock
	ids   id.Generator
}

// ModelAddInput 描述新增模型时可提交的字段。
type ModelAddInput struct {
	Name        string
	AuthToken   string
	BaseURL     string
	Model       string
	Env         map[string]string
	Description string
}

// ModelEditInput 描述编辑模型时可变更的字段。
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

// NewModelService 创建模型服务实例。
func NewModelService(store ConfigStore, clock Clock, gen id.Generator) *ModelService {
	return &ModelService{
		store: store,
		clock: defaultClock(clock),
		ids:   defaultIDGenerator(gen),
	}
}

// Add 新增一个手工维护的模型配置。
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
	// 模型的三项核心连接信息统一拼装进 Env，外部自定义变量只能走 custom env。
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

// List 返回当前全部模型配置。
func (s *ModelService) List(ctx context.Context) ([]model.ModelProfile, error) {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return nil, err
	}
	return cfg.Models, nil
}

// Edit 根据 ID 前缀更新指定模型。
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

	// 先拆出自定义环境变量，再根据 Replace/Clear 语义更新，避免误覆盖保留字段。
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

	// 保留字段单独处理，这样 CLI 可以通过专用 flag 精准更新认证与模型绑定信息。
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

// Remove 根据 ID 前缀删除指定模型。
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
	// 正在使用的模型默认不允许删除，除非显式 force，同时清除当前绑定关系。
	if cfg.ClaudeBinding.CurrentModelID == target.ID {
		if !force {
			return fmt.Errorf("%w: model %s is currently bound; use `mcfg model use <other-id>` or `mcfg model remove %s --force`", exitcode.ErrBusiness, target.ID, prefix)
		}
		cfg.ClaudeBinding.CurrentModelID = ""
	}

	cfg.Models = slices.Delete(cfg.Models, index, index+1)
	return s.store.Save(ctx, cfg)
}

// Use 根据 ID 前缀切换当前绑定模型。
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
	// 所有查找都支持前缀匹配，便于 CLI 使用短 ID 操作对象。
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
	// 保留键必须通过专用参数传入，避免普通 env 覆盖业务关键字段。
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
	// 先复制自定义变量，再覆盖保留键，确保最终结果始终以显式参数为准。
	env := cloneMap(custom)
	env["ANTHROPIC_AUTH_TOKEN"] = token
	env["ANTHROPIC_BASE_URL"] = baseURL
	env["ANTHROPIC_MODEL"] = modelName
	return env
}
