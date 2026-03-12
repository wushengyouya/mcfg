package validator_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"mcfg/internal/model"
	"mcfg/internal/validator"
)

func TestValidate_ModelBaseURL_Invalid(t *testing.T) {
	err := validator.ValidateModelProfile(model.ModelProfile{
		Name:      "demo",
		Env:       map[string]string{"ANTHROPIC_AUTH_TOKEN": "token", "ANTHROPIC_MODEL": "model", "ANTHROPIC_BASE_URL": "ftp://example.com"},
		Source:    model.SourceManual,
		CreatedAt: "2026-03-10T10:00:00Z",
		UpdatedAt: "2026-03-10T10:00:00Z",
	})
	require.Error(t, err)
}

func TestValidate_MCPEnvKeyInvalid(t *testing.T) {
	err := validator.ValidateMCPServer(model.MCPServer{
		Name:      "git",
		Transport: "stdio",
		Command:   "npx",
		Env:       map[string]string{"bad-key": "x"},
		Source:    model.SourceManual,
		CreatedAt: "2026-03-10T10:00:00Z",
		UpdatedAt: "2026-03-10T10:00:00Z",
	})
	require.Error(t, err)
}

func TestValidate_ConsistencyCheck_BindingRefMissing(t *testing.T) {
	cfg := model.NewConfigRoot()
	cfg.ClaudeBinding.CurrentModelID = "missing"

	issues := validator.ValidateConfigRoot(cfg)
	require.Len(t, issues, 1)
	require.Equal(t, "missing_ref", issues[0].Code)
}

func TestValidate_AllPass(t *testing.T) {
	cfg := model.NewConfigRoot()
	cfg.Models = append(cfg.Models, model.ModelProfile{
		ID:   "01HQXBF7M6SJHMR6G32P5D1K7Y",
		Name: "demo",
		Env: map[string]string{
			"ANTHROPIC_AUTH_TOKEN": "token",
			"ANTHROPIC_BASE_URL":   "https://example.com",
			"ANTHROPIC_MODEL":      "claude",
		},
		Source:    model.SourceManual,
		CreatedAt: "2026-03-10T10:00:00Z",
		UpdatedAt: "2026-03-10T10:00:00Z",
	})
	cfg.MCPServers = append(cfg.MCPServers, model.MCPServer{
		ID:        "01HQXBG84ESB7XJQ9WAAYH54AM",
		Name:      "fs",
		Transport: "stdio",
		Command:   "npx",
		Source:    model.SourceManual,
		CreatedAt: "2026-03-10T10:00:00Z",
		UpdatedAt: "2026-03-10T10:00:00Z",
	})
	cfg.ClaudeBinding.CurrentModelID = cfg.Models[0].ID
	cfg.ClaudeBinding.EnabledMCPIDs = []string{cfg.MCPServers[0].ID}

	require.Empty(t, validator.ValidateConfigRoot(cfg))
}
