package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"mcfg/internal/exitcode"
	"mcfg/internal/id"
	"mcfg/internal/model"
	"mcfg/internal/service"
)

func newModelCommand(app *App) *cobra.Command {
	command := &cobra.Command{
		Use:   "model",
		Short: "Manage model profiles",
		Long:  "Manage local model profiles and switch the active Claude Code model binding.",
	}
	command.AddCommand(
		newModelListCommand(app),
		newModelAddCommand(app),
		newModelEditCommand(app),
		newModelRemoveCommand(app),
		newModelUseCommand(app),
	)
	return command
}

func newModelListCommand(app *App) *cobra.Command {
	var jsonOutput bool

	command := &cobra.Command{
		Use:     "list",
		Short:   "List models",
		Example: "  mcfg model list\n  mcfg model list --json",
		RunE: withLock(app, sharedLockMode, func(cmd *cobra.Command, _ []string) error {
			items, err := app.ModelService.List(cmd.Context())
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
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "No models configured.")
				return err
			}
			for _, item := range items {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", item.ID, item.Name, item.Env["ANTHROPIC_MODEL"]); err != nil {
					return err
				}
			}
			return nil
		}),
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "Output JSON")
	return command
}

func newModelAddCommand(app *App) *cobra.Command {
	var name string
	var baseURL string
	var modelName string
	var authToken string
	var authTokenStdin bool
	var authTokenFile string
	var envItems []string
	var description string

	command := &cobra.Command{
		Use:   "add",
		Short: "Add a model profile",
		Example: "  mcfg model add --name \"Claude Sonnet\" --base-url https://example.com --model claude-sonnet-4-0 --auth-token secret\n" +
			"  printf 'secret\\n' | mcfg model add --auth-token-stdin --name \"Claude Sonnet\" --base-url https://example.com --model claude-sonnet-4-0\n" +
			"  mcfg model add --name \"Claude Sonnet\" --base-url https://example.com --model claude-sonnet-4-0 --auth-token-file ~/.config/anthropic.token",
		RunE: withLock(app, nil, func(cmd *cobra.Command, _ []string) error {
			token, err := resolveToken(cmd.InOrStdin(), authToken, authTokenStdin, authTokenFile)
			if err != nil {
				return err
			}
			env, err := parseEnvItems(envItems)
			if err != nil {
				return err
			}
			profile, err := app.ModelService.Add(cmd.Context(), service.ModelAddInput{
				Name:        name,
				AuthToken:   token,
				BaseURL:     baseURL,
				Model:       modelName,
				Env:         env,
				Description: description,
			})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "added model %s (%s)\n", profile.Name, profile.ID)
			return err
		}),
	}
	command.Flags().StringVar(&name, "name", "", "Unique model name")
	command.Flags().StringVar(&baseURL, "base-url", "", "Base URL")
	command.Flags().StringVar(&modelName, "model", "", "Model name")
	command.Flags().StringVar(&authToken, "auth-token", "", "Auth token")
	command.Flags().BoolVar(&authTokenStdin, "auth-token-stdin", false, "Read auth token from stdin")
	command.Flags().StringVar(&authTokenFile, "auth-token-file", "", "Read auth token from file")
	command.Flags().StringArrayVar(&envItems, "env", nil, "Extra env vars")
	command.Flags().StringVar(&description, "desc", "", "Description")
	_ = command.MarkFlagRequired("name")
	_ = command.MarkFlagRequired("base-url")
	_ = command.MarkFlagRequired("model")
	return command
}

func newModelEditCommand(app *App) *cobra.Command {
	var name string
	var baseURL string
	var modelName string
	var authToken string
	var authTokenStdin bool
	var authTokenFile string
	var envItems []string
	var replaceEnv bool
	var clearEnv bool
	var description string

	command := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a model profile",
		Args:  cobra.ExactArgs(1),
		Example: "  mcfg model edit 01ARZ3NDEKTSV4RRFFQ69G5FAV --name \"Claude Opus\"\n" +
			"  mcfg model edit 01ARZ3ND --model claude-opus-4-1 --base-url https://example.com/v2\n" +
			"  mcfg model edit 01ARZ3ND --clear-env",
		RunE: withLock(app, nil, func(cmd *cobra.Command, args []string) error {
			token, err := resolveOptionalToken(cmd.InOrStdin(), authToken, authTokenStdin, authTokenFile)
			if err != nil {
				return err
			}
			env, err := parseEnvItems(envItems)
			if err != nil {
				return err
			}
			input := service.ModelEditInput{
				ReplaceEnv: replaceEnv,
				ClearEnv:   clearEnv,
			}
			if cmd.Flags().Changed("name") {
				input.Name = &name
			}
			if token != nil {
				input.AuthToken = token
			}
			if cmd.Flags().Changed("base-url") {
				input.BaseURL = &baseURL
			}
			if cmd.Flags().Changed("model") {
				input.Model = &modelName
			}
			if cmd.Flags().Changed("desc") {
				input.Description = &description
			}
			if cmd.Flags().Changed("env") {
				input.Env = env
				input.ReplaceEnv = true
			}
			updated, err := app.ModelService.Edit(cmd.Context(), args[0], input)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "updated model %s (%s)\n", updated.Name, updated.ID)
			return err
		}),
	}
	command.Flags().StringVar(&name, "name", "", "Unique model name")
	command.Flags().StringVar(&baseURL, "base-url", "", "Base URL")
	command.Flags().StringVar(&modelName, "model", "", "Model name")
	command.Flags().StringVar(&authToken, "auth-token", "", "Auth token")
	command.Flags().BoolVar(&authTokenStdin, "auth-token-stdin", false, "Read auth token from stdin")
	command.Flags().StringVar(&authTokenFile, "auth-token-file", "", "Read auth token from file")
	command.Flags().StringArrayVar(&envItems, "env", nil, "Extra env vars")
	command.Flags().BoolVar(&replaceEnv, "replace-env", false, "Replace custom env")
	command.Flags().BoolVar(&clearEnv, "clear-env", false, "Clear custom env")
	command.Flags().StringVar(&description, "desc", "", "Description")
	return command
}

