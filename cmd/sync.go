package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"mcfg/internal/service"
)

func newSyncCommand(app *App) *cobra.Command {
	var dryRun bool
	var initTarget bool
	var jsonOutput bool

	command := &cobra.Command{
		Use:   "sync",
		Short: "Sync local configuration to Claude Code",
		Long:  "Render managed Claude Code fields from the local config center and write them atomically to the target files.",
		Example: "  mcfg sync\n" +
			"  mcfg sync --dry-run\n" +
			"  mcfg sync --init-target\n" +
			"  mcfg sync --dry-run --json",
		RunE: withLock(app, syncLockMode, func(cmd *cobra.Command, _ []string) error {
			result, err := app.SyncService.Sync(cmd.Context(), service.SyncOptions{
				DryRun:     dryRun,
				InitTarget: initTarget,
			})
			if err != nil {
				return err
			}
			if jsonOutput {
				data, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return err
			}
			if dryRun {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "dry-run changes: %v\n", result.ChangedPaths)
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "synced changes: %v\n", result.ChangedPaths)
			return err
		}),
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without writing")
	command.Flags().BoolVar(&initTarget, "init-target", false, "Create target skeleton files when missing")
	command.Flags().BoolVar(&jsonOutput, "json", false, "Output JSON")
	return command
}
