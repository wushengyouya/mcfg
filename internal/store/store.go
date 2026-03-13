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

// Store 负责管理 mcfg 本地配置文件及备份目录。
type Store struct {
	homeDir    string
	configDir  string
	configPath string
	backupsDir string
}

// New 根据用户主目录创建配置存储实例。
func New(homeDir string) *Store {
	configDir := filepath.Join(homeDir, ".mcfg")
	return &Store{
		homeDir:    homeDir,
		configDir:  configDir,
		configPath: filepath.Join(configDir, "config.json"),
		backupsDir: filepath.Join(configDir, "backups"),
	}
}

// ConfigDir 返回配置目录路径。
func (s *Store) ConfigDir() string { return s.configDir }

// ConfigPath 返回主配置文件路径。
func (s *Store) ConfigPath() string { return s.configPath }

// BackupsDir 返回备份目录路径。
func (s *Store) BackupsDir() string { return s.backupsDir }

// Init 初始化配置目录和默认配置文件。
func (s *Store) Init(ctx context.Context) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	// 先建目录再校正权限，保证配置中心默认只对当前用户可读写。
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

// Load 从磁盘读取并解析配置文件。
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

	// 所有配置解析都汇总到 model.ParseConfigRoot，确保默认值修正逻辑只维护一份。
	cfg, err := model.ParseConfigRoot(data)
	if err != nil {
		return model.ConfigRoot{}, fmt.Errorf("%w: parse config: %v", exitcode.ErrIO, err)
	}
	return cfg, nil
}

// Save 以原子替换方式将配置写回磁盘。
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

	// 采用临时文件 + rename 的方式落盘，避免进程中断时写出半截配置文件。
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
