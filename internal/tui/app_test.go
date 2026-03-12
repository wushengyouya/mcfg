package tui_test

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"mcfg/internal/model"
	"mcfg/internal/tui"
)

func TestTUI_DefaultPage_Overview(t *testing.T) {
	app := tui.New(tui.Snapshot{
		CurrentModelName: "Claude Sonnet",
		EnabledMCPCount:  2,
		LastSyncResult:   "success",
	}, nil)

	view := app.View()
	require.Contains(t, view, "Overview")
	require.Contains(t, view, "Current model: Claude Sonnet")
}

func TestNav_SwitchToModels(t *testing.T) {
	app := tui.New(tui.Snapshot{}, nil)

	updated, _ := app.Update(key("j"))
	view := updated.(tui.App).View()
	require.Contains(t, view, "> Models")
}

func TestNav_WrapAround(t *testing.T) {
	app := tui.New(tui.Snapshot{}, nil)

	updated, _ := app.Update(key("k"))
	view := updated.(tui.App).View()
	require.Contains(t, view, "> Backups")
}

func TestOverview_NoModel(t *testing.T) {
	app := tui.New(tui.Snapshot{EnabledMCPCount: 1}, nil)

	view := app.View()
	require.Contains(t, view, "Current model: (none)")
}

func TestModelsPage_ListDisplays(t *testing.T) {
	app := tui.New(tui.Snapshot{
		CurrentModelID:   "01HQXBF7M6SJHMR6G32P5D1K7Y",
		CurrentModelName: "Claude Sonnet",
		Models: []model.ModelProfile{
			{ID: "01HQXBF7M6SJHMR6G32P5D1K7Y", Name: "Claude Sonnet"},
			{ID: "01HQXBG84ESB7XJQ9WAAYH54AM", Name: "GPT-4.1"},
		},
	}, nil)

	updated, _ := app.Update(key("j"))
	view := updated.(tui.App).View()
	require.Contains(t, view, "Claude Sonnet [active]")
	require.Contains(t, view, "GPT-4.1")
}

func TestModelsPage_AddModel(t *testing.T) {
	controller := &stubController{
		snapshot: tui.Snapshot{
			Models: []model.ModelProfile{{ID: "01", Name: "Claude Sonnet"}},
		},
	}
	app := tui.New(tui.Snapshot{}, controller)

	updated, _ := app.Update(key("j"))
	updated, _ = updated.(tui.App).Update(key("a"))
	for _, key := range []string{"D", "e", "m", "o", "enter", "h", "t", "t", "p", "s", ":", "/", "/", "e", ".", "c", "o", "m", "enter", "c", "l", "a", "u", "d", "e", "enter", "t", "o", "k", "e", "n", "enter", "n", "o", "t", "e", "enter"} {
		updated, _ = updated.(tui.App).Update(keyMsg(key))
	}

	view := updated.(tui.App).View()
	require.Contains(t, view, "Demo")
	require.Contains(t, view, "Info: model added")
}

func TestModelsPage_EditModel(t *testing.T) {
	controller := &stubController{
		snapshot: tui.Snapshot{
			Models: []model.ModelProfile{{ID: "01", Name: "Renamed", Env: map[string]string{"ANTHROPIC_BASE_URL": "https://e.com", "ANTHROPIC_MODEL": "claude", "ANTHROPIC_AUTH_TOKEN": "token"}}},
		},
	}
	app := tui.New(tui.Snapshot{
		Models: []model.ModelProfile{{ID: "01", Name: "Old", Env: map[string]string{"ANTHROPIC_BASE_URL": "https://e.com", "ANTHROPIC_MODEL": "claude", "ANTHROPIC_AUTH_TOKEN": "token"}}},
	}, controller)

	updated, _ := app.Update(key("j"))
	updated, _ = updated.(tui.App).Update(key("e"))
	for range 3 {
		updated, _ = updated.(tui.App).Update(key("backspace"))
	}
	for _, key := range []string{"R", "e", "n", "a", "m", "e", "d", "enter", "enter", "enter", "enter", "enter"} {
		updated, _ = updated.(tui.App).Update(keyMsg(key))
	}

	view := updated.(tui.App).View()
	require.Contains(t, view, "Renamed")
	require.Contains(t, view, "Info: model updated")
}

func TestModelsPage_DeleteModel(t *testing.T) {
	controller := &stubController{
		snapshot: tui.Snapshot{
			Models: []model.ModelProfile{},
		},
	}
	app := tui.New(tui.Snapshot{
		Models: []model.ModelProfile{{ID: "01", Name: "Old"}},
	}, controller)

	updated, _ := app.Update(key("j"))
	updated, _ = updated.(tui.App).Update(key("d"))
	updated, _ = updated.(tui.App).Update(key("enter"))

	view := updated.(tui.App).View()
	require.Contains(t, view, "No models configured.")
	require.Contains(t, view, "Info: model removed")
}

