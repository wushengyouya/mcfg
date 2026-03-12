package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"mcfg/internal/model"
	"mcfg/internal/service"
	"mcfg/internal/store"
)

func TestValidate_TargetSync_OutOfSync(t *testing.T) {
	home := t.TempDir()
	st := store.New(home)
	_, err := st.Init(context.Background())
	require.NoError(t, err)

	cfg := model.NewConfigRoot()
	cfg.Models = append(cfg.Models, validModel("01HQXBF7M6SJHMR6G32P5D1K7Y"))
	cfg.ClaudeBinding.CurrentModelID = cfg.Models[0].ID
	require.NoError(t, st.Save(context.Background(), cfg))

	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"`+home+`":{"mcpServers":{}}}`), 0o600))

	report, err := service.NewValidateService(st, home).Validate(context.Background())
	require.NoError(t, err)
	require.Equal(t, "out_of_sync", report.SyncStatus)
	require.Contains(t, report.Drift.ManagedPathsChanged, "env.ANTHROPIC_MODEL")
}

func TestValidate_TargetSync_InSync(t *testing.T) {
	home := t.TempDir()
	st := store.New(home)
	_, err := st.Init(context.Background())
	require.NoError(t, err)

	cfg := model.NewConfigRoot()
	cfg.Models = append(cfg.Models, validModel("01HQXBF7M6SJHMR6G32P5D1K7Y"))
	cfg.ClaudeBinding.CurrentModelID = cfg.Models[0].ID
	require.NoError(t, st.Save(context.Background(), cfg))

	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{"ANTHROPIC_AUTH_TOKEN":"token","ANTHROPIC_BASE_URL":"https://example.com","ANTHROPIC_MODEL":"claude"}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"`+home+`":{"mcpServers":{}}}`), 0o600))

	report, err := service.NewValidateService(st, home).Validate(context.Background())
	require.NoError(t, err)
	require.Equal(t, "in_sync", report.SyncStatus)
	require.Empty(t, report.Drift.ManagedPathsChanged)
}
