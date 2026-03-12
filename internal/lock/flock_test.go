package lock_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"mcfg/internal/lock"
)

func TestExclusiveLock_Acquire(t *testing.T) {
	manager := lock.New(filepath.Join(t.TempDir(), "run.lock"), timeNow)
	handle, err := manager.Acquire(lock.Exclusive, "test acquire")
	require.NoError(t, err)
	require.NoError(t, handle.Release())
}

func TestExclusiveLock_WritesMetadata(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "run.lock")
	manager := lock.New(lockPath, timeNow)
	handle, err := manager.Acquire(lock.Exclusive, "test metadata")
	require.NoError(t, err)
	defer func() { require.NoError(t, handle.Release()) }()

	data, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	var metadata lock.Metadata
	require.NoError(t, json.Unmarshal(data, &metadata))
	require.Equal(t, "test metadata", metadata.Command)
	require.Equal(t, lock.Exclusive, metadata.Mode)
}

func TestExclusiveLock_Conflict(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "run.lock")
	holder := startLockHolder(t, lockPath, "exclusive", "holder-exclusive")
	defer holder.stop(t)

	manager := lock.New(lockPath, timeNow)
	_, err := manager.Acquire(lock.Exclusive, "contender")
	require.Error(t, err)
	require.Contains(t, err.Error(), "holder-exclusive")
}

func TestExclusiveLock_Conflict_ActionableMessage(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "run.lock")
	holder := startLockHolder(t, lockPath, "exclusive", "mcfg tui")
	defer holder.stop(t)

	manager := lock.New(lockPath, timeNow)
	_, err := manager.Acquire(lock.Exclusive, "contender")
	require.Error(t, err)
	require.Contains(t, err.Error(), "pid")
	require.Contains(t, err.Error(), "started at")
	require.Contains(t, err.Error(), "mcfg tui")
	require.Contains(t, err.Error(), "close the existing TUI")
}

func TestSharedLock_MultipleReaders(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "run.lock")
	holder := startLockHolder(t, lockPath, "shared", "holder-shared")
	defer holder.stop(t)

	manager := lock.New(lockPath, timeNow)
	handle, err := manager.Acquire(lock.Shared, "reader-2")
	require.NoError(t, err)
	require.NoError(t, handle.Release())
}

func TestSharedLock_BlocksWriter(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "run.lock")
	holder := startLockHolder(t, lockPath, "shared", "holder-shared")
	defer holder.stop(t)

	manager := lock.New(lockPath, timeNow)
	_, err := manager.Acquire(lock.Exclusive, "writer")
	require.Error(t, err)
	require.Contains(t, err.Error(), "read-only")
	require.Contains(t, err.Error(), "wait for it to finish")
}

func TestExclusiveLock_BlocksReader(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "run.lock")
	holder := startLockHolder(t, lockPath, "exclusive", "holder-exclusive")
	defer holder.stop(t)

	manager := lock.New(lockPath, timeNow)
	_, err := manager.Acquire(lock.Shared, "reader")
	require.Error(t, err)
}

func TestLockHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_LOCK_HELPER") != "1" {
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
	command := args[index+3]

	mode := lock.Shared
	if modeArg == "exclusive" {
		mode = lock.Exclusive
	}

	manager := lock.New(lockPath, timeNow)
	handle, err := manager.Acquire(mode, command)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(3)
	}
	defer func() { _ = handle.Release() }()

	fmt.Fprintln(os.Stdout, "ready")
	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadString('\n')
	os.Exit(0)
}

type lockHolder struct {
	cmd   *exec.Cmd
	stdin *bufio.Writer
}

func startLockHolder(t *testing.T, lockPath, mode, command string) lockHolder {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=TestLockHelperProcess", "--", lockPath, mode, command)
	cmd.Env = append(os.Environ(), "GO_WANT_LOCK_HELPER=1")
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())

	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "ready", strings.TrimSpace(line))

	return lockHolder{cmd: cmd, stdin: bufio.NewWriter(stdin)}
}

func (h lockHolder) stop(t *testing.T) {
	t.Helper()
	_, err := h.stdin.WriteString("stop\n")
	require.NoError(t, err)
	require.NoError(t, h.stdin.Flush())
	require.NoError(t, h.cmd.Wait())
}

func timeNow() time.Time {
	return time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)
}