func TestModelsPage_UseModel(t *testing.T) {
	controller := &stubController{
		snapshot: tui.Snapshot{
			CurrentModelID:   "02",
			CurrentModelName: "GPT-4.1",
			Models: []model.ModelProfile{
				{ID: "01", Name: "Claude Sonnet"},
				{ID: "02", Name: "GPT-4.1"},
			},
		},
	}
	app := tui.New(tui.Snapshot{
		CurrentModelID:   "01",
		CurrentModelName: "Claude Sonnet",
		Models: []model.ModelProfile{
			{ID: "01", Name: "Claude Sonnet"},
			{ID: "02", Name: "GPT-4.1"},
		},
	}, controller)

	updated, _ := app.Update(key("j"))
	updated, _ = updated.(tui.App).Update(key("j"))
	updated, _ = updated.(tui.App).Update(key("u"))

	view := updated.(tui.App).View()
	require.Contains(t, view, "GPT-4.1 [active]")
	require.Contains(t, view, "Info: model switched")
}

func TestMCPPage_AddEditDelete(t *testing.T) {
	controller := &stubController{
		snapshot: tui.Snapshot{
			MCPServers: []model.MCPServer{{ID: "m1", Name: "git", Command: "git-cmd"}},
		},
	}
	app := tui.New(tui.Snapshot{}, controller)

	updated, _ := app.Update(key("j"))
	updated, _ = updated.(tui.App).Update(key("l"))
	updated, _ = updated.(tui.App).Update(key("a"))
	for _, key := range []string{"g", "i", "t", "enter", "n", "p", "x", "enter", "-", "y", ",", "s", "e", "r", "v", "e", "r", "enter", "K", "=", "V", "enter", "d", "e", "s", "c", "enter"} {
		updated, _ = updated.(tui.App).Update(keyMsg(key))
	}
	view := updated.(tui.App).View()
	require.Contains(t, view, "git")
	require.Contains(t, view, "Info: mcp added")

	controller.snapshot = tui.Snapshot{
		MCPServers: []model.MCPServer{{ID: "m1", Name: "git2", Command: "npx"}},
	}
	updated, _ = updated.(tui.App).Update(key("e"))
	for _, key := range []string{"backspace", "backspace", "backspace", "g", "i", "t", "2", "enter", "enter", "enter", "enter", "enter"} {
		updated, _ = updated.(tui.App).Update(keyMsg(key))
	}
	view = updated.(tui.App).View()
	require.Contains(t, view, "git2")
	require.Contains(t, view, "Info: mcp updated")

	controller.snapshot = tui.Snapshot{}
	updated, _ = updated.(tui.App).Update(key("d"))
	updated, _ = updated.(tui.App).Update(key("enter"))
	view = updated.(tui.App).View()
	require.Contains(t, view, "No MCP servers configured.")
	require.Contains(t, view, "Info: mcp removed")
}

func TestMCPPage_ToggleEnable(t *testing.T) {
	controller := &stubController{
		snapshot: tui.Snapshot{
			EnabledMCPIDs:   []string{"m1"},
			EnabledMCPCount: 1,
			MCPServers: []model.MCPServer{
				{ID: "m1", Name: "filesystem", Command: "npx"},
			},
		},
	}
	app := tui.New(tui.Snapshot{
		MCPServers: []model.MCPServer{
			{ID: "m1", Name: "filesystem", Command: "npx"},
		},
	}, controller)

	updated, _ := app.Update(key("j"))
	updated, _ = updated.(tui.App).Update(key("l"))
	updated, _ = updated.(tui.App).Update(key(" "))

	view := updated.(tui.App).View()
	require.Contains(t, view, "[enabled]")
	require.Contains(t, view, "Info: mcp toggled")
}

func TestSyncPreview_Confirm(t *testing.T) {
	controller := &stubController{
		drift: []string{"env.ANTHROPIC_MODEL"},
		snapshot: tui.Snapshot{
			LastSyncResult: "success",
		},
		appliedDrift: []string{},
	}
	app := tui.New(tui.Snapshot{}, controller)

	updated, _ := app.Update(key("s"))
	updated, _ = updated.(tui.App).Update(key("enter"))
	updated, _ = updated.(tui.App).Update(key("enter"))

	view := updated.(tui.App).View()
	require.Contains(t, view, "Info: sync completed")
	require.Contains(t, view, "No drift loaded")
}

func TestSyncPreview_Cancel(t *testing.T) {
	controller := &stubController{drift: []string{"env.ANTHROPIC_MODEL"}}
	app := tui.New(tui.Snapshot{}, controller)

	updated, _ := app.Update(key("s"))
	updated, _ = updated.(tui.App).Update(key("enter"))
	updated, _ = updated.(tui.App).Update(key("esc"))

	view := updated.(tui.App).View()
	require.Contains(t, view, "Info: cancelled")
	require.Contains(t, view, "env.ANTHROPIC_MODEL")
}

