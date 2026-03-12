package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"mcfg/internal/service"
)

func newBackupCommand(app *App) *cobra.Command {
	command := &cobra.Command{
		Use:   "backup",
		Short: "Manage Claude Code backups",
		Long:  "Create, inspect, restore, and prune Claude Code target file backups managed by mcfg.",
	}
	command.AddCommand(
		newBackupCreateCommand(app),
		newBackupListCommand(app),
		newBackupRestoreCommand(app),
		newBackupPruneCommand(app),
	)
	return command
}

func newBackupCreateCommand(app *App) *cobra.Command {
	var jsonOutput bool

	command := &cobra.Command{
		Use:   "create",
		Short: "Create a backup",
		Example: "  mcfg backup create\n" +
			"  mcfg backup create --json",
		RunE: withLock(app, nil, func(cmd *cobra.Command, _ []string) error {
			meta, err := app.BackupService.Create(cmd.Context(), "manual")
			if err != nil {
				return err
			}
			if jsonOutput {
				data, err := json.MarshalIndent(meta, "", "  ")
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "created backup %s\n", meta.ID)
			return err
		}),
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "Output JSON")
	return command
}

func newBackupListCommand(app *App) *cobra.Command {
	var jsonOutput bool

	command := &cobra.Command{
		Use:   "list",
		Short: "List backups",
		Example: "  mcfg backup list\n" +
			"  mcfg backup list --json",
		RunE: withLock(app, sharedLockMode, func(cmd *cobra.Command, _ []string) error {
			records, err := app.BackupService.List(cmd.Context())
			if err != nil {
				return err
			}
			if jsonOutput {
				data, err := json.MarshalIndent(records, "", "  ")
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return err
			}
			if len(records) == 0 {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "No backups found.")
				return err
			}
			for _, record := range records {
				line := fmt.Sprintf("%s\t%s\t%s", record.Meta.ID, record.Meta.CreatedAt, record.Meta.Reason)
				if record.Corrupted {
					line += "\tcorrupted"
				}
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
					return err
				}
			}
			return nil
		}),
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "Output JSON")
	return command
}

func newBackupRestoreCommand(app *App) *cobra.Command {
	var jsonOutput bool

	command := &cobra.Command{
		Use:   "restore <backup-id>",
		Short: "Restore a backup",
		Args:  cobra.ExactArgs(1),
		Example: "  mcfg backup restore <backup-id>\n" +
			"  mcfg backup restore <backup-id> --json",
		RunE: withLock(app, nil, func(cmd *cobra.Command, args []string) error {
			meta, err := app.BackupService.Restore(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				data, err := json.MarshalIndent(meta, "", "  ")
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "restored backup %s\n", meta.ID)
			return err
		}),
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "Output JSON")
	return command
}

func newBackupPruneCommand(app *App) *cobra.Command {
	var keep int
	var jsonOutput bool

	command := &cobra.Command{
		Use:   "prune",
		Short: "Prune old backups",
		Example: "  mcfg backup prune --keep 3\n" +
			"  mcfg backup prune --keep 3 --json",
		RunE: withLock(app, nil, func(cmd *cobra.Command, _ []string) error {
			removed, err := app.BackupService.Prune(cmd.Context(), keep)
			if err != nil {
				return err
			}
			if jsonOutput {
				data, err := json.MarshalIndent(map[string]int{"removed": removed, "keep": keep}, "", "  ")
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "pruned backups: %d\n", removed)
			return err
		}),
	}
	command.Flags().IntVar(&keep, "keep", service.DefaultBackupKeep, "How many backups to keep")
	command.Flags().BoolVar(&jsonOutput, "json", false, "Output JSON")
	return command
}
