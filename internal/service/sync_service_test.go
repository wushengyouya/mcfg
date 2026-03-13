package service_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"mcfg/internal/exitcode"
	"mcfg/internal/model"
	"mcfg/internal/service"
	"mcfg/internal/store"
)

func TestSync_Success_WritesSettings(t *testing.T) {
	home := t.TempDir()
	st := store.New(home)
	_, err := st.Init(context.Background())
	require.NoError(t, err)

	cfg := model.NewConfigRoot()
	cfg.Models = append(cfg.Models, validModel("01HQXBF7M6SJHMR6G32P5D1K7Y"))
	cfg.ClaudeBinding.CurrentModelID = cfg.Models[0].ID
	require.NoError(t, st.Save(context.Background(), cfg))

	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{"EXTRA":"1"}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), projectScopedClaudeJSON(home, `{}`), 0o600))

	svc := service.NewSyncService(st, home, st.BackupsDir(), fixedClock{}, stubIDGen{id: "01HQXBG84ESB7XJQ9WAAYH54AM"})
	result, err := svc.Sync(context.Background(), service.SyncOptions{})
	require.NoError(t, err)
	require.Contains(t, result.ChangedPaths, "env.ANTHROPIC_MODEL")

	settingsData, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	require.NoError(t, err)
	require.Contains(t, string(settingsData), `"ANTHROPIC_MODEL": "claude"`)

	loaded, err := st.Load(context.Background())
	require.NoError(t, err)
	require.Equal(t, "success", loaded.ClaudeBinding.LastSyncResult)
	require.Len(t, loaded.BackupIndex, 1)
}

func TestSync_InitTarget_CreatesFiles(t *testing.T) {
	home := t.TempDir()
	st := store.New(home)
	_, err := st.Init(context.Background())
	require.NoError(t, err)

	cfg := model.NewConfigRoot()
	cfg.Models = append(cfg.Models, validModel("01HQXBF7M6SJHMR6G32P5D1K7Y"))
	cfg.ClaudeBinding.CurrentModelID = cfg.Models[0].ID
	require.NoError(t, st.Save(context.Background(), cfg))

	svc := service.NewSyncService(st, home, st.BackupsDir(), fixedClock{}, stubIDGen{id: "01HQXBG84ESB7XJQ9WAAYH54AM"})
	_, err = svc.Sync(context.Background(), service.SyncOptions{InitTarget: true})
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(home, ".claude", "settings.json"))
	require.FileExists(t, filepath.Join(home, ".claude.json"))
	claudeJSONData, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	require.NoError(t, err)
	require.JSONEq(t, string(projectScopedClaudeJSON(home, `{}`)), string(claudeJSONData))
}

func TestSync_DryRun_NoWrite(t *testing.T) {
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
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), projectScopedClaudeJSON(home, `{}`), 0o600))

	svc := service.NewSyncService(st, home, st.BackupsDir(), fixedClock{}, stubIDGen{id: "01HQXBG84ESB7XJQ9WAAYH54AM"})
	_, err = svc.Sync(context.Background(), service.SyncOptions{DryRun: true})
	require.NoError(t, err)

	loaded, err := st.Load(context.Background())
	require.NoError(t, err)
	require.Empty(t, loaded.BackupIndex)
	require.Empty(t, loaded.ClaudeBinding.LastSyncResult)
}

func TestSync_WriteFail_Rollback(t *testing.T) {
	home := t.TempDir()
	st := store.New(home)
	_, err := st.Init(context.Background())
	require.NoError(t, err)

	cfg := model.NewConfigRoot()
	cfg.Models = append(cfg.Models, validModel("01HQXBF7M6SJHMR6G32P5D1K7Y"))
	cfg.ClaudeBinding.CurrentModelID = cfg.Models[0].ID
	require.NoError(t, st.Save(context.Background(), cfg))

	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	originalSettings := []byte(`{"env":{"ORIGINAL":"1"}}`)
	originalClaudeJSON := projectScopedClaudeJSON(home, `{"old":{"type":"stdio","command":"old"}}`)
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), originalSettings, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), originalClaudeJSON, 0o600))

	svc := service.NewSyncService(st, home, st.BackupsDir(), fixedClock{}, stubIDGen{id: "01HQXBG84ESB7XJQ9WAAYH54AM"})
	svc.SetTestHooks(service.SyncTestHooks{
		BeforeWriteClaudeJSON: func() error {
			return fmt.Errorf("%w: injected write failure", exitcode.ErrIO)
		},
	})

	_, err = svc.Sync(context.Background(), service.SyncOptions{})
	require.Error(t, err)

	settingsData, readErr := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	require.NoError(t, readErr)
	require.JSONEq(t, string(originalSettings), string(settingsData))

	claudeJSONData, readErr := os.ReadFile(filepath.Join(home, ".claude.json"))
	require.NoError(t, readErr)
	require.JSONEq(t, string(originalClaudeJSON), string(claudeJSONData))
}

func TestSync_ExternalModification_Detected(t *testing.T) {
	home := t.TempDir()
	st := store.New(home)
	_, err := st.Init(context.Background())
	require.NoError(t, err)

	cfg := model.NewConfigRoot()
	cfg.Models = append(cfg.Models, validModel("01HQXBF7M6SJHMR6G32P5D1K7Y"))
	cfg.ClaudeBinding.CurrentModelID = cfg.Models[0].ID
	require.NoError(t, st.Save(context.Background(), cfg))

	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{"ORIGINAL":"1"}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), projectScopedClaudeJSON(home, `{}`), 0o600))

	svc := service.NewSyncService(st, home, st.BackupsDir(), fixedClock{}, stubIDGen{id: "01HQXBG84ESB7XJQ9WAAYH54AM"})
	svc.SetTestHooks(service.SyncTestHooks{
		BeforeWriteSettings: func() error {
			return os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{"EXTERNAL":"1"}}`), 0o600)
		},
	})

	_, err = svc.Sync(context.Background(), service.SyncOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "external modification detected")

	settingsData, readErr := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	require.NoError(t, readErr)
	require.JSONEq(t, `{"env":{"EXTERNAL":"1"}}`, string(settingsData))
}
