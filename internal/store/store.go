package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"mcfg/internal/exitcode"
	"mcfg/internal/model"
)

const (
	dirPerm  = 0o700
	filePerm = 0o600
)

type Store struct {
	homeDir    string
	configDir  string
	configPath string
	backupsDir string
}

func New(homeDir string) *Store {
	configDir := filepath.Join(homeDir, ".mcfg")
	return &Store{
		homeDir:    homeDir,
		configDir:  configDir,
		configPath: filepath.Join(configDir, "config.json"),
		backupsDir: filepath.Join(configDir, "backups"),
	}
}

func (s *Store) ConfigDir() string  { return s.configDir }
func (s *Store) ConfigPath() string { return s.configPath }
func (s *Store) BackupsDir() string { return s.backupsDir }

func (s *Store) Init(ctx context.Context) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	if err := os.MkdirAll(s.backupsDir, dirPerm); err != nil {
		return false, fmt.Errorf("%w: create config directories: %v", exitcode.ErrIO, err)
	}
	if err := os.Chmod(s.configDir, dirPerm); err != nil {
		return false, fmt.Errorf("%w: chmod config directory: %v", exitcode.ErrIO, err)
	}
	if err := os.Chmod(s.backupsDir, dirPerm); err != nil {
		return false, fmt.Errorf("%w: chmod backups directory: %v", exitcode.ErrIO, err)
	}

	if _, err := os.Stat(s.configPath); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("%w: stat config file: %v", exitcode.ErrIO, err)
	}

	cfg := model.NewConfigRoot()
	if err := s.Save(ctx, cfg); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) Load(ctx context.Context) (model.ConfigRoot, error) {
	select {
	case <-ctx.Done():
		return model.ConfigRoot{}, ctx.Err()
	default:
	}

	data, err := os.ReadFile(s.configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.ConfigRoot{}, fmt.Errorf("%w: config not found at %s", exitcode.ErrIO, s.configPath)
		}
		return model.ConfigRoot{}, fmt.Errorf("%w: read config: %v", exitcode.ErrIO, err)
	}

	cfg, err := model.ParseConfigRoot(data)
	if err != nil {
		return model.ConfigRoot{}, fmt.Errorf("%w: parse config: %v", exitcode.ErrIO, err)
	}
	return cfg, nil
}

func (s *Store) Save(ctx context.Context, cfg model.ConfigRoot) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := os.MkdirAll(s.configDir, dirPerm); err != nil {
		return fmt.Errorf("%w: create config directory: %v", exitcode.ErrIO, err)
	}

	data, err := cfg.Marshal()
	if err != nil {
		return fmt.Errorf("%w: marshal config: %v", exitcode.ErrIO, err)
	}

	tmpFile, err := os.CreateTemp(s.configDir, "config-*.tmp")
	if err != nil {
		return fmt.Errorf("%w: create temp config: %v", exitcode.ErrIO, err)
	}
	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		_ = tmpFile.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmpFile.Chmod(filePerm); err != nil {
		return fmt.Errorf("%w: chmod temp config: %v", exitcode.ErrIO, err)
	}
	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("%w: write temp config: %v", exitcode.ErrIO, err)
	}
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("%w: sync temp config: %v", exitcode.ErrIO, err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("%w: close temp config: %v", exitcode.ErrIO, err)
	}
	if err := os.Rename(tmpPath, s.configPath); err != nil {
		return fmt.Errorf("%w: replace config: %v", exitcode.ErrIO, err)
	}
	if err := os.Chmod(s.configPath, filePerm); err != nil {
		return fmt.Errorf("%w: chmod config: %v", exitcode.ErrIO, err)
	}

	cleanup = false
	return nil
}
