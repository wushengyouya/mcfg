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

type MCPService struct {
	store ConfigStore
	clock Clock
	ids   id.Generator
}

type MCPAddInput struct {
	Name        string
	Command     string
	Args        []string
	Env         map[string]string
	Description string
}

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

func NewMCPService(store ConfigStore, clock Clock, gen id.Generator) *MCPService {
	return &MCPService{
		store: store,
		clock: defaultClock(clock),
		ids:   defaultIDGenerator(gen),
	}
}

func (s *MCPService) Add(ctx context.Context, input MCPAddInput) (model.MCPServer, error) {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return model.MCPServer{}, err
	}

	idValue, err := s.ids.New()
	if err != nil {
		return model.MCPServer{}, fmt.Errorf("%w: generate mcp id: %v", exitcode.ErrBusiness, err)
	}
	timestamp := nowRFC3339(s.clock)
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

func (s *MCPService) List(ctx context.Context) ([]model.MCPServer, error) {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return nil, err
	}
	return cfg.MCPServers, nil
}

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
		current.Name = *input.Name
	}
	if input.Command != nil {
		current.Command = *input.Command
	}
	if input.Description != nil {
		current.Description = *input.Description
	}
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
	if slices.Contains(cfg.ClaudeBinding.EnabledMCPIDs, target.ID) {
		if !force {
			return fmt.Errorf("%w: mcp %s is enabled; use `mcfg mcp disable %s` or `mcfg mcp remove %s --force`", exitcode.ErrBusiness, target.ID, prefix, prefix)
		}
		cfg.ClaudeBinding.EnabledMCPIDs = deleteString(cfg.ClaudeBinding.EnabledMCPIDs, target.ID)
	}
	cfg.MCPServers = slices.Delete(cfg.MCPServers, index, index+1)
	return s.store.Save(ctx, cfg)
}

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
	if slices.Contains(cfg.ClaudeBinding.EnabledMCPIDs, target.ID) {
		return true, target, nil
	}
	cfg.ClaudeBinding.EnabledMCPIDs = append(cfg.ClaudeBinding.EnabledMCPIDs, target.ID)
	if err := s.store.Save(ctx, cfg); err != nil {
		return false, model.MCPServer{}, err
	}
	return false, target, nil
}

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

func deleteString(items []string, target string) []string {
	filtered := items[:0]
	for _, item := range items {
		if item != target {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
