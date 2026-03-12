package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"mcfg/internal/model"
	"mcfg/internal/scanner"
	"mcfg/internal/service"
	"mcfg/internal/store"
)

func TestImport_NoConfigCenter(t *testing.T) {
	home := t.TempDir()
	svc := service.NewImportService(store.New(home), scanner.New(home, importNow, stubIDGen{id: "01HQXBF7M6SJHMR6G32P5D1K7Y"}))

	_, err := svc.Import(context.Background())
	require.Error(t, err)
}

func TestImport_NewModelsImported(t *testing.T) {
	home := t.TempDir()
	st := store.New(home)
	_, err := st.Init(context.Background())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{"ANTHROPIC_AUTH_TOKEN":"token","ANTHROPIC_BASE_URL":"https://example.com","ANTHROPIC_MODEL":"claude"}}`), 0o600))

	svc := service.NewImportService(st, scanner.New(home, importNow, stubIDGen{id: "01HQXBF7M6SJHMR6G32P5D1K7Y"}))
	summary, err := svc.Import(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, summary.ImportedModels)

	cfg, err := st.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, cfg.Models, 1)
	require.Equal(t, model.SourceImported, cfg.Models[0].Source)
}

func TestImport_RepeatedExecution(t *testing.T) {
	home := t.TempDir()
	st := store.New(home)
	_, err := st.Init(context.Background())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{"ANTHROPIC_AUTH_TOKEN":"token","ANTHROPIC_BASE_URL":"https://example.com","ANTHROPIC_MODEL":"claude"}}`), 0o600))

	svc := service.NewImportService(st, scanner.New(home, importNow, stubIDGen{id: "01HQXBF7M6SJHMR6G32P5D1K7Y"}))
	_, err = svc.Import(context.Background())
	require.NoError(t, err)
	summary, err := svc.Import(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, summary.ImportedModels)
	require.Equal(t, 1, summary.Skipped)
}

func importNow() string {
	return time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC).Format(time.RFC3339)
}
