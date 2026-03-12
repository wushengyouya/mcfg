package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"mcfg/internal/lock"
)

type lockModeFunc func(cmd *cobra.Command, args []string) lock.Mode

func withLock(app *App, modeFn lockModeFunc, run func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		mode := lock.Exclusive
		if modeFn != nil {
			mode = modeFn(cmd, args)
		}
		manager := lock.New(app.LockPath, time.Now)
		handle, err := manager.Acquire(mode, commandSummary(cmd, args))
		if err != nil {
			return err
		}
		defer func() { _ = handle.Release() }()
		return run(cmd, args)
	}
}

func sharedLockMode(_ *cobra.Command, _ []string) lock.Mode {
	return lock.Shared
}

func syncLockMode(cmd *cobra.Command, _ []string) lock.Mode {
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err == nil && dryRun {
		return lock.Shared
	}
	return lock.Exclusive
}

func commandSummary(cmd *cobra.Command, args []string) string {
	parts := append([]string{cmd.CommandPath()}, args...)
	return strings.TrimSpace(strings.Join(parts, " "))
}

func lockConflictMessage(err error) string {
	return fmt.Sprintf("%v", err)
}
