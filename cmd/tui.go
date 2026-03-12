package cmd

import (
	"fmt"
	"slices"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"mcfg/internal/service"
	internaltui "mcfg/internal/tui"
)

func newTUICommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the Bubble Tea interface",
		Long:  "Launch the interactive Bubble Tea interface. This is the same entrypoint used when running mcfg without subcommands.",
		Example: "  mcfg\n" +
			"  mcfg tui",
		RunE: withLock(app, nil, func(cmd *cobra.Command, _ []string) error {
			return runTUI(cmd, app)
		}),
	}
}

func runTUI(cmd *cobra.Command, app *App) error {
	controller := &tuiController{app: app, cmd: cmd}
	snapshot, err := controller.Refresh()
	if err != nil {
		return fmt.Errorf("load config before starting TUI: %w", err)
	}

	model := internaltui.New(snapshot, controller)
	program := tea.NewProgram(model, tea.WithInput(cmd.InOrStdin()), tea.WithOutput(cmd.OutOrStdout()))
	_, err = program.Run()
	return err
}

type tuiController struct {
	app *App
	cmd *cobra.Command
}

func (c *tuiController) Refresh() (internaltui.Snapshot, error) {
	cfg, err := c.app.Store.Load(c.cmd.Context())
	if err != nil {
		return internaltui.Snapshot{}, err
	}

	currentModelName := ""
	for _, item := range cfg.Models {
		if item.ID == cfg.ClaudeBinding.CurrentModelID {
			currentModelName = item.Name
			break
		}
	}
	return internaltui.Snapshot{
		CurrentModelID:   cfg.ClaudeBinding.CurrentModelID,
		CurrentModelName: currentModelName,
		EnabledMCPIDs:    append([]string(nil), cfg.ClaudeBinding.EnabledMCPIDs...),
		EnabledMCPCount:  len(cfg.ClaudeBinding.EnabledMCPIDs),
		LastSyncResult:   cfg.ClaudeBinding.LastSyncResult,
		LockStatus:       "exclusive",
		TargetStatus:     targetStatus(cfg.ClaudeBinding.LastSyncResult),
		Models:           cfg.Models,
		MCPServers:       cfg.MCPServers,
	}, nil
}

func (c *tuiController) UseModel(id string) (internaltui.Snapshot, error) {
	if _, err := c.app.ModelService.Use(c.cmd.Context(), id); err != nil {
		return internaltui.Snapshot{}, err
	}
	return c.Refresh()
}

func (c *tuiController) ToggleMCP(id string) (internaltui.Snapshot, error) {
	snapshot, err := c.Refresh()
	if err != nil {
		return internaltui.Snapshot{}, err
	}
	if slices.Contains(snapshot.EnabledMCPIDs, id) {
		if _, _, err := c.app.MCPService.Disable(c.cmd.Context(), id); err != nil {
			return internaltui.Snapshot{}, err
		}
	} else {
		if _, _, err := c.app.MCPService.Enable(c.cmd.Context(), id); err != nil {
			return internaltui.Snapshot{}, err
		}
	}
	return c.Refresh()
}

func (c *tuiController) SyncPreview() ([]string, error) {
	result, err := c.app.SyncService.Sync(c.cmd.Context(), service.SyncOptions{DryRun: true})
	if err != nil {
		return nil, err
	}
	return result.ChangedPaths, nil
}

func (c *tuiController) ListBackups() ([]internaltui.BackupItem, error) {
	records, err := c.app.BackupService.List(c.cmd.Context())
	if err != nil {
		return nil, err
	}
	items := make([]internaltui.BackupItem, 0, len(records))
	for _, record := range records {
		items = append(items, internaltui.BackupItem{
			ID:        record.Meta.ID,
			CreatedAt: record.Meta.CreatedAt,
			Reason:    record.Meta.Reason,
			Corrupted: record.Corrupted,
		})
	}
	return items, nil
}

func (c *tuiController) SyncApply() (internaltui.Snapshot, []string, error) {
	result, err := c.app.SyncService.Sync(c.cmd.Context(), service.SyncOptions{})
	if err != nil {
		return internaltui.Snapshot{}, nil, err
	}
	snapshot, err := c.Refresh()
	if err != nil {
		return internaltui.Snapshot{}, nil, err
	}
	return snapshot, result.ChangedPaths, nil
}

func (c *tuiController) RestoreBackup(id string) (internaltui.Snapshot, []internaltui.BackupItem, error) {
	if _, err := c.app.BackupService.Restore(c.cmd.Context(), id); err != nil {
		return internaltui.Snapshot{}, nil, err
	}
	snapshot, err := c.Refresh()
	if err != nil {
		return internaltui.Snapshot{}, nil, err
	}
	backups, err := c.ListBackups()
	if err != nil {
		return internaltui.Snapshot{}, nil, err
	}
	return snapshot, backups, nil
}

func (c *tuiController) AddModel(input internaltui.ModelFormInput) (internaltui.Snapshot, error) {
	if _, err := c.app.ModelService.Add(c.cmd.Context(), service.ModelAddInput{
		Name:        input.Name,
		AuthToken:   input.AuthToken,
		BaseURL:     input.BaseURL,
		Model:       input.Model,
		Description: input.Description,
		Env:         map[string]string{},
	}); err != nil {
		return internaltui.Snapshot{}, err
	}
	return c.Refresh()
}

func (c *tuiController) EditModel(id string, input internaltui.ModelFormInput) (internaltui.Snapshot, error) {
	if _, err := c.app.ModelService.Edit(c.cmd.Context(), id, service.ModelEditInput{
		Name:        stringPtr(input.Name),
		AuthToken:   stringPtr(input.AuthToken),
		BaseURL:     stringPtr(input.BaseURL),
		Model:       stringPtr(input.Model),
		Description: stringPtr(input.Description),
	}); err != nil {
		return internaltui.Snapshot{}, err
	}
	return c.Refresh()
}

func (c *tuiController) RemoveModel(id string) (internaltui.Snapshot, error) {
	if err := c.app.ModelService.Remove(c.cmd.Context(), id, true); err != nil {
		return internaltui.Snapshot{}, err
	}
	return c.Refresh()
}

func (c *tuiController) AddMCP(input internaltui.MCPFormInput) (internaltui.Snapshot, error) {
	if _, err := c.app.MCPService.Add(c.cmd.Context(), service.MCPAddInput{
		Name:        input.Name,
		Command:     input.Command,
		Args:        input.Args,
		Env:         input.Env,
		Description: input.Description,
	}); err != nil {
		return internaltui.Snapshot{}, err
	}
	return c.Refresh()
}

func (c *tuiController) EditMCP(id string, input internaltui.MCPFormInput) (internaltui.Snapshot, error) {
	if _, err := c.app.MCPService.Edit(c.cmd.Context(), id, service.MCPEditInput{
		Name:        stringPtr(input.Name),
		Command:     stringPtr(input.Command),
		Args:        input.Args,
		ReplaceArgs: true,
		Env:         input.Env,
		ReplaceEnv:  true,
		Description: stringPtr(input.Description),
	}); err != nil {
		return internaltui.Snapshot{}, err
	}
	return c.Refresh()
}

func (c *tuiController) RemoveMCP(id string) (internaltui.Snapshot, error) {
	if err := c.app.MCPService.Remove(c.cmd.Context(), id, true); err != nil {
		return internaltui.Snapshot{}, err
	}
	return c.Refresh()
}

func targetStatus(lastSyncResult string) string {
	if lastSyncResult == "" {
		return "unknown"
	}
	return lastSyncResult
}

func stringPtr(value string) *string {
	return &value
}
