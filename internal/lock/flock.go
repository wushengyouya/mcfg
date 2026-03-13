package lock

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"mcfg/internal/exitcode"
)

// Mode 表示锁的持有模式。
type Mode string

const (
	// Shared 表示共享读锁。
	Shared Mode = "shared"
	// Exclusive 表示独占写锁。
	Exclusive Mode = "exclusive"
)

// Metadata 记录当前锁持有者的元数据。
type Metadata struct {
	PID       int    `json:"pid"`
	StartedAt string `json:"started_at"`
	Command   string `json:"command"`
	Mode      Mode   `json:"mode"`
}

// Manager 负责申请和释放文件锁。
type Manager struct {
	path     string
	metaPath string
	now      func() time.Time
}

// Handle 表示一次成功获取的锁句柄。
type Handle struct {
	file    *os.File
	manager *Manager
	mode    Mode
}

// New 创建指向指定锁文件路径的锁管理器。
func New(path string, now func() time.Time) *Manager {
	return &Manager{
		path:     path,
		metaPath: path + ".meta",
		now:      now,
	}
}

// Acquire 以给定模式申请锁，并在独占锁成功时写入持有者元数据。
func (m *Manager) Acquire(mode Mode, command string) (*Handle, error) {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return nil, fmt.Errorf("%w: create lock directory: %v", exitcode.ErrIO, err)
	}

	file, err := os.OpenFile(m.path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("%w: open lock file: %v", exitcode.ErrIO, err)
	}

	flag := syscall.LOCK_NB
	if mode == Exclusive {
		flag |= syscall.LOCK_EX
	} else {
		flag |= syscall.LOCK_SH
	}
	if err := syscall.Flock(int(file.Fd()), flag); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, m.conflictError()
		}
		return nil, fmt.Errorf("%w: acquire lock: %v", exitcode.ErrIO, err)
	}

	handle := &Handle{file: file, manager: m, mode: mode}
	if mode == Exclusive {
		if err := m.writeMetadata(file, Metadata{
			PID:       os.Getpid(),
			StartedAt: m.now().UTC().Format(time.RFC3339),
			Command:   strings.TrimSpace(command),
			Mode:      mode,
		}); err != nil {
			_ = handle.Release()
			return nil, err
		}
	} else {
		_ = os.Remove(m.metaPath)
	}
	return handle, nil
}

// Release 释放锁并清理独占锁留下的元数据文件。
func (h *Handle) Release() error {
	if h == nil || h.file == nil {
		return nil
	}
	if h.mode == Exclusive {
		_ = os.Remove(h.manager.metaPath)
	}
	err := h.file.Close()
	h.file = nil
	if err != nil {
		return fmt.Errorf("%w: close lock file: %v", exitcode.ErrIO, err)
	}
	return nil
}

func (m *Manager) writeMetadata(file *os.File, metadata Metadata) error {
	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("%w: encode lock metadata: %v", exitcode.ErrIO, err)
	}
	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("%w: truncate lock file: %v", exitcode.ErrIO, err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("%w: seek lock file: %v", exitcode.ErrIO, err)
	}
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("%w: write lock metadata: %v", exitcode.ErrIO, err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("%w: sync lock file: %v", exitcode.ErrIO, err)
	}
	if err := os.WriteFile(m.metaPath, data, 0o600); err != nil {
		return fmt.Errorf("%w: write lock meta file: %v", exitcode.ErrIO, err)
	}
	return nil
}

func (m *Manager) conflictError() error {
	if metadata, ok := m.readMetadata(); ok && metadata.Mode == Exclusive {
		advice := "close the existing TUI or wait for the running command to finish"
		if metadata.Command != "" && !strings.Contains(metadata.Command, "tui") {
			advice = "wait for the running command to finish"
		}
		return fmt.Errorf("%w: mcfg is locked by pid %d started at %s (%s); %s", exitcode.ErrLock, metadata.PID, metadata.StartedAt, metadata.Command, advice)
	}
	return fmt.Errorf("%w: mcfg is locked by another read-only command; wait for it to finish", exitcode.ErrLock)
}

func (m *Manager) readMetadata() (Metadata, bool) {
	for _, path := range []string{m.metaPath, m.path} {
		data, err := os.ReadFile(path)
		if err != nil || len(data) == 0 {
			continue
		}
		var metadata Metadata
		if err := json.Unmarshal(data, &metadata); err == nil && metadata.PID != 0 {
			return metadata, true
		}
	}
	return Metadata{}, false
}
