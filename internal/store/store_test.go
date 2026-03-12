package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"mcfg/internal/model"
	"mcfg/internal/store"
)

func TestInitStore_CreatesDirectories(t *testing.T) {
	s := store.New(t.TempDir())

	created, err := s.Init(context.Background())
	require.NoError(t, err)
	require.True(t, created)

	require.DirExists(t, s.ConfigDir())
	require.DirExists(t, s.BackupsDir())
}

func TestInitStore_CreatesEmptyConfig(t *testing.T) {
	s := store.New(t.TempDir())

	_, err := s.Init(context.Background())
	require.NoError(t, err)

	cfg, err := s.Load(context.Background())
	require.NoError(t, err)
	require.Equal(t, model.NewConfigRoot(), cfg)
}

func TestInitStore_FilePermissions(t *testing.T) {
	s := store.New(t.TempDir())

	_, err := s.Init(context.Background())
	require.NoError(t, err)

	info, err := os.Stat(s.ConfigPath())
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestLoad_CorruptedJSON(t *testing.T) {
	s := store.New(t.TempDir())
	require.NoError(t, os.MkdirAll(s.ConfigDir(), 0o700))
	require.NoError(t, os.WriteFile(s.ConfigPath(), []byte("{"), 0o600))

	_, err := s.Load(context.Background())
	require.Error(t, err)
}

func TestSave_RoundTrip(t *testing.T) {
	s := store.New(t.TempDir())
	_, err := s.Init(context.Background())
	require.NoError(t, err)

	cfg := model.NewConfigRoot()
	cfg.Models = append(cfg.Models, model.ModelProfile{
		ID:        "01HQXBF7M6SJHMR6G32P5D1K7Y",
		Name:      "Claude Sonnet",
		Env:       map[string]string{"ANTHROPIC_MODEL": "claude-sonnet-4-0"},
		Source:    model.SourceManual,
		CreatedAt: "2026-03-10T10:00:00Z",
		UpdatedAt: "2026-03-10T10:00:00Z",
	})

	require.NoError(t, s.Save(context.Background(), cfg))

	loaded, err := s.Load(context.Background())
	require.NoError(t, err)
	require.Equal(t, cfg, loaded)
}

func TestSave_AtomicWriteLeavesNoTempFile(t *testing.T) {
	s := store.New(t.TempDir())
	_, err := s.Init(context.Background())
	require.NoError(t, err)

	require.NoError(t, s.Save(context.Background(), model.NewConfigRoot()))

	matches, err := filepath.Glob(filepath.Join(s.ConfigDir(), "config-*.tmp"))
	require.NoError(t, err)
	require.Empty(t, matches)
}
