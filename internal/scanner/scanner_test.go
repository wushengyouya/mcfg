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

func TestScan_RealPath(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	claudeJSONPath := filepath.Join(home, ".claude.json")
	result, err := scanner.New(home, now, scannerStubID("01HQXBF7M6SJHMR6G32P5D1K7Y")).Scan(context.Background(), model.NewConfigRoot())
	require.NoError(t, err)
	require.Len(t, result.Models, 1)
	require.Equal(t, model.SourceImported, result.Models[0].Source)
	require.NotEmpty(t, result.Models[0].Env["ANTHROPIC_AUTH_TOKEN"])
	require.NotEmpty(t, result.Models[0].Env["ANTHROPIC_BASE_URL"])
	require.NotEmpty(t, result.Models[0].Env["ANTHROPIC_MODEL"])

	t.Logf("home=%s", home)
	t.Logf("settings path=%s", settingsPath)
	t.Logf("claude json path=%s", claudeJSONPath)
	t.Logf("imported model=%s base_url=%s", result.Models[0].Env["ANTHROPIC_MODEL"], result.Models[0].Env["ANTHROPIC_BASE_URL"])
}

func TestScan_ClaudeJSON_ExtractMCPs(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{
  "numStartups": 82,
  "`+home+`": {
    "mcpServers": {
      "legacy": {
        "type": "stdio",
        "command": "node",
        "args": ["legacy.js"],
        "env": {"LEGACY_TOKEN": "ignored"}
      }
    }
  },
  "projects": {
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
  }
}`), 0o600))

	result, err := scanner.New(home, now, scannerStubID("01HQXBG84ESB7XJQ9WAAYH54AM")).Scan(context.Background(), model.NewConfigRoot())
	require.NoError(t, err)
	require.Len(t, result.MCPServers, 1)
	require.Equal(t, "github", result.MCPServers[0].Name)
	require.Equal(t, "npx", result.MCPServers[0].Command)
	require.Equal(t, "x", result.MCPServers[0].Env["GITHUB_TOKEN"])
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

func TestScan_SettingsWithRealClaudeStructure_ExtractModel(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{
  "effortLevel": "high",
  "enabledPlugins": {
    "document-skills@anthropic-agent-skills": true,
    "github@claude-plugins-official": true
  },
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "token",
    "ANTHROPIC_BASE_URL": "https://api.minimaxi.com/anthropic",
    "ANTHROPIC_MODEL": "MiniMax-M2.5"
  },
  "hooks": {
    "Notification": [
      {
        "hooks": [
          {
            "async": true,
            "command": "/home/mikasa/.claude/hooks/peon-ping/peon.sh",
            "timeout": 10,
            "type": "command"
          }
        ],
        "matcher": ""
      }
    ]
  },
  "language": "中文",
  "model": "opus",
  "skipDangerousModePermissionPrompt": true
}`), 0o600))

	result, err := scanner.New(home, now, scannerStubID("01HQXBF7M6SJHMR6G32P5D1K7Y")).Scan(context.Background(), model.NewConfigRoot())
	require.NoError(t, err)
	require.Len(t, result.Models, 1)
	require.Equal(t, "MiniMax-M2.5", result.Models[0].Name)
	require.Equal(t, "https://api.minimaxi.com/anthropic", result.Models[0].Env["ANTHROPIC_BASE_URL"])
	require.Equal(t, "token", result.Models[0].Env["ANTHROPIC_AUTH_TOKEN"])
}

func now() string {
	return time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC).Format(time.RFC3339)
}

type scannerStubID string

func (s scannerStubID) New() (string, error) {
	return string(s), nil
}
