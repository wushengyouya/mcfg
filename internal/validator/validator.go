package validator

import (
	"fmt"
	"net/url"
	"regexp"
	"time"

	"mcfg/internal/exitcode"
	"mcfg/internal/model"
)

var envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Issue 表示一次校验发现的问题。
type Issue struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ValidateSource 校验来源字段是否合法。
func ValidateSource(source model.Source) error {
	if !source.Valid() {
		return fmt.Errorf("%w: invalid source %q", exitcode.ErrParam, source)
	}
	return nil
}

// ValidateRFC3339 校验时间字段是否符合 RFC3339 格式。
func ValidateRFC3339(path, value string) error {
	if value == "" {
		return nil
	}
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		return fmt.Errorf("%w: %s must be RFC3339", exitcode.ErrParam, path)
	}
	return nil
}

// ValidateModelProfile 校验单个模型配置是否合法。
func ValidateModelProfile(profile model.ModelProfile) error {
	if profile.Name == "" {
		return fmt.Errorf("%w: model name is required", exitcode.ErrParam)
	}
	if err := ValidateSource(profile.Source); err != nil {
		return err
	}
	if profile.Env["ANTHROPIC_AUTH_TOKEN"] == "" {
		return fmt.Errorf("%w: ANTHROPIC_AUTH_TOKEN is required", exitcode.ErrParam)
	}
	if profile.Env["ANTHROPIC_MODEL"] == "" {
		return fmt.Errorf("%w: ANTHROPIC_MODEL is required", exitcode.ErrParam)
	}
	baseURL := profile.Env["ANTHROPIC_BASE_URL"]
	if baseURL == "" {
		return fmt.Errorf("%w: ANTHROPIC_BASE_URL is required", exitcode.ErrParam)
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || !parsed.IsAbs() || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%w: ANTHROPIC_BASE_URL must be a valid http/https URL", exitcode.ErrParam)
	}
	if err := ValidateRFC3339("created_at", profile.CreatedAt); err != nil {
		return err
	}
	if err := ValidateRFC3339("updated_at", profile.UpdatedAt); err != nil {
		return err
	}
	return nil
}

// ValidateMCPServer 校验单个 MCP 服务配置是否合法。
func ValidateMCPServer(server model.MCPServer) error {
	if server.Name == "" {
		return fmt.Errorf("%w: mcp name is required", exitcode.ErrParam)
	}
	if err := ValidateSource(server.Source); err != nil {
		return err
	}
	if server.Transport != "stdio" {
		return fmt.Errorf("%w: transport must be stdio", exitcode.ErrParam)
	}
	if server.Command == "" {
		return fmt.Errorf("%w: command is required", exitcode.ErrParam)
	}
	for _, arg := range server.Args {
		if arg == "" {
			return fmt.Errorf("%w: args cannot contain empty items", exitcode.ErrParam)
		}
	}
	for key := range server.Env {
		if !envKeyPattern.MatchString(key) {
			return fmt.Errorf("%w: invalid env key %q", exitcode.ErrParam, key)
		}
	}
	if err := ValidateRFC3339("created_at", server.CreatedAt); err != nil {
		return err
	}
	if err := ValidateRFC3339("updated_at", server.UpdatedAt); err != nil {
		return err
	}
	return nil
}

// ValidateConfigRoot 校验整棵配置树并返回全部问题。
func ValidateConfigRoot(cfg model.ConfigRoot) []Issue {
	issues := []Issue{}
	if cfg.SchemaVersion != model.SchemaVersion {
		issues = append(issues, Issue{Path: "schema_version", Code: "schema_version", Message: "schema_version must be 1"})
	}

	modelIDs := map[string]struct{}{}
	for i, profile := range cfg.Models {
		if _, exists := modelIDs[profile.ID]; exists {
			issues = append(issues, Issue{Path: fmt.Sprintf("models[%d].id", i), Code: "duplicate_id", Message: "duplicate model id"})
		}
		modelIDs[profile.ID] = struct{}{}
		if err := ValidateModelProfile(profile); err != nil {
			issues = append(issues, Issue{Path: fmt.Sprintf("models[%d]", i), Code: "invalid_model", Message: err.Error()})
		}
	}

	mcpIDs := map[string]struct{}{}
	for i, server := range cfg.MCPServers {
		if _, exists := mcpIDs[server.ID]; exists {
			issues = append(issues, Issue{Path: fmt.Sprintf("mcp_servers[%d].id", i), Code: "duplicate_id", Message: "duplicate mcp id"})
		}
		mcpIDs[server.ID] = struct{}{}
		if err := ValidateMCPServer(server); err != nil {
			issues = append(issues, Issue{Path: fmt.Sprintf("mcp_servers[%d]", i), Code: "invalid_mcp", Message: err.Error()})
		}
	}

	if cfg.ClaudeBinding.CurrentModelID != "" {
		if _, exists := modelIDs[cfg.ClaudeBinding.CurrentModelID]; !exists {
			issues = append(issues, Issue{Path: "claude_binding.current_model_id", Code: "missing_ref", Message: "current_model_id does not reference an existing model"})
		}
	}

	enabledSet := map[string]struct{}{}
	for i, id := range cfg.ClaudeBinding.EnabledMCPIDs {
		if _, exists := mcpIDs[id]; !exists {
			issues = append(issues, Issue{Path: fmt.Sprintf("claude_binding.enabled_mcp_ids[%d]", i), Code: "missing_ref", Message: "enabled MCP does not reference an existing server"})
		}
		if _, exists := enabledSet[id]; exists {
			issues = append(issues, Issue{Path: fmt.Sprintf("claude_binding.enabled_mcp_ids[%d]", i), Code: "duplicate_ref", Message: "duplicate enabled MCP id"})
		}
		enabledSet[id] = struct{}{}
	}

	return issues
}
