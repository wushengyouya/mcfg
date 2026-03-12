package scanner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"mcfg/internal/model"
	"mcfg/internal/scanner"
)

func TestScan_SettingsExists_ExtractModel(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "token",
    "ANTHROPIC_BASE_URL": "https://example.com",
    "ANTHROPIC_MODEL": "claude-sonnet-4-0",
    "X_EXTRA": "1"
  }
}`), 0o600))

	result, err := scanner.New(home, now, scannerStubID("01HQXBF7M6SJHMR6G32P5D1K7Y")).Scan(context.Background(), model.NewConfigRoot())
	require.NoError(t, err)
	require.Len(t, result.Models, 1)
	require.Equal(t, model.SourceImported, result.Models[0].Source)
	require.Equal(t, "1", result.Models[0].Env["X_EXTRA"])
}

func TestScan_ClaudeJSON_ExtractMCPs(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{
  "`+home+`": {
    "mcpServers": {
      "github": {
        "type": "stdio",
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-github"],
        "env": {"GITHUB_TOKEN": "x"}
      }
    }
  }
}`), 0o600))

	result, err := scanner.New(home, now, scannerStubID("01HQXBG84ESB7XJQ9WAAYH54AM")).Scan(context.Background(), model.NewConfigRoot())
	require.NoError(t, err)
	require.Len(t, result.MCPServers, 1)
	require.Equal(t, "github", result.MCPServers[0].Name)
}

func TestScan_BothCorrupted(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte("{"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{"), 0o600))

	result, err := scanner.New(home, now, scannerStubID("01HQXBF7M6SJHMR6G32P5D1K7Y")).Scan(context.Background(), model.NewConfigRoot())
	require.NoError(t, err)
	require.Empty(t, result.Models)
	require.Empty(t, result.MCPServers)
	require.Len(t, result.Warnings, 2)
}

func now() string {
	return time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC).Format(time.RFC3339)
}

type scannerStubID string

func (s scannerStubID) New() (string, error) {
	return string(s), nil
}
