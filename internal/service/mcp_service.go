package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"mcfg/internal/exitcode"
	"mcfg/internal/id"
	"mcfg/internal/model"
	"mcfg/internal/validator"
)

// MCPService 负责 MCP 配置的增删改查与启停绑定。
type MCPService struct {
	store ConfigStore
	clock Clock
	ids   id.Generator
}

// MCPAddInput 描述新增 MCP 服务时可提交的字段。
type MCPAddInput struct {
	Name        string
	Command     string
	Args        []string
	Env         map[string]string
	Description string
}

// MCPEditInput 描述编辑 MCP 服务时可变更的字段。
type MCPEditInput struct {
	Name        *string
	Command     *string
	Args        []string
	ReplaceArgs bool
	ClearArgs   bool
	Env         map[string]string
	ReplaceEnv  bool
	ClearEnv    bool
	Description *string
}

// NewMCPService 创建 MCP 服务实例。
func NewMCPService(store ConfigStore, clock Clock, gen id.Generator) *MCPService {
	return &MCPService{
		store: store,
		clock: defaultClock(clock),
		ids:   defaultIDGenerator(gen),
	}
}

// Add 新增一个手工维护的 MCP 服务。
func (s *MCPService) Add(ctx context.Context, input MCPAddInput) (model.MCPServer, error) {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return model.MCPServer{}, err
	}
	if hasMCPNameConflict(cfg.MCPServers, input.Name, "") {
		return model.MCPServer{}, fmt.Errorf("%w: mcp name %q already exists; choose a different name", exitcode.ErrBusiness, input.Name)
	}

	idValue, err := s.ids.New()
	if err != nil {
		return model.MCPServer{}, fmt.Errorf("%w: generate mcp id: %v", exitcode.ErrBusiness, err)
	}
	timestamp := nowRFC3339(s.clock)
	// MCP 当前只支持 stdio 传输，因此新增时直接固化 transport，减少上层参数复杂度。
	server := model.MCPServer{
		ID:          idValue,
		Name:        input.Name,
		Transport:   "stdio",
		Command:     input.Command,
		Args:        slices.Clone(input.Args),
		Env:         cloneMap(input.Env),
		Source:      model.SourceManual,
		Description: input.Description,
		CreatedAt:   timestamp,
		UpdatedAt:   timestamp,
	}
	if err := validator.ValidateMCPServer(server); err != nil {
		return model.MCPServer{}, err
	}
	cfg.MCPServers = append(cfg.MCPServers, server)
	if err := s.store.Save(ctx, cfg); err != nil {
		return model.MCPServer{}, err
	}
	return server, nil
}

// List 返回当前全部 MCP 服务配置。
func (s *MCPService) List(ctx context.Context) ([]model.MCPServer, error) {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return nil, err
	}
	return cfg.MCPServers, nil
}

// Edit 根据 ID 前缀更新指定 MCP 服务。
func (s *MCPService) Edit(ctx context.Context, prefix string, input MCPEditInput) (model.MCPServer, error) {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return model.MCPServer{}, err
	}

	index, err := findMCPIndex(cfg.MCPServers, prefix)
	if err != nil {
		return model.MCPServer{}, err
	}

	current := cfg.MCPServers[index]
	if input.Name != nil {
		if hasMCPNameConflict(cfg.MCPServers, *input.Name, current.ID) {
			return model.MCPServer{}, fmt.Errorf("%w: mcp name %q already exists; choose a different name", exitcode.ErrBusiness, *input.Name)
		}
		current.Name = *input.Name
	}
	if input.Command != nil {
		current.Command = *input.Command
	}
	if input.Description != nil {
		current.Description = *input.Description
	}
	// Args 和 Env 都支持清空或整体替换，保持命令行语义明确，不做隐式合并。
	switch {
	case input.ClearArgs:
		current.Args = []string{}
	case input.ReplaceArgs:
		current.Args = slices.Clone(input.Args)
	}
	switch {
	case input.ClearEnv:
		current.Env = map[string]string{}
	case input.ReplaceEnv:
		current.Env = cloneMap(input.Env)
	}
	current.UpdatedAt = nowRFC3339(s.clock)
	if err := validator.ValidateMCPServer(current); err != nil {
		return model.MCPServer{}, err
	}
	cfg.MCPServers[index] = current
	if err := s.store.Save(ctx, cfg); err != nil {
		return model.MCPServer{}, err
	}
	return current, nil
}

