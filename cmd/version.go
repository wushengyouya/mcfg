package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"mcfg/internal/buildinfo"
)

func newVersionCommand() *cobra.Command {
	var jsonOutput bool

	command := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  "Show application version, build metadata, Go runtime version, and target platform.",
		Example: "  mcfg version\n" +
			"  mcfg version --json\n" +
			"  mcfg --version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := buildinfo.Current()
			if jsonOutput {
				data, err := json.MarshalIndent(info, "", "  ")
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return err
			}

			_, err := fmt.Fprintf(
				cmd.OutOrStdout(),
				"Version: %s\nCommit: %s\nBuild Date: %s\nGo: %s\nPlatform: %s\n",
				info.Version,
				info.Commit,
				info.BuildDate,
				info.GoVersion,
				info.Platform,
			)
			return err
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "Output JSON")
	return command
}
