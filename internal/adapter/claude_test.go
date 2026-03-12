package adapter_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"mcfg/internal/adapter"
	"mcfg/internal/model"
)

func TestAdapter_ModelToSettingsEnv(t *testing.T) {
	rendered, err := adapter.Claude{HomeDir: "/tmp/home"}.RenderSettings(nil, &model.ModelProfile{
		Env: map[string]string{
			"ANTHROPIC_AUTH_TOKEN": "token",
			"ANTHROPIC_BASE_URL":   "https://example.com",
			"ANTHROPIC_MODEL":      "claude",
		},
	})
	require.NoError(t, err)
	values, err := adapter.SettingsManagedValues(rendered)
	require.NoError(t, err)
	require.Equal(t, "claude", values["env.ANTHROPIC_MODEL"])
}

func TestAdapter_PreservesUnmanagedFields(t *testing.T) {
	rendered, err := adapter.Claude{HomeDir: "/tmp/home"}.RenderSettings([]byte(`{"theme":"dark","env":{"EXTRA":"1"}}`), &model.ModelProfile{
		Env: map[string]string{
			"ANTHROPIC_AUTH_TOKEN": "token",
			"ANTHROPIC_BASE_URL":   "https://example.com",
			"ANTHROPIC_MODEL":      "claude",
		},
	})
	require.NoError(t, err)
	require.Contains(t, string(rendered), `"theme": "dark"`)
	require.Contains(t, string(rendered), `"EXTRA": "1"`)
}

func TestAdapter_MCPsToClaudeJSON(t *testing.T) {
	rendered, err := adapter.Claude{HomeDir: "/tmp/home"}.RenderClaudeJSON(nil, []model.MCPServer{{
		Name:      "github",
		Transport: "stdio",
		Command:   "npx",
		Args:      []string{"-y"},
		Env:       map[string]string{"GITHUB_TOKEN": "x"},
	}})
	require.NoError(t, err)
	require.Contains(t, string(rendered), `"github"`)
	require.Contains(t, string(rendered), `"mcpServers"`)
}

func TestAdapter_PreservesOtherPathNodes(t *testing.T) {
	rendered, err := adapter.Claude{HomeDir: "/tmp/home"}.RenderClaudeJSON([]byte(`{
  "/other": {"mcpServers":{"x":{"type":"stdio","command":"cmd"}}},
  "/tmp/home": {"allowedTools":["Read"]}
}`), nil)
	require.NoError(t, err)
	require.Contains(t, string(rendered), `"/other"`)
	require.Contains(t, string(rendered), `"allowedTools"`)
}