func newModelRemoveCommand(app *App) *cobra.Command {
	var force bool

	command := &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a model profile",
		Args:  cobra.ExactArgs(1),
		Example: "  mcfg model remove 01ARZ3NDEKTSV4RRFFQ69G5FAV\n" +
			"  mcfg model remove 01ARZ3ND --force",
		RunE: withLock(app, nil, func(cmd *cobra.Command, args []string) error {
			cfg, err := app.Store.Load(cmd.Context())
			if err != nil {
				return err
			}
			targetID, err := matchModelID(args[0], cfg.Models)
			if err != nil {
				return err
			}
			wasBound := cfg.ClaudeBinding.CurrentModelID == targetID
			if err := app.ModelService.Remove(cmd.Context(), args[0], force); err != nil {
				return err
			}
			if force && wasBound {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "removed model %s (cleared current model binding)\n", args[0])
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "removed model %s\n", args[0])
			return err
		}),
	}
	command.Flags().BoolVar(&force, "force", false, "Force remove even if bound")
	return command
}

func newModelUseCommand(app *App) *cobra.Command {
	var syncNow bool

	command := &cobra.Command{
		Use:   "use <id>",
		Short: "Bind the current model",
		Args:  cobra.ExactArgs(1),
		Example: "  mcfg model use 01ARZ3NDEKTSV4RRFFQ69G5FAV\n" +
			"  mcfg model use 01ARZ3ND --sync",
		RunE: withLock(app, nil, func(cmd *cobra.Command, args []string) error {
			cfg, err := app.Store.Load(cmd.Context())
			if err != nil {
				return err
			}
			previousModelID := cfg.ClaudeBinding.CurrentModelID

			used, err := app.ModelService.Use(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if syncNow {
				result, syncErr := app.SyncService.Sync(cmd.Context(), service.SyncOptions{})
				if syncErr != nil {
					cfg.ClaudeBinding.CurrentModelID = previousModelID
					if saveErr := app.Store.Save(cmd.Context(), cfg); saveErr != nil {
						return fmt.Errorf("sync failed: %w; rollback failed: %v", syncErr, saveErr)
					}
					return syncErr
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "current model: %s (synced %v)\n", used.Name, result.ChangedPaths)
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "current model: %s\n", used.Name)
			return err
		}),
	}
	command.Flags().BoolVar(&syncNow, "sync", false, "Sync after switching model")
	return command
}

func parseEnvItems(items []string) (map[string]string, error) {
	env := map[string]string{}
	for _, item := range items {
		key, value, found := strings.Cut(item, "=")
		if !found || key == "" {
			return nil, fmt.Errorf("%w: invalid env item %q", exitcode.ErrParam, item)
		}
		env[key] = value
	}
	return env, nil
}

func resolveToken(reader io.Reader, literal string, fromStdin bool, filePath string) (string, error) {
	token, err := resolveOptionalToken(reader, literal, fromStdin, filePath)
	if err != nil {
		return "", err
	}
	if token == nil || *token == "" {
		return "", fmt.Errorf("%w: exactly one auth token source is required", exitcode.ErrParam)
	}
	return *token, nil
}

func resolveOptionalToken(reader io.Reader, literal string, fromStdin bool, filePath string) (*string, error) {
	count := 0
	if literal != "" {
		count++
	}
	if fromStdin {
		count++
	}
	if filePath != "" {
		count++
	}
	if count > 1 {
		return nil, fmt.Errorf("%w: auth token flags are mutually exclusive", exitcode.ErrParam)
	}
	switch {
	case literal != "":
		return &literal, nil
	case fromStdin:
		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("%w: read auth token from stdin: %v", exitcode.ErrIO, err)
		}
		value := strings.TrimSpace(string(data))
		return &value, nil
	case filePath != "":
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("%w: read auth token file: %v", exitcode.ErrIO, err)
		}
		value := strings.TrimSpace(string(data))
		return &value, nil
	default:
		return nil, nil
	}
}

func matchModelID(prefix string, items []model.ModelProfile) (string, error) {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return id.MatchByPrefix(prefix, ids)
}
