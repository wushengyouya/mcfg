package cmd

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

func newStatusCommand(app *App) *cobra.Command {
	var jsonOutput bool

	command := &cobra.Command{
		Use:   "status",
		Short: "Show the current binding status",
		Long:  "Show the active model binding, enabled MCP servers, and the latest known sync summary.",
		Example: "  mcfg status\n" +
			"  mcfg status --json",
		RunE: withLock(app, sharedLockMode, func(cmd *cobra.Command, _ []string) error {
			cfg, err := app.Store.Load(cmd.Context())
			if err != nil {
				return err
			}

			currentModelName := ""
			for _, item := range cfg.Models {
				if item.ID == cfg.ClaudeBinding.CurrentModelID {
					currentModelName = item.Name
					break
				}
			}

			enabled := make([]string, 0, len(cfg.ClaudeBinding.EnabledMCPIDs))
			for _, server := range cfg.MCPServers {
				if slices.Contains(cfg.ClaudeBinding.EnabledMCPIDs, server.ID) {
					enabled = append(enabled, server.Name)
				}
			}

			payload := map[string]any{
				"current_model_id":   cfg.ClaudeBinding.CurrentModelID,
				"current_model_name": currentModelName,
				"enabled_mcp_ids":    cfg.ClaudeBinding.EnabledMCPIDs,
				"enabled_mcp_names":  enabled,
				"sync_status":        emptyAs(cfg.ClaudeBinding.LastSyncResult, "never"),
				"last_sync_at":       cfg.ClaudeBinding.LastSyncAt,
				"last_sync_result":   cfg.ClaudeBinding.LastSyncResult,
			}
			if jsonOutput {
				data, err := json.MarshalIndent(payload, "", "  ")
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return err
			}

			if currentModelName == "" {
				currentModelName = "(none)"
			}
			enabledSummary := fmt.Sprintf("%d", len(enabled))
			if len(enabled) > 0 {
				enabledSummary = fmt.Sprintf("%d (%s)", len(enabled), strings.Join(enabled, ", "))
			}
			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"Current model: %s\nEnabled MCPs: %s\nSync status: %s\nLast sync at: %s\n",
				currentModelName,
				enabledSummary,
				emptyAs(cfg.ClaudeBinding.LastSyncResult, "never"),
				emptyAs(cfg.ClaudeBinding.LastSyncAt, "never"),
			)
			return err
		}),
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "Output JSON")
	return command
}

func emptyAs(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
