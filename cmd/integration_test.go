package cmd_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"mcfg/cmd"
	"mcfg/internal/exitcode"
	"mcfg/internal/lock"
)

func TestCLI_ModelList_EmptyOutput(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)

	stdout, _, code := executeInHome(t, home, nil, "model", "list")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "No models configured.")
}

func TestCLI_ModelAdd_Success(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)

	stdout, _, code := executeInHome(t, home, nil,
		"model", "add",
		"--name", "Claude Sonnet",
		"--base-url", "https://example.com",
		"--model", "claude-sonnet-4-0",
		"--auth-token", "secret",
	)
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "added model")
}

func TestCLI_ModelAdd_DuplicateName_Fails(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)

	_, _, code = executeInHome(t, home, nil,
		"model", "add",
		"--name", "Claude Sonnet",
		"--base-url", "https://example.com",
		"--model", "claude-sonnet-4-0",
		"--auth-token", "secret",
	)
	require.Equal(t, exitcode.Success, code)

	_, _, err := executeErrInHome(t, home, nil,
		"model", "add",
		"--name", "claude sonnet",
		"--base-url", "https://example.org",
		"--model", "claude-sonnet-4-1",
		"--auth-token", "secret-2",
	)
	require.Error(t, err)
	require.Equal(t, exitcode.Business, exitcode.FromError(err))
	require.Contains(t, err.Error(), "already exists")
	require.Contains(t, err.Error(), "choose a different name")
}

