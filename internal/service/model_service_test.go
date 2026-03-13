package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"mcfg/internal/model"
	"mcfg/internal/service"
)

func TestModelAdd_Success(t *testing.T) {
	store := &memoryStore{cfg: model.NewConfigRoot()}
	svc := service.NewModelService(store, fixedClock{}, stubIDGen{id: "01HQXBF7M6SJHMR6G32P5D1K7Y"})

	profile, err := svc.Add(context.Background(), service.ModelAddInput{
		Name:      "Claude Sonnet",
		AuthToken: "token",
		BaseURL:   "https://example.com",
		Model:     "claude-sonnet-4-0",
		Env:       map[string]string{"X_TRACE": "1"},
	})
	require.NoError(t, err)
	require.Equal(t, "01HQXBF7M6SJHMR6G32P5D1K7Y", profile.ID)
	require.Equal(t, model.SourceManual, profile.Source)
	require.Equal(t, "1", profile.Env["X_TRACE"])
}

func TestModelAdd_DuplicateName_Fails(t *testing.T) {
	cfg := model.NewConfigRoot()
	cfg.Models = append(cfg.Models, validModel("01HQXBF7M6SJHMR6G32P5D1K7Y"))

	store := &memoryStore{cfg: cfg}
	svc := service.NewModelService(store, fixedClock{}, stubIDGen{id: "01HQXBF7M6SJHMR6G32P5D1K7Z"})

	_, err := svc.Add(context.Background(), service.ModelAddInput{
		Name:      " claude sonnet ",
		AuthToken: "token-2",
		BaseURL:   "https://example.org",
		Model:     "claude-sonnet-4-1",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}

func TestModelEdit_ReplaceEnv(t *testing.T) {
	cfg := model.NewConfigRoot()
	cfg.Models = append(cfg.Models, model.ModelProfile{
		ID:   "01HQXBF7M6SJHMR6G32P5D1K7Y",
		Name: "Claude Sonnet",
		Env: map[string]string{
			"ANTHROPIC_AUTH_TOKEN": "token",
			"ANTHROPIC_BASE_URL":   "https://example.com",
			"ANTHROPIC_MODEL":      "claude",
			"X_OLD":                "1",
		},
		Source:    model.SourceManual,
		CreatedAt: "2026-03-10T10:00:00Z",
		UpdatedAt: "2026-03-10T10:00:00Z",
	})

	store := &memoryStore{cfg: cfg}
	svc := service.NewModelService(store, fixedClock{}, stubIDGen{})

	updated, err := svc.Edit(context.Background(), "01HQXBF7", service.ModelEditInput{
		Env:        map[string]string{"X_NEW": "2"},
		ReplaceEnv: true,
	})
	require.NoError(t, err)
	require.NotContains(t, updated.Env, "X_OLD")
	require.Equal(t, "2", updated.Env["X_NEW"])
}

func TestModelEdit_RenameToDuplicate_Fails(t *testing.T) {
	cfg := model.NewConfigRoot()
	cfg.Models = append(cfg.Models,
		validModel("01HQXBF7M6SJHMR6G32P5D1K7Y"),
		model.ModelProfile{
			ID:   "01HQXBF7M6SJHMR6G32P5D1K7Z",
			Name: "Claude Opus",
			Env: map[string]string{
				"ANTHROPIC_AUTH_TOKEN": "token",
				"ANTHROPIC_BASE_URL":   "https://example.com",
				"ANTHROPIC_MODEL":      "claude-opus",
			},
			Source:    model.SourceManual,
			CreatedAt: "2026-03-10T10:00:00Z",
			UpdatedAt: "2026-03-10T10:00:00Z",
		},
	)

	store := &memoryStore{cfg: cfg}
	svc := service.NewModelService(store, fixedClock{}, stubIDGen{})

	name := "CLAUDE SONNET"
	_, err := svc.Edit(context.Background(), "01HQXBF7M6SJHMR6G32P5D1K7Z", service.ModelEditInput{Name: &name})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}

func TestModelRemove_Bound_NoForce(t *testing.T) {
	cfg := model.NewConfigRoot()
	cfg.Models = append(cfg.Models, validModel("01HQXBF7M6SJHMR6G32P5D1K7Y"))
	cfg.ClaudeBinding.CurrentModelID = cfg.Models[0].ID

	store := &memoryStore{cfg: cfg}
	svc := service.NewModelService(store, fixedClock{}, stubIDGen{})

	err := svc.Remove(context.Background(), "01HQXBF7", false)
	require.Error(t, err)
}

func TestModelUse_Success(t *testing.T) {
	cfg := model.NewConfigRoot()
	cfg.Models = append(cfg.Models, validModel("01HQXBF7M6SJHMR6G32P5D1K7Y"))

	store := &memoryStore{cfg: cfg}
	svc := service.NewModelService(store, fixedClock{}, stubIDGen{})

	used, err := svc.Use(context.Background(), "01HQXBF7")
	require.NoError(t, err)
	require.Equal(t, cfg.Models[0].ID, used.ID)
	require.Equal(t, cfg.Models[0].ID, store.cfg.ClaudeBinding.CurrentModelID)
}

type memoryStore struct {
	cfg model.ConfigRoot
}

func (m *memoryStore) Load(context.Context) (model.ConfigRoot, error) {
	return m.cfg, nil
}

func (m *memoryStore) Save(_ context.Context, cfg model.ConfigRoot) error {
	m.cfg = cfg
	return nil
}

type fixedClock struct{}

func (fixedClock) Now() time.Time {
	return time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)
}

type plusOneSecondClock struct{}

func (plusOneSecondClock) Now() time.Time {
	return time.Date(2026, 3, 10, 10, 0, 1, 0, time.UTC)
}

type stubIDGen struct {
	id string
}

func (s stubIDGen) New() (string, error) {
	return s.id, nil
}

func validModel(id string) model.ModelProfile {
	return model.ModelProfile{
		ID:   id,
		Name: "Claude Sonnet",
		Env: map[string]string{
			"ANTHROPIC_AUTH_TOKEN": "token",
			"ANTHROPIC_BASE_URL":   "https://example.com",
			"ANTHROPIC_MODEL":      "claude",
		},
		Source:    model.SourceManual,
		CreatedAt: "2026-03-10T10:00:00Z",
		UpdatedAt: "2026-03-10T10:00:00Z",
	}
}
