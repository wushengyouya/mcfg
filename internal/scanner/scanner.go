package scanner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"mcfg/internal/id"
	"mcfg/internal/model"
)

type Scanner struct {
	homeDir string
	now     func() string
	ids     id.Generator
}

type Warning struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Result struct {
	Models     []model.ModelProfile `json:"models"`
	MCPServers []model.MCPServer    `json:"mcp_servers"`
	Warnings   []Warning            `json:"warnings"`
	Skipped    int                  `json:"skipped"`
}

func New(homeDir string, now func() string, gen id.Generator) *Scanner {
	return &Scanner{homeDir: homeDir, now: now, ids: gen}
}

func (s *Scanner) Scan(ctx context.Context, existing model.ConfigRoot) (Result, error) {
	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}

	result := Result{
		Models:     []model.ModelProfile{},
		MCPServers: []model.MCPServer{},
		Warnings:   []Warning{},
	}

	settingsPath := filepath.Join(s.homeDir, ".claude", "settings.json")
	if profile, warning, ok := s.scanSettings(settingsPath, existing); ok {
		result.Models = append(result.Models, profile)
	} else if warning != nil {
		result.Warnings = append(result.Warnings, *warning)
	}

	mcps, warnings := s.scanMCPs(filepath.Join(s.homeDir, ".claude.json"), existing)
	result.MCPServers = append(result.MCPServers, mcps...)
	result.Warnings = append(result.Warnings, warnings...)
	for _, warning := range result.Warnings {
		if warning.Code == "model_skipped" || warning.Code == "mcp_skipped" {
			result.Skipped++
		}
	}
	return result, nil
}

func (s *Scanner) scanSettings(path string, existing model.ConfigRoot) (model.ModelProfile, *Warning, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.ModelProfile{}, &Warning{Path: path, Code: "settings_missing", Message: "Claude settings.json not found"}, false
		}
		return model.ModelProfile{}, &Warning{Path: path, Code: "settings_read_failed", Message: err.Error()}, false
	}

	var payload struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return model.ModelProfile{}, &Warning{Path: path, Code: "settings_corrupted", Message: "Claude settings.json is corrupted"}, false
	}
	if payload.Env["ANTHROPIC_AUTH_TOKEN"] == "" || payload.Env["ANTHROPIC_BASE_URL"] == "" || payload.Env["ANTHROPIC_MODEL"] == "" {
		return model.ModelProfile{}, nil, false
	}
	if hasDuplicateModel(existing.Models, payload.Env["ANTHROPIC_MODEL"], payload.Env["ANTHROPIC_BASE_URL"]) {
		return model.ModelProfile{}, &Warning{Path: path, Code: "model_skipped", Message: "duplicate model skipped"}, false
	}

	idValue, err := s.ids.New()
	if err != nil {
		return model.ModelProfile{}, &Warning{Path: path, Code: "id_generation_failed", Message: err.Error()}, false
	}
	return model.ModelProfile{
		ID:        idValue,
		Name:      payload.Env["ANTHROPIC_MODEL"],
		Env:       payload.Env,
		Source:    model.SourceImported,
		CreatedAt: s.now(),
		UpdatedAt: s.now(),
	}, nil, true
}

func (s *Scanner) scanMCPs(path string, existing model.ConfigRoot) ([]model.MCPServer, []Warning) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, []Warning{{Path: path, Code: "claude_json_missing", Message: "Claude user mcp config not found"}}
		}
		return nil, []Warning{{Path: path, Code: "claude_json_read_failed", Message: err.Error()}}
	}

	var payload map[string]map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, []Warning{{Path: path, Code: "claude_json_corrupted", Message: "Claude user mcp config is corrupted"}}
	}

	homeNode, ok := payload[s.homeDir]
	if !ok {
		return nil, nil
	}
	rawServers, ok := homeNode["mcpServers"].(map[string]any)
	if !ok {
		return nil, nil
	}

	names := make([]string, 0, len(rawServers))
	for name := range rawServers {
		names = append(names, name)
	}
	sort.Strings(names)

	servers := make([]model.MCPServer, 0, len(names))
	warnings := []Warning{}
	for _, name := range names {
		entry, ok := rawServers[name].(map[string]any)
		if !ok {
			warnings = append(warnings, Warning{Path: path, Code: "mcp_invalid", Message: fmt.Sprintf("mcp %s must be an object", name)})
			continue
		}
		transport, _ := entry["type"].(string)
		command, _ := entry["command"].(string)
		if transport != "stdio" || command == "" {
			warnings = append(warnings, Warning{Path: path, Code: "mcp_invalid", Message: fmt.Sprintf("mcp %s is invalid", name)})
			continue
		}

		args := []string{}
		if rawArgs, ok := entry["args"].([]any); ok {
			for _, item := range rawArgs {
				value, _ := item.(string)
				args = append(args, value)
			}
		}

		env := map[string]string{}
		if rawEnv, ok := entry["env"].(map[string]any); ok {
			for key, value := range rawEnv {
				stringValue, _ := value.(string)
				env[key] = stringValue
			}
		}

		if hasDuplicateMCP(existing.MCPServers, transport, command, args, env) {
			warnings = append(warnings, Warning{Path: path, Code: "mcp_skipped", Message: fmt.Sprintf("duplicate mcp %s skipped", name)})
			continue
		}

		idValue, idErr := s.ids.New()
		if idErr != nil {
			warnings = append(warnings, Warning{Path: path, Code: "id_generation_failed", Message: idErr.Error()})
			continue
		}
		servers = append(servers, model.MCPServer{
			ID:        idValue,
			Name:      name,
			Transport: transport,
			Command:   command,
			Args:      args,
			Env:       env,
			Source:    model.SourceImported,
			CreatedAt: s.now(),
			UpdatedAt: s.now(),
		})
	}
	return servers, warnings
}

func hasDuplicateModel(items []model.ModelProfile, modelName, baseURL string) bool {
	for _, item := range items {
		if item.Env["ANTHROPIC_MODEL"] == modelName && item.Env["ANTHROPIC_BASE_URL"] == baseURL {
			return true
		}
	}
	return false
}

func hasDuplicateMCP(items []model.MCPServer, transport, command string, args []string, env map[string]string) bool {
	for _, item := range items {
		if item.Transport == transport && item.Command == command && equalStrings(item.Args, args) && equalStringMap(item.Env, env) {
			return true
		}
	}
	return false
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func equalStringMap(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}
