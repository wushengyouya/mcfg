package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newInitCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize the local config center",
		Long:  "Create ~/.mcfg, initialize the local store, and import existing Claude Code user configuration on first run.",
		Example: "  mcfg init\n" +
			"  mcfg init",
		RunE: withLock(app, nil, func(cmd *cobra.Command, _ []string) error {
			created, err := app.Store.Init(cmd.Context())
			if err != nil {
				return err
			}
			if !created {
				if _, loadErr := app.Store.Load(cmd.Context()); loadErr != nil {
					return loadErr
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "already initialized")
				return err
			}
			summary, err := app.ImportService.Import(cmd.Context())
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "initialized ~/.mcfg (imported models=%d mcps=%d skipped=%d)\n", summary.ImportedModels, summary.ImportedMCPs, summary.Skipped)
			return err
		}),
	}
}
