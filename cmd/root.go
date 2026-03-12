package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"mcfg/internal/buildinfo"
	"mcfg/internal/exitcode"
	"mcfg/internal/id"
	"mcfg/internal/scanner"
	"mcfg/internal/service"
	"mcfg/internal/store"
)

type Options struct {
	HomeDir string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

type App struct {
	HomeDir         string
	LockPath        string
	Store           *store.Store
	ModelService    *service.ModelService
	MCPService      *service.MCPService
	ImportService   *service.ImportService
	BackupService   *service.BackupService
	SyncService     *service.SyncService
	ValidateService *service.ValidateService
}

func Execute(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	homeDir, _ := os.UserHomeDir()
	cmd := NewRootCommand(Options{
		HomeDir: homeDir,
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
	})
	cmd.SetArgs(args)
	return exitcode.FromError(cmd.Execute())
}

func NewRootCommand(opts Options) *cobra.Command {
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.HomeDir == "" {
		opts.HomeDir, _ = os.UserHomeDir()
	}

	appStore := store.New(opts.HomeDir)
	app := &App{
		HomeDir:         opts.HomeDir,
		LockPath:        filepath.Join(appStore.ConfigDir(), "run.lock"),
		Store:           appStore,
		ModelService:    service.NewModelService(appStore, nil, id.ULIDGenerator{}),
		MCPService:      service.NewMCPService(appStore, nil, id.ULIDGenerator{}),
		ImportService:   service.NewImportService(appStore, scanner.New(opts.HomeDir, func() string { return time.Now().UTC().Format(time.RFC3339) }, id.ULIDGenerator{})),
		BackupService:   service.NewBackupService(appStore, opts.HomeDir, appStore.BackupsDir(), nil, id.ULIDGenerator{}),
		SyncService:     service.NewSyncService(appStore, opts.HomeDir, appStore.BackupsDir(), nil, id.ULIDGenerator{}),
		ValidateService: service.NewValidateService(appStore, opts.HomeDir),
	}

	root := &cobra.Command{
		Use:           "mcfg",
		Short:         "Claude Code configuration manager",
		Long:          "Claude Code configuration manager.\n\nmcfg manages Claude Code model bindings, MCP servers, sync, validation, backups, and the interactive TUI from one local config center.",
		Example:       "  mcfg\n  mcfg init\n  mcfg status --json\n  mcfg tui",
		Version:       buildinfo.Current().Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          withLock(app, nil, func(cmd *cobra.Command, _ []string) error { return runTUI(cmd, app) }),
	}
	root.SetIn(opts.Stdin)
	root.SetOut(opts.Stdout)
	root.SetErr(opts.Stderr)
	root.SetContext(context.Background())

	root.AddCommand(
		newInitCommand(app),
		newImportCommand(app),
		newStatusCommand(app),
		newValidateCommand(app),
		newSyncCommand(app),
		newBackupCommand(app),
		newTUICommand(app),
		newVersionCommand(),
		newModelCommand(app),
		newMCPCommand(app),
	)
	return root
}