// Remove 根据 ID 前缀删除指定 MCP 服务。
func (s *MCPService) Remove(ctx context.Context, prefix string, force bool) error {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return err
	}
	index, err := findMCPIndex(cfg.MCPServers, prefix)
	if err != nil {
		return err
	}
	target := cfg.MCPServers[index]
	// 已启用的 MCP 默认不能直接删除，force 时会先从绑定列表中摘除。
	if slices.Contains(cfg.ClaudeBinding.EnabledMCPIDs, target.ID) {
		if !force {
			return fmt.Errorf("%w: mcp %s is enabled; use `mcfg mcp disable %s` or `mcfg mcp remove %s --force`", exitcode.ErrBusiness, target.ID, prefix, prefix)
		}
		cfg.ClaudeBinding.EnabledMCPIDs = deleteString(cfg.ClaudeBinding.EnabledMCPIDs, target.ID)
	}
	cfg.MCPServers = slices.Delete(cfg.MCPServers, index, index+1)
	return s.store.Save(ctx, cfg)
}

// Enable 根据 ID 前缀启用指定 MCP 服务。
func (s *MCPService) Enable(ctx context.Context, prefix string) (bool, model.MCPServer, error) {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return false, model.MCPServer{}, err
	}
	index, err := findMCPIndex(cfg.MCPServers, prefix)
	if err != nil {
		return false, model.MCPServer{}, err
	}
	target := cfg.MCPServers[index]
	// 返回值中的 bool 表示目标原本是否已经处于启用状态，便于 CLI 输出更准确的提示。
	if slices.Contains(cfg.ClaudeBinding.EnabledMCPIDs, target.ID) {
		return true, target, nil
	}
	cfg.ClaudeBinding.EnabledMCPIDs = append(cfg.ClaudeBinding.EnabledMCPIDs, target.ID)
	if err := s.store.Save(ctx, cfg); err != nil {
		return false, model.MCPServer{}, err
	}
	return false, target, nil
}

// Disable 根据 ID 前缀禁用指定 MCP 服务。
func (s *MCPService) Disable(ctx context.Context, prefix string) (bool, model.MCPServer, error) {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return false, model.MCPServer{}, err
	}
	index, err := findMCPIndex(cfg.MCPServers, prefix)
	if err != nil {
		return false, model.MCPServer{}, err
	}
	target := cfg.MCPServers[index]
	if !slices.Contains(cfg.ClaudeBinding.EnabledMCPIDs, target.ID) {
		return true, target, nil
	}
	cfg.ClaudeBinding.EnabledMCPIDs = deleteString(cfg.ClaudeBinding.EnabledMCPIDs, target.ID)
	if err := s.store.Save(ctx, cfg); err != nil {
		return false, model.MCPServer{}, err
	}
	return false, target, nil
}

func findMCPIndex(items []model.MCPServer, prefix string) (int, error) {
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
	return -1, fmt.Errorf("%w: mcp %q not found", exitcode.ErrBusiness, prefix)
}

func hasMCPNameConflict(items []model.MCPServer, name, skipID string) bool {
	target := normalizeMCPName(name)
	for _, item := range items {
		if item.ID == skipID {
			continue
		}
		if normalizeMCPName(item.Name) == target {
			return true
		}
	}
	return false
}

func normalizeMCPName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func deleteString(items []string, target string) []string {
	// 复用原切片底层数组过滤元素，避免额外分配。
	filtered := items[:0]
	for _, item := range items {
		if item != target {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