func TestBackupsPage_RestoreConfirm(t *testing.T) {
	controller := &stubController{
		backups: []tui.BackupItem{
			{ID: "b1", CreatedAt: "2026-03-10T10:00:00Z", Reason: "sync"},
		},
		restoredBackups: []tui.BackupItem{
			{ID: "b1", CreatedAt: "2026-03-10T10:00:00Z", Reason: "sync"},
		},
	}
	app := tui.New(tui.Snapshot{}, controller)

	updated, _ := app.Update(key("k"))
	updated, _ = updated.(tui.App).Update(key("enter"))
	updated, _ = updated.(tui.App).Update(key("enter"))

	view := updated.(tui.App).View()
	require.Contains(t, view, "Info: backup restored")
}

func TestBackupsPage_RestoreCancel(t *testing.T) {
	controller := &stubController{
		backups: []tui.BackupItem{
			{ID: "b1", CreatedAt: "2026-03-10T10:00:00Z", Reason: "sync"},
		},
	}
	app := tui.New(tui.Snapshot{}, controller)

	updated, _ := app.Update(key("k"))
	updated, _ = updated.(tui.App).Update(key("enter"))
	updated, _ = updated.(tui.App).Update(key("esc"))

	view := updated.(tui.App).View()
	require.Contains(t, view, "Info: cancelled")
}

func TestTUI_Quit(t *testing.T) {
	app := tui.New(tui.Snapshot{}, nil)

	updated, cmd := app.Update(key("q"))
	require.NotNil(t, cmd)
	require.Equal(t, "Bye.", updated.(tui.App).View())
}

type stubController struct {
	snapshot        tui.Snapshot
	drift           []string
	appliedDrift    []string
	backups         []tui.BackupItem
	restoredBackups []tui.BackupItem
	nextID          int
}

func (s *stubController) Refresh() (tui.Snapshot, error)         { return s.snapshot, nil }
func (s *stubController) UseModel(string) (tui.Snapshot, error)  { return s.snapshot, nil }
func (s *stubController) ToggleMCP(string) (tui.Snapshot, error) { return s.snapshot, nil }
func (s *stubController) SyncPreview() ([]string, error)         { return s.drift, nil }
func (s *stubController) ListBackups() ([]tui.BackupItem, error) { return s.backups, nil }
func (s *stubController) SyncApply() (tui.Snapshot, []string, error) {
	return s.snapshot, s.appliedDrift, nil
}
func (s *stubController) RestoreBackup(string) (tui.Snapshot, []tui.BackupItem, error) {
	if s.restoredBackups != nil {
		return s.snapshot, s.restoredBackups, nil
	}
	return s.snapshot, s.backups, nil
}
func (s *stubController) AddModel(input tui.ModelFormInput) (tui.Snapshot, error) {
	s.nextID++
	s.snapshot.Models = append(s.snapshot.Models, model.ModelProfile{ID: idFor(s.nextID), Name: input.Name})
	return s.snapshot, nil
}
func (s *stubController) EditModel(id string, input tui.ModelFormInput) (tui.Snapshot, error) {
	for i := range s.snapshot.Models {
		if s.snapshot.Models[i].ID == id {
			s.snapshot.Models[i].Name = input.Name
		}
	}
	return s.snapshot, nil
}
func (s *stubController) RemoveModel(id string) (tui.Snapshot, error) {
	filtered := s.snapshot.Models[:0]
	for _, item := range s.snapshot.Models {
		if item.ID != id {
			filtered = append(filtered, item)
		}
	}
	s.snapshot.Models = filtered
	return s.snapshot, nil
}
func (s *stubController) AddMCP(input tui.MCPFormInput) (tui.Snapshot, error) {
	s.nextID++
	s.snapshot.MCPServers = append(s.snapshot.MCPServers, model.MCPServer{ID: idFor(s.nextID), Name: input.Name, Command: input.Command})
	return s.snapshot, nil
}
func (s *stubController) EditMCP(id string, input tui.MCPFormInput) (tui.Snapshot, error) {
	for i := range s.snapshot.MCPServers {
		if s.snapshot.MCPServers[i].ID == id {
			s.snapshot.MCPServers[i].Name = input.Name
			s.snapshot.MCPServers[i].Command = input.Command
		}
	}
	return s.snapshot, nil
}
func (s *stubController) RemoveMCP(id string) (tui.Snapshot, error) {
	filtered := s.snapshot.MCPServers[:0]
	for _, item := range s.snapshot.MCPServers {
		if item.ID != id {
			filtered = append(filtered, item)
		}
	}
	s.snapshot.MCPServers = filtered
	return s.snapshot, nil
}

func key(value string) tea.KeyMsg { return keyMsg(value) }

func keyMsg(value string) tea.KeyMsg {
	switch value {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
	}
}

func idFor(index int) string {
	return fmt.Sprintf("id-%d", index)
}
