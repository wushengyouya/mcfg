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

type Mode string

const (
	Shared    Mode = "shared"
	Exclusive Mode = "exclusive"
)

type Metadata struct {
	PID       int    `json:"pid"`
	StartedAt string `json:"started_at"`
	Command   string `json:"command"`
	Mode      Mode   `json:"mode"`
}

type Manager struct {
	path     string
	metaPath string
	now      func() time.Time
}

type Handle struct {
	file    *os.File
	manager *Manager
	mode    Mode
}

func New(path string, now func() time.Time) *Manager {
	return &Manager{
		path:     path,
		metaPath: path + ".meta",
		now:      now,
	}
}

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
