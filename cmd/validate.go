package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"mcfg/internal/exitcode"
)

func newValidateCommand(app *App) *cobra.Command {
	var jsonOutput bool

	command := &cobra.Command{
		Use:   "validate",
		Short: "Validate local configuration",
		Long:  "Validate local config consistency, field-level correctness, and managed target drift without writing any files.",
		Example: "  mcfg validate\n" +
			"  mcfg validate --json",
		RunE: withLock(app, sharedLockMode, func(cmd *cobra.Command, _ []string) error {
			report, err := app.ValidateService.Validate(cmd.Context())
			if err != nil {
				return err
			}
			if jsonOutput {
				data, marshalErr := json.MarshalIndent(report, "", "  ")
				if marshalErr != nil {
					return marshalErr
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				if err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Summary\nvalid: %t\nerrors: %d\nwarnings: %d\nsync_status: %s\n\n", report.Valid, len(report.Errors), len(report.Warnings), report.SyncStatus); err != nil {
					return err
				}
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Errors\n"); err != nil {
					return err
				}
				if len(report.Errors) == 0 {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), "(none)"); err != nil {
						return err
					}
				}
				for _, issue := range report.Errors {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "- [%s] %s: %s\n", issue.Code, issue.Path, issue.Message); err != nil {
						return err
					}
				}
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "\nWarnings\n"); err != nil {
					return err
				}
				if len(report.Warnings) == 0 {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), "(none)"); err != nil {
						return err
					}
				} else {
					for _, issue := range report.Warnings {
						if _, err := fmt.Fprintf(cmd.OutOrStdout(), "- [%s] %s: %s\n", issue.Code, issue.Path, issue.Message); err != nil {
							return err
						}
					}
				}
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "\nTarget Drift\n"); err != nil {
					return err
				}
				if len(report.Drift.ManagedPathsChanged) == 0 {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), "(none)"); err != nil {
						return err
					}
				} else {
					for _, path := range report.Drift.ManagedPathsChanged {
						if _, err := fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", path); err != nil {
							return err
						}
					}
				}
			}

			if !report.Valid {
				return fmt.Errorf("%w: validation failed", exitcode.ErrBusiness)
			}
			return nil
		}),
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "Output JSON")
	return command
}

func ternary[T any](condition bool, left, right T) T {
	if condition {
		return left
	}
	return right
}
