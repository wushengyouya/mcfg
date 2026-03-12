package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newImportCommand(app *App) *cobra.Command {
	var jsonOutput bool

	command := &cobra.Command{
		Use:   "import",
		Short: "Import Claude Code user configuration",
		Long:  "Scan Claude Code user-level files and import discovered models and MCP servers into the local config center.",
		Example: "  mcfg import\n" +
			"  mcfg import --json",
		RunE: withLock(app, nil, func(cmd *cobra.Command, _ []string) error {
			summary, err := app.ImportService.Import(cmd.Context())
			if err != nil {
				return err
			}
			if jsonOutput {
				data, err := json.MarshalIndent(summary, "", "  ")
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "imported models=%d mcps=%d skipped=%d\n", summary.ImportedModels, summary.ImportedMCPs, summary.Skipped)
			return err
		}),
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "Output JSON")
	return command
}
