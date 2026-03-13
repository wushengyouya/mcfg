package cmd

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"mcfg/internal/id"
	"mcfg/internal/model"
	"mcfg/internal/service"
)

func newMCPCommand(app *App) *cobra.Command {
	command := &cobra.Command{
		Use:   "mcp",
		Short: "Manage MCP servers",
		Long:  "Manage local MCP server definitions and their enabled state for Claude Code.",
	}
	command.AddCommand(
		newMCPListCommand(app),
		newMCPAddCommand(app),
		newMCPEditCommand(app),
		newMCPRemoveCommand(app),
		newMCPEnableCommand(app),
		newMCPDisableCommand(app),
	)
	return command
}

func newMCPListCommand(app *App) *cobra.Command {
	var jsonOutput bool

	command := &cobra.Command{
		Use:   "list",
		Short: "List MCP servers",
		Example: "  mcfg mcp list\n" +
			"  mcfg mcp list --json",
		RunE: withLock(app, sharedLockMode, func(cmd *cobra.Command, _ []string) error {
			items, err := app.MCPService.List(cmd.Context())
			if err != nil {
				return err
			}
			if jsonOutput {
				data, err := json.MarshalIndent(items, "", "  ")
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return err
			}
			if len(items) == 0 {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "No MCP servers configured.")
				return err
			}
			for _, item := range items {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", item.ID, item.Name, item.Command); err != nil {
					return err
				}
			}
			return nil
		}),
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "Output JSON")
	return command
}

func newMCPAddCommand(app *App) *cobra.Command {
	var name string
	var commandValue string
	var args []string
	var envItems []string
	var description string

	command := &cobra.Command{
		Use:   "add",
		Short: "Add an MCP server",
		Example: "  mcfg mcp add --name filesystem --command npx --args -y --args @modelcontextprotocol/server-filesystem\n" +
			"  mcfg mcp add --name postgres --command uvx --env DATABASE_URL=postgres://localhost/app",
		RunE: withLock(app, nil, func(cmd *cobra.Command, _ []string) error {
			env, err := parseEnvItems(envItems)
			if err != nil {
				return err
			}
			server, err := app.MCPService.Add(cmd.Context(), service.MCPAddInput{
				Name:        name,
				Command:     commandValue,
				Args:        args,
				Env:         env,
				Description: description,
			})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "added mcp %s (%s)\n", server.Name, server.ID)
			return err
		}),
	}
	command.Flags().StringVar(&name, "name", "", "Unique MCP name")
	command.Flags().StringVar(&commandValue, "command", "", "Command")
	command.Flags().StringArrayVar(&args, "args", nil, "Command arguments")
	command.Flags().StringArrayVar(&envItems, "env", nil, "Environment variables")
	command.Flags().StringVar(&description, "desc", "", "Description")
	_ = command.MarkFlagRequired("name")
	_ = command.MarkFlagRequired("command")
	return command
}

func newMCPEditCommand(app *App) *cobra.Command {
	var name string
	var commandValue string
	var args []string
	var envItems []string
	var clearArgs bool
	var clearEnv bool
	var description string

	command := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit an MCP server",
		Args:  cobra.ExactArgs(1),
		Example: "  mcfg mcp edit 01ARZ3NDEKTSV4RRFFQ69G5FAV --command uvx\n" +
			"  mcfg mcp edit 01ARZ3ND --args -y --args @modelcontextprotocol/server-git\n" +
			"  mcfg mcp edit 01ARZ3ND --clear-env",
		RunE: withLock(app, nil, func(cmd *cobra.Command, values []string) error {
			env, err := parseEnvItems(envItems)
			if err != nil {
				return err
			}
			input := service.MCPEditInput{
				ClearArgs: clearArgs,
				ClearEnv:  clearEnv,
			}
			if cmd.Flags().Changed("name") {
				input.Name = &name
			}
			if cmd.Flags().Changed("command") {
				input.Command = &commandValue
			}
			if cmd.Flags().Changed("args") {
				input.Args = args
				input.ReplaceArgs = true
			}
			if cmd.Flags().Changed("env") {
				input.Env = env
				input.ReplaceEnv = true
			}
			if cmd.Flags().Changed("desc") {
				input.Description = &description
			}
			server, err := app.MCPService.Edit(cmd.Context(), values[0], input)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "updated mcp %s (%s)\n", server.Name, server.ID)
			return err
		}),
	}
	command.Flags().StringVar(&name, "name", "", "Unique MCP name")
	command.Flags().StringVar(&commandValue, "command", "", "Command")
	command.Flags().StringArrayVar(&args, "args", nil, "Command arguments")
	command.Flags().StringArrayVar(&envItems, "env", nil, "Environment variables")
	command.Flags().BoolVar(&clearArgs, "clear-args", false, "Clear args")
	command.Flags().BoolVar(&clearEnv, "clear-env", false, "Clear env")
	command.Flags().StringVar(&description, "desc", "", "Description")
	return command
}

func newMCPRemoveCommand(app *App) *cobra.Command {
	var force bool

	command := &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove an MCP server",
		Args:  cobra.ExactArgs(1),
		Example: "  mcfg mcp remove 01ARZ3NDEKTSV4RRFFQ69G5FAV\n" +
			"  mcfg mcp remove 01ARZ3ND --force",
		RunE: withLock(app, nil, func(cmd *cobra.Command, args []string) error {
			cfg, err := app.Store.Load(cmd.Context())
			if err != nil {
				return err
			}
			targetID, err := matchMCPID(args[0], cfg.MCPServers)
			if err != nil {
				return err
			}
			wasEnabled := slices.Contains(cfg.ClaudeBinding.EnabledMCPIDs, targetID)
			if err := app.MCPService.Remove(cmd.Context(), args[0], force); err != nil {
				return err
			}
			if force && wasEnabled {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "removed mcp %s (disabled before removal)\n", args[0])
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "removed mcp %s\n", args[0])
			return err
		}),
	}
	command.Flags().BoolVar(&force, "force", false, "Force remove even if enabled")
	return command
}

func newMCPEnableCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <id>",
		Short: "Enable an MCP server",
		Args:  cobra.ExactArgs(1),
		Example: "  mcfg mcp enable 01ARZ3NDEKTSV4RRFFQ69G5FAV\n" +
			"  mcfg mcp enable 01ARZ3ND",
		RunE: withLock(app, nil, func(cmd *cobra.Command, args []string) error {
			alreadyEnabled, server, err := app.MCPService.Enable(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if alreadyEnabled {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "already enabled: %s\n", server.Name)
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "enabled mcp %s\n", server.Name)
			return err
		}),
	}
}

func newMCPDisableCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <id>",
		Short: "Disable an MCP server",
		Args:  cobra.ExactArgs(1),
		Example: "  mcfg mcp disable 01ARZ3NDEKTSV4RRFFQ69G5FAV\n" +
			"  mcfg mcp disable 01ARZ3ND",
		RunE: withLock(app, nil, func(cmd *cobra.Command, args []string) error {
			alreadyDisabled, server, err := app.MCPService.Disable(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if alreadyDisabled {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "already disabled: %s\n", server.Name)
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "disabled mcp %s\n", server.Name)
			return err
		}),
	}
}

func matchMCPID(prefix string, items []model.MCPServer) (string, error) {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return id.MatchByPrefix(prefix, ids)
}
