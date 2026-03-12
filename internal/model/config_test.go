package model_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"mcfg/internal/model"
)

func TestConfigRoot_NewEmpty(t *testing.T) {
	cfg := model.NewConfigRoot()

	require.Equal(t, model.SchemaVersion, cfg.SchemaVersion)
	require.Empty(t, cfg.Models)
	require.Empty(t, cfg.MCPServers)
	require.Empty(t, cfg.ClaudeBinding.CurrentModelID)
	require.Empty(t, cfg.ClaudeBinding.EnabledMCPIDs)
	require.Empty(t, cfg.BackupIndex)
}

func TestConfigRoot_SerializeRoundTrip(t *testing.T) {
	cfg := model.NewConfigRoot()
	cfg.Models = append(cfg.Models, model.ModelProfile{
		ID:        "01HQXBF7M6SJHMR6G32P5D1K7Y",
		Name:      "Claude Sonnet",
		Env:       map[string]string{"ANTHROPIC_MODEL": "claude-sonnet-4-0"},
		Source:    model.SourceManual,
		CreatedAt: "2026-03-10T10:00:00Z",
		UpdatedAt: "2026-03-10T10:00:00Z",
	})

	data, err := cfg.Marshal()
	require.NoError(t, err)

	decoded, err := model.ParseConfigRoot(data)
	require.NoError(t, err)
	require.Equal(t, cfg, decoded)
}

func TestModelProfile_EnvPreservesUnknownKeys(t *testing.T) {
	raw := []byte(`{"id":"1","name":"n","env":{"ANTHROPIC_MODEL":"m","X_FOO":"bar"},"source":"manual","created_at":"2026-03-10T10:00:00Z","updated_at":"2026-03-10T10:00:00Z"}`)

	var profile model.ModelProfile
	err := json.Unmarshal(raw, &profile)
	require.NoError(t, err)
	require.Equal(t, "bar", profile.Env["X_FOO"])
}

func TestConfigRoot_DeserializeInvalidJSON(t *testing.T) {
	_, err := model.ParseConfigRoot([]byte("{"))
	require.Error(t, err)
}

func TestSource_OnlyManualOrImported(t *testing.T) {
	_, err := model.ParseConfigRoot([]byte(`{
		"schema_version":1,
		"models":[{"id":"1","name":"n","env":{},"source":"unknown","created_at":"2026-03-10T10:00:00Z","updated_at":"2026-03-10T10:00:00Z"}],
		"mcp_servers":[],
		"claude_binding":{"current_model_id":"","enabled_mcp_ids":[],"last_sync_at":"","last_sync_result":""},
		"backup_index":[]
	}`))
	require.Error(t, err)
}
