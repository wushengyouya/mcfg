package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"mcfg/internal/service"
	"mcfg/internal/store"
)

func TestBackupCreate_Success(t *testing.T) {
	home := t.TempDir()
	st := store.New(home)
	_, err := st.Init(context.Background())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), projectScopedClaudeJSON(home, `{}`), 0o600))

	svc := service.NewBackupService(st, home, st.BackupsDir(), fixedClock{}, stubIDGen{id: "01HQXBG84ESB7XJQ9WAAYH54AM"})
	meta, err := svc.Create(context.Background(), "manual")
	require.NoError(t, err)
	require.Equal(t, "manual", meta.Reason)
	require.Len(t, meta.Files, 2)

	cfg, err := st.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, cfg.BackupIndex, 1)
	require.FileExists(t, meta.Files[0].BackupPath)
}

func TestBackupCreate_TargetMissing_Fails(t *testing.T) {
	home := t.TempDir()
	st := store.New(home)
	_, err := st.Init(context.Background())
	require.NoError(t, err)

	svc := service.NewBackupService(st, home, st.BackupsDir(), fixedClock{}, stubIDGen{id: "01HQXBG84ESB7XJQ9WAAYH54AM"})
	_, err = svc.Create(context.Background(), "manual")
	require.Error(t, err)
}

func TestBackupRestore_Success(t *testing.T) {
	home := t.TempDir()
	st := store.New(home)
	_, err := st.Init(context.Background())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{"OLD":"1"}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), projectScopedClaudeJSON(home, `{"old":{"type":"stdio","command":"old"}}`), 0o600))

	svc := service.NewBackupService(st, home, st.BackupsDir(), fixedClock{}, stubIDGen{id: "01HQXBG84ESB7XJQ9WAAYH54AM"})
	meta, err := svc.Create(context.Background(), "manual")
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{"NEW":"2"}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), projectScopedClaudeJSON(home, `{}`), 0o600))

	_, err = svc.Restore(context.Background(), meta.ID[:8])
	require.NoError(t, err)

	settingsData, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	require.NoError(t, err)
	require.Contains(t, string(settingsData), `"OLD":"1"`)
}

func TestBackupPrune_Keep1(t *testing.T) {
	home := t.TempDir()
	st := store.New(home)
	_, err := st.Init(context.Background())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), projectScopedClaudeJSON(home, `{}`), 0o600))

	svc1 := service.NewBackupService(st, home, st.BackupsDir(), fixedClock{}, stubIDGen{id: "01HQXBG84ESB7XJQ9WAAYH54AM"})
	_, err = svc1.Create(context.Background(), "manual")
	require.NoError(t, err)
	svc2 := service.NewBackupService(st, home, st.BackupsDir(), plusOneSecondClock{}, stubIDGen{id: "01HQXBG84ESB7XJQ9WAAYH54AN"})
	_, err = svc2.Create(context.Background(), "manual")
	require.NoError(t, err)

	removed, err := svc2.Prune(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 1, removed)

	cfg, err := st.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, cfg.BackupIndex, 1)
	require.Equal(t, "01HQXBG84ESB7XJQ9WAAYH54AN", cfg.BackupIndex[0].ID)
}

func TestBackupRestore_ExternalModification_Detected(t *testing.T) {
	home := t.TempDir()
	st := store.New(home)
	_, err := st.Init(context.Background())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{"OLD":"1"}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), projectScopedClaudeJSON(home, `{}`), 0o600))

	svc := service.NewBackupService(st, home, st.BackupsDir(), fixedClock{}, stubIDGen{id: "01HQXBG84ESB7XJQ9WAAYH54AM"})
	meta, err := svc.Create(context.Background(), "manual")
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{"CURRENT":"1"}}`), 0o600))
	svc.SetTestHooks(service.BackupTestHooks{
		BeforeRestoreSettings: func() error {
			return os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{"EXTERNAL":"1"}}`), 0o600)
		},
	})

	_, err = svc.Restore(context.Background(), meta.ID[:8])
	require.Error(t, err)
	require.Contains(t, err.Error(), "external modification detected")

	settingsData, readErr := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	require.NoError(t, readErr)
	require.JSONEq(t, `{"env":{"EXTERNAL":"1"}}`, string(settingsData))
}