func TestCLI_Import_Success(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{"ANTHROPIC_AUTH_TOKEN":"secret","ANTHROPIC_BASE_URL":"https://example.com","ANTHROPIC_MODEL":"claude"}}`), 0o600))

	stdout, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "imported models=1")

	stdout, _, code = executeInHome(t, home, nil, "import")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "imported models=0")
}

func TestCLI_ModelAdd_MissingRequired(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)

	_, _, code = executeInHome(t, home, nil,
		"model", "add",
		"--name", "Claude Sonnet",
		"--base-url", "https://example.com",
	)
	require.Equal(t, exitcode.Param, code)
}

func TestCLI_ModelUse_Success(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)
	_, _, code = executeInHome(t, home, nil,
		"model", "add",
		"--name", "Claude Sonnet",
		"--base-url", "https://example.com",
		"--model", "claude-sonnet-4-0",
		"--auth-token", "secret",
	)
	require.Equal(t, exitcode.Success, code)

	listOutput, _, code := executeInHome(t, home, nil, "model", "list")
	require.Equal(t, exitcode.Success, code)
	fields := strings.Fields(listOutput)
	require.NotEmpty(t, fields)
	id := fields[0]

	_, _, code = executeInHome(t, home, nil, "model", "use", id[:8])
	require.Equal(t, exitcode.Success, code)

	statusOutput, _, code := executeInHome(t, home, nil, "status")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, statusOutput, "Claude Sonnet")
}

func TestCLI_ModelEdit_Success(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)
	_, _, code = executeInHome(t, home, nil,
		"model", "add",
		"--name", "Claude Sonnet",
		"--base-url", "https://example.com",
		"--model", "claude-sonnet-4-0",
		"--auth-token", "secret",
	)
	require.Equal(t, exitcode.Success, code)

	listOutput, _, code := executeInHome(t, home, nil, "model", "list")
	require.Equal(t, exitcode.Success, code)
	id := strings.Fields(listOutput)[0]

	stdout, _, code := executeInHome(t, home, nil,
		"model", "edit", id[:8],
		"--name", "Claude Opus",
		"--model", "claude-opus-4-1",
	)
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "updated model Claude Opus")

	listOutput, _, code = executeInHome(t, home, nil, "model", "list")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, listOutput, "Claude Opus")
	require.Contains(t, listOutput, "claude-opus-4-1")
}

func TestCLI_MCPEAdd_DuplicateName_Fails(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)

	_, _, code = executeInHome(t, home, nil, "mcp", "add", "--name", "filesystem", "--command", "npx")
	require.Equal(t, exitcode.Success, code)

	_, _, err := executeErrInHome(t, home, nil, "mcp", "add", "--name", "Filesystem", "--command", "uvx")
	require.Error(t, err)
	require.Equal(t, exitcode.Business, exitcode.FromError(err))
	require.Contains(t, err.Error(), "already exists")
	require.Contains(t, err.Error(), "choose a different name")
}

func TestCLI_ModelRemove_Success(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)
	_, _, code = executeInHome(t, home, nil,
		"model", "add",
		"--name", "Claude Sonnet",
		"--base-url", "https://example.com",
		"--model", "claude-sonnet-4-0",
		"--auth-token", "secret",
	)
	require.Equal(t, exitcode.Success, code)

	listOutput, _, code := executeInHome(t, home, nil, "model", "list")
	require.Equal(t, exitcode.Success, code)
	id := strings.Fields(listOutput)[0]

	stdout, _, code := executeInHome(t, home, nil, "model", "remove", id[:8])
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "removed model")

	listOutput, _, code = executeInHome(t, home, nil, "model", "list")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, listOutput, "No models configured.")
}

func TestCLI_ModelRemove_BoundNoForce(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)
	_, _, code = executeInHome(t, home, nil,
		"model", "add",
		"--name", "Claude Sonnet",
		"--base-url", "https://example.com",
		"--model", "claude-sonnet-4-0",
		"--auth-token", "secret",
	)
	require.Equal(t, exitcode.Success, code)

	listOutput, _, code := executeInHome(t, home, nil, "model", "list")
	require.Equal(t, exitcode.Success, code)
	id := strings.Fields(listOutput)[0]
	_, _, code = executeInHome(t, home, nil, "model", "use", id[:8])
	require.Equal(t, exitcode.Success, code)

	_, _, code = executeInHome(t, home, nil, "model", "remove", id[:8])
	require.Equal(t, exitcode.Business, code)
}

func TestCLI_ModelRemove_BoundNoForce_Message(t *testing.T) {
	home := t.TempDir()
	_, _, err := executeErrInHome(t, home, nil, "init")
	require.NoError(t, err)
	_, _, err = executeErrInHome(t, home, nil,
		"model", "add",
		"--name", "Claude Sonnet",
		"--base-url", "https://example.com",
		"--model", "claude-sonnet-4-0",
		"--auth-token", "secret",
	)
	require.NoError(t, err)

	listOutput, _, err := executeErrInHome(t, home, nil, "model", "list")
	require.NoError(t, err)
	id := strings.Fields(listOutput)[0]

	_, _, err = executeErrInHome(t, home, nil, "model", "use", id[:8])
	require.NoError(t, err)

	_, _, err = executeErrInHome(t, home, nil, "model", "remove", id[:8])
	require.Error(t, err)
	require.Contains(t, err.Error(), "mcfg model use <other-id>")
	require.Contains(t, err.Error(), "mcfg model remove "+id[:8]+" --force")
}

func TestCLI_ModelRemove_Force_Message(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)
	_, _, code = executeInHome(t, home, nil,
		"model", "add",
		"--name", "Claude Sonnet",
		"--base-url", "https://example.com",
		"--model", "claude-sonnet-4-0",
		"--auth-token", "secret",
	)
	require.Equal(t, exitcode.Success, code)

	listOutput, _, code := executeInHome(t, home, nil, "model", "list")
	require.Equal(t, exitcode.Success, code)
	id := strings.Fields(listOutput)[0]
	_, _, code = executeInHome(t, home, nil, "model", "use", id[:8])
	require.Equal(t, exitcode.Success, code)

	stdout, _, code := executeInHome(t, home, nil, "model", "remove", id[:8], "--force")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "removed model")
	require.Contains(t, stdout, "cleared current model binding")
}

func TestCLI_ModelUse_WithSync_Success(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), projectScopedClaudeJSON(home, `{}`), 0o600))

	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)
	_, _, code = executeInHome(t, home, nil,
		"model", "add",
		"--name", "Claude Sonnet",
		"--base-url", "https://example.com",
		"--model", "claude-sonnet-4-0",
		"--auth-token", "secret",
	)
	require.Equal(t, exitcode.Success, code)

	listOutput, _, code := executeInHome(t, home, nil, "model", "list")
	require.Equal(t, exitcode.Success, code)
	id := strings.Fields(listOutput)[0]

	stdout, _, code := executeInHome(t, home, nil, "model", "use", id[:8], "--sync")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "synced")
}

func TestCLI_ModelUse_WithSync_RollbackOnFailure(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)
	_, _, code = executeInHome(t, home, nil,
		"model", "add",
		"--name", "Claude Sonnet",
		"--base-url", "https://example.com",
		"--model", "claude-sonnet-4-0",
		"--auth-token", "secret",
	)
	require.Equal(t, exitcode.Success, code)

	listOutput, _, code := executeInHome(t, home, nil, "model", "list")
	require.Equal(t, exitcode.Success, code)
	id := strings.Fields(listOutput)[0]

	_, _, code = executeInHome(t, home, nil, "model", "use", id[:8], "--sync")
	require.Equal(t, exitcode.Business, code)

	statusOutput, _, code := executeInHome(t, home, nil, "status")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, statusOutput, "Current model: (none)")
}

func TestCLI_Sync_InitTarget_Success(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)
	_, _, code = executeInHome(t, home, nil,
		"model", "add",
		"--name", "Claude Sonnet",
		"--base-url", "https://example.com",
		"--model", "claude-sonnet-4-0",
		"--auth-token", "secret",
	)
	require.Equal(t, exitcode.Success, code)

	listOutput, _, code := executeInHome(t, home, nil, "model", "list")
	require.Equal(t, exitcode.Success, code)
	id := strings.Fields(listOutput)[0]
	_, _, code = executeInHome(t, home, nil, "model", "use", id[:8])
	require.Equal(t, exitcode.Success, code)

	stdout, _, code := executeInHome(t, home, nil, "sync", "--init-target")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "synced changes:")
}

func TestCLI_Validate_JSONOutput(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)

	stdout, _, code := executeInHome(t, home, nil, "validate", "--json")
	require.Equal(t, exitcode.Business, code)
	require.Contains(t, stdout, `"sync_status": "unavailable"`)
}

func TestCLI_Validate_JSONOutputFields(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)

	stdout, _, code := executeInHome(t, home, nil, "validate", "--json")
	require.Equal(t, exitcode.Business, code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	require.Contains(t, payload, "valid")
	require.Contains(t, payload, "sync_status")
	require.Contains(t, payload, "errors")
	require.Contains(t, payload, "warnings")
	require.Contains(t, payload, "checks")
	require.Contains(t, payload, "drift")
}

func TestCLI_Validate_HumanOutput(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)

	stdout, _, code := executeInHome(t, home, nil, "validate")
	require.Equal(t, exitcode.Business, code)
	require.Contains(t, stdout, "Summary")
	require.Contains(t, stdout, "Errors")
	require.Contains(t, stdout, "Warnings")
	require.Contains(t, stdout, "Target Drift")
}

func TestCLI_Status_Output(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)
	_, _, code = executeInHome(t, home, nil,
		"model", "add",
		"--name", "Claude Sonnet",
		"--base-url", "https://example.com",
		"--model", "claude-sonnet-4-0",
		"--auth-token", "secret",
	)
	require.Equal(t, exitcode.Success, code)
	_, _, code = executeInHome(t, home, nil, "mcp", "add", "--name", "filesystem", "--command", "npx")
	require.Equal(t, exitcode.Success, code)

	modelListOutput, _, code := executeInHome(t, home, nil, "model", "list")
	require.Equal(t, exitcode.Success, code)
	modelID := strings.Fields(modelListOutput)[0]
	_, _, code = executeInHome(t, home, nil, "model", "use", modelID[:8])
	require.Equal(t, exitcode.Success, code)

	mcpListOutput, _, code := executeInHome(t, home, nil, "mcp", "list")
	require.Equal(t, exitcode.Success, code)
	mcpID := strings.Fields(mcpListOutput)[0]
	_, _, code = executeInHome(t, home, nil, "mcp", "enable", mcpID[:8])
	require.Equal(t, exitcode.Success, code)

	stdout, _, code := executeInHome(t, home, nil, "status")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "Current model: Claude Sonnet")
	require.Contains(t, stdout, "Enabled MCPs: 1 (filesystem)")
	require.Contains(t, stdout, "Sync status: never")
	require.Contains(t, stdout, "Last sync at: never")
}

func TestCLI_Status_JSONOutputFields(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)
	_, _, code = executeInHome(t, home, nil,
		"model", "add",
		"--name", "Claude Sonnet",
		"--base-url", "https://example.com",
		"--model", "claude-sonnet-4-0",
		"--auth-token", "secret",
	)
	require.Equal(t, exitcode.Success, code)
	_, _, code = executeInHome(t, home, nil, "mcp", "add", "--name", "filesystem", "--command", "npx")
	require.Equal(t, exitcode.Success, code)

	modelListOutput, _, code := executeInHome(t, home, nil, "model", "list")
	require.Equal(t, exitcode.Success, code)
	modelID := strings.Fields(modelListOutput)[0]
	_, _, code = executeInHome(t, home, nil, "model", "use", modelID[:8])
	require.Equal(t, exitcode.Success, code)

	mcpListOutput, _, code := executeInHome(t, home, nil, "mcp", "list")
	require.Equal(t, exitcode.Success, code)
	mcpID := strings.Fields(mcpListOutput)[0]
	_, _, code = executeInHome(t, home, nil, "mcp", "enable", mcpID[:8])
	require.Equal(t, exitcode.Success, code)

	stdout, _, code := executeInHome(t, home, nil, "status", "--json")
	require.Equal(t, exitcode.Success, code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	require.Contains(t, payload, "current_model_id")
	require.Contains(t, payload, "current_model_name")
	require.Contains(t, payload, "enabled_mcp_ids")
	require.Contains(t, payload, "enabled_mcp_names")
	require.Contains(t, payload, "sync_status")
	require.Contains(t, payload, "last_sync_at")
	require.Contains(t, payload, "last_sync_result")
	require.Equal(t, "Claude Sonnet", payload["current_model_name"])
	require.Equal(t, "never", payload["sync_status"])
}

func TestCLI_ModelList_JSONOutputFields(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)
	_, _, code = executeInHome(t, home, nil,
		"model", "add",
		"--name", "Claude Sonnet",
		"--base-url", "https://example.com",
		"--model", "claude-sonnet-4-0",
		"--auth-token", "secret",
	)
	require.Equal(t, exitcode.Success, code)

	stdout, _, code := executeInHome(t, home, nil, "model", "list", "--json")
	require.Equal(t, exitcode.Success, code)

	var items []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &items))
	require.Len(t, items, 1)
	require.Contains(t, items[0], "id")
	require.Contains(t, items[0], "name")
	require.Contains(t, items[0], "env")
	require.Contains(t, items[0], "source")
	require.Contains(t, items[0], "created_at")
	require.Contains(t, items[0], "updated_at")
}

func TestCLI_MCPList_JSONOutputFields(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)
	_, _, code = executeInHome(t, home, nil, "mcp", "add", "--name", "filesystem", "--command", "npx")
	require.Equal(t, exitcode.Success, code)

	stdout, _, code := executeInHome(t, home, nil, "mcp", "list", "--json")
	require.Equal(t, exitcode.Success, code)

	var items []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &items))
	require.Len(t, items, 1)
	require.Contains(t, items[0], "id")
	require.Contains(t, items[0], "name")
	require.Contains(t, items[0], "transport")
	require.Contains(t, items[0], "command")
	require.Contains(t, items[0], "source")
	require.Contains(t, items[0], "created_at")
	require.Contains(t, items[0], "updated_at")
}

func TestCLI_BackupCreate_AndList(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), projectScopedClaudeJSON(home, `{}`), 0o600))
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)

	createOutput, _, code := executeInHome(t, home, nil, "backup", "create")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, createOutput, "created backup")

	listOutput, _, code := executeInHome(t, home, nil, "backup", "list")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, listOutput, "manual")
}

func TestCLI_BackupCreate_JSON(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), projectScopedClaudeJSON(home, `{}`), 0o600))
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)

	output, _, code := executeInHome(t, home, nil, "backup", "create", "--json")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, output, `"target": "claude_code"`)
}

func TestCLI_BackupPrune_Success(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), projectScopedClaudeJSON(home, `{}`), 0o600))
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)

	_, _, code = executeInHome(t, home, nil, "backup", "create")
	require.Equal(t, exitcode.Success, code)
	_, _, code = executeInHome(t, home, nil, "backup", "create")
	require.Equal(t, exitcode.Success, code)

	output, _, code := executeInHome(t, home, nil, "backup", "prune", "--keep", "1")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, output, "pruned backups: 1")
}

func TestCLI_BackupPrune_JSON(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), projectScopedClaudeJSON(home, `{}`), 0o600))
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)
	_, _, code = executeInHome(t, home, nil, "backup", "create")
	require.Equal(t, exitcode.Success, code)

	output, _, code := executeInHome(t, home, nil, "backup", "prune", "--keep", "1", "--json")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, output, `"removed": 0`)
	require.Contains(t, output, `"keep": 1`)
}

func TestCLI_LockConflict_ExitCode2(t *testing.T) {
	home := t.TempDir()
	holder := startCLILockHolder(t, filepath.Join(home, ".mcfg", "run.lock"), "exclusive", "holder-exclusive")
	defer holder.stop(t)

	_, _, code := executeInHome(t, home, nil, "status")
	require.Equal(t, exitcode.LockConflict, code)
}

func TestCLI_LockConflict_MessageIncludesOwner(t *testing.T) {
	home := t.TempDir()
	holder := startCLILockHolder(t, filepath.Join(home, ".mcfg", "run.lock"), "exclusive", "mcfg tui")
	defer holder.stop(t)

	_, _, err := executeErrInHome(t, home, nil, "status")
	require.Error(t, err)
	require.Contains(t, err.Error(), "mcfg is locked by pid")
	require.Contains(t, err.Error(), "mcfg tui")
	require.Contains(t, err.Error(), "close the existing TUI")
}

func TestCLI_LockConflict_SharedHolderMessage(t *testing.T) {
	home := t.TempDir()
	holder := startCLILockHolder(t, filepath.Join(home, ".mcfg", "run.lock"), "shared", "mcfg status")
	defer holder.stop(t)

	_, _, err := executeErrInHome(t, home, nil, "import")
	require.Error(t, err)
	require.Contains(t, err.Error(), "read-only command")
	require.Contains(t, err.Error(), "wait for it to finish")
}

func TestCLI_TUICommand_MissingInitFails(t *testing.T) {
	home := t.TempDir()

	_, _, code := executeInHome(t, home, bytes.NewBufferString("q"), "tui")
	require.Equal(t, exitcode.IO, code)
}

func TestCLI_MCPEnable_Success(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)
	_, _, code = executeInHome(t, home, nil, "mcp", "add", "--name", "filesystem", "--command", "npx")
	require.Equal(t, exitcode.Success, code)

	listOutput, _, code := executeInHome(t, home, nil, "mcp", "list")
	require.Equal(t, exitcode.Success, code)
	id := strings.Fields(listOutput)[0]

	_, _, code = executeInHome(t, home, nil, "mcp", "enable", id[:8])
	require.Equal(t, exitcode.Success, code)

	statusOutput, _, code := executeInHome(t, home, nil, "status")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, statusOutput, "Enabled MCPs: 1")
}

func TestCLI_MCPDisable_Success(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)

	stdout, _, code := executeInHome(t, home, nil, "mcp", "add", "--name", "filesystem", "--command", "npx")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "added mcp")

	listOutput, _, code := executeInHome(t, home, nil, "mcp", "list")
	require.Equal(t, exitcode.Success, code)
	id := strings.Fields(listOutput)[0]

	_, _, code = executeInHome(t, home, nil, "mcp", "enable", id[:8])
	require.Equal(t, exitcode.Success, code)

	stdout, _, code = executeInHome(t, home, nil, "mcp", "disable", id[:8])
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "disabled mcp filesystem")

	statusOutput, _, code := executeInHome(t, home, nil, "status")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, statusOutput, "Enabled MCPs: 0")
}

func TestCLI_MCPRemove_EnabledNoForce_Message(t *testing.T) {
	home := t.TempDir()
	_, _, err := executeErrInHome(t, home, nil, "init")
	require.NoError(t, err)
	_, _, err = executeErrInHome(t, home, nil, "mcp", "add", "--name", "filesystem", "--command", "npx")
	require.NoError(t, err)

	listOutput, _, err := executeErrInHome(t, home, nil, "mcp", "list")
	require.NoError(t, err)
	id := strings.Fields(listOutput)[0]

	_, _, err = executeErrInHome(t, home, nil, "mcp", "enable", id[:8])
	require.NoError(t, err)

	_, _, err = executeErrInHome(t, home, nil, "mcp", "remove", id[:8])
	require.Error(t, err)
	require.Contains(t, err.Error(), "mcfg mcp disable "+id[:8])
	require.Contains(t, err.Error(), "mcfg mcp remove "+id[:8]+" --force")
}

func TestCLI_MCPRemove_Force_Message(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)
	_, _, code = executeInHome(t, home, nil, "mcp", "add", "--name", "filesystem", "--command", "npx")
	require.Equal(t, exitcode.Success, code)

	listOutput, _, code := executeInHome(t, home, nil, "mcp", "list")
	require.Equal(t, exitcode.Success, code)
	id := strings.Fields(listOutput)[0]
	_, _, code = executeInHome(t, home, nil, "mcp", "enable", id[:8])
	require.Equal(t, exitcode.Success, code)

	stdout, _, code := executeInHome(t, home, nil, "mcp", "remove", id[:8], "--force")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "removed mcp")
	require.Contains(t, stdout, "disabled before removal")
}

func TestCLI_ModelAdd_Help_ShowsExamples(t *testing.T) {
	stdout, _, code := execute(t, nil, "model", "add", "--help")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "Examples:")
	require.Contains(t, stdout, "mcfg model add --name \"Claude Sonnet\"")
	require.Contains(t, stdout, "mcfg model add --auth-token-stdin")
}

func TestCLI_Version_Command(t *testing.T) {
	stdout, _, code := execute(t, nil, "version")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "Version:")
	require.Contains(t, stdout, "dev")
	require.Contains(t, stdout, "Go:")
}

func TestCLI_Version_Command_JSON(t *testing.T) {
	stdout, _, code := execute(t, nil, "version", "--json")
	require.Equal(t, exitcode.Success, code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	require.Equal(t, "dev", payload["version"])
	require.Contains(t, payload, "commit")
	require.Contains(t, payload, "build_date")
	require.Contains(t, payload, "go_version")
	require.Contains(t, payload, "platform")
}

func TestCLI_Version_Help_ShowsExamples(t *testing.T) {
	stdout, _, code := execute(t, nil, "version", "--help")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "Examples:")
	require.Contains(t, stdout, "mcfg version")
	require.Contains(t, stdout, "mcfg version --json")
	require.Contains(t, stdout, "mcfg --version")
}

func TestCLI_RootVersionFlag(t *testing.T) {
	stdout, _, code := execute(t, nil, "--version")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "dev")
}

func TestCLI_RootHelp_ShowsExamples(t *testing.T) {
	stdout, _, code := execute(t, nil, "--help")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "Claude Code configuration manager")
	require.Contains(t, stdout, "Examples:")
	require.Contains(t, stdout, "mcfg init")
	require.Contains(t, stdout, "mcfg tui")
}

func TestCLI_Init_Help_ShowsExamples(t *testing.T) {
	stdout, _, code := execute(t, nil, "init", "--help")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "Examples:")
	require.Contains(t, stdout, "mcfg init")
}

func TestCLI_Status_Help_ShowsExamples(t *testing.T) {
	stdout, _, code := execute(t, nil, "status", "--help")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "Examples:")
	require.Contains(t, stdout, "mcfg status")
	require.Contains(t, stdout, "mcfg status --json")
}

func TestCLI_Sync_Help_ShowsExamples(t *testing.T) {
	stdout, _, code := execute(t, nil, "sync", "--help")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "Examples:")
	require.Contains(t, stdout, "mcfg sync --dry-run")
	require.Contains(t, stdout, "mcfg sync --init-target")
}

func TestCLI_Import_Help_ShowsExamples(t *testing.T) {
	stdout, _, code := execute(t, nil, "import", "--help")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "Examples:")
	require.Contains(t, stdout, "mcfg import")
	require.Contains(t, stdout, "mcfg import --json")
}

func TestCLI_Validate_Help_ShowsExamples(t *testing.T) {
	stdout, _, code := execute(t, nil, "validate", "--help")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "Examples:")
	require.Contains(t, stdout, "mcfg validate")
	require.Contains(t, stdout, "mcfg validate --json")
}

func TestCLI_BackupRestore_Help_ShowsExamples(t *testing.T) {
	stdout, _, code := execute(t, nil, "backup", "restore", "--help")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "Examples:")
	require.Contains(t, stdout, "mcfg backup restore <backup-id>")
	require.Contains(t, stdout, "mcfg backup restore <backup-id> --json")
}

func TestCLI_BackupList_Help_ShowsExamples(t *testing.T) {
	stdout, _, code := execute(t, nil, "backup", "list", "--help")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "Examples:")
	require.Contains(t, stdout, "mcfg backup list")
	require.Contains(t, stdout, "mcfg backup list --json")
}

func TestCLI_BackupPrune_Help_ShowsExamples(t *testing.T) {
	stdout, _, code := execute(t, nil, "backup", "prune", "--help")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "Examples:")
	require.Contains(t, stdout, "mcfg backup prune --keep 3")
	require.Contains(t, stdout, "mcfg backup prune --keep 3 --json")
}

func TestCLI_TUI_Help_ShowsExamples(t *testing.T) {
	stdout, _, code := execute(t, nil, "tui", "--help")
	require.Equal(t, exitcode.Success, code)
	require.Contains(t, stdout, "Examples:")
	require.Contains(t, stdout, "mcfg")
	require.Contains(t, stdout, "mcfg tui")
}

func TestCLI_ModelAdd_TokenMutualExclusion(t *testing.T) {
	home := t.TempDir()
	_, _, code := executeInHome(t, home, nil, "init")
	require.Equal(t, exitcode.Success, code)

	_, _, code = executeInHome(t, home, bytes.NewBufferString("secret"),
		"model", "add",
		"--name", "Claude Sonnet",
		"--base-url", "https://example.com",
		"--model", "claude-sonnet-4-0",
		"--auth-token", "secret",
		"--auth-token-stdin",
	)
	require.Equal(t, exitcode.Param, code)
}

func execute(t *testing.T, stdin *bytes.Buffer, args ...string) (string, string, int) {
	t.Helper()
	return executeInHome(t, t.TempDir(), stdin, args...)
}

func executeInHome(t *testing.T, home string, stdin *bytes.Buffer, args ...string) (string, string, int) {
	t.Helper()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	command := cmd.NewRootCommand(cmd.Options{
		HomeDir: home,
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
	})
	command.SetArgs(args)
	err := command.Execute()
	return stdout.String(), stderr.String(), exitcode.FromError(err)
}

func executeErrInHome(t *testing.T, home string, stdin *bytes.Buffer, args ...string) (string, string, error) {
	t.Helper()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	command := cmd.NewRootCommand(cmd.Options{
		HomeDir: home,
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
	})
	command.SetArgs(args)
	err := command.Execute()
	return stdout.String(), stderr.String(), err
}

func TestCLIHelperLockHolder(t *testing.T) {
	if os.Getenv("GO_WANT_CLI_LOCK_HELPER") != "1" {
		return
	}

	args := os.Args
	index := 0
	for index < len(args) && args[index] != "--" {
		index++
	}
	if index+3 >= len(args) {
		os.Exit(2)
	}
	lockPath := args[index+1]
	modeArg := args[index+2]
	commandSummary := args[index+3]

	mode := lock.Shared
	if modeArg == "exclusive" {
		mode = lock.Exclusive
	}
	handle, err := lock.New(lockPath, func() time.Time { return time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC) }).Acquire(mode, commandSummary)
	if err != nil {
		os.Exit(3)
	}
	defer func() { _ = handle.Release() }()

	_, _ = os.Stdout.WriteString("ready\n")
	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadString('\n')
	os.Exit(0)
}

type cliLockHolder struct {
	cmd   *exec.Cmd
	stdin *bufio.Writer
}

func startCLILockHolder(t *testing.T, lockPath, mode, command string) cliLockHolder {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=TestCLIHelperLockHolder", "--", lockPath, mode, command)
	cmd.Env = append(os.Environ(), "GO_WANT_CLI_LOCK_HELPER=1")
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())

	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "ready", strings.TrimSpace(line))

	return cliLockHolder{cmd: cmd, stdin: bufio.NewWriter(stdin)}
}

func (h cliLockHolder) stop(t *testing.T) {
	t.Helper()
	_, err := h.stdin.WriteString("stop\n")
	require.NoError(t, err)
	require.NoError(t, h.stdin.Flush())
	require.NoError(t, h.cmd.Wait())
}
