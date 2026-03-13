package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"mcfg/internal/adapter"
	"mcfg/internal/exitcode"
	"mcfg/internal/id"
	"mcfg/internal/model"
)

// SyncService 负责将本地配置中心同步到 Claude Code 目标文件。
type SyncService struct {
	store      ConfigStore
	adapter    adapter.Claude
	clock      Clock
	ids        id.Generator
	homeDir    string
	backupsDir string
	hooks      SyncTestHooks
}

// SyncOptions 描述一次同步操作的执行选项。
type SyncOptions struct {
	DryRun     bool
	InitTarget bool
}

// SyncResult 描述一次同步操作的结果。
type SyncResult struct {
	ChangedPaths []string `json:"changed_paths"`
	BackupID     string   `json:"backup_id,omitempty"`
}

// SyncTestHooks 提供测试时插入故障的钩子。
type SyncTestHooks struct {
	BeforeWriteSettings   func() error
	BeforeWriteClaudeJSON func() error
}

// NewSyncService 创建同步服务实例。
func NewSyncService(store ConfigStore, homeDir, backupsDir string, clock Clock, gen id.Generator) *SyncService {
	return &SyncService{
		store:      store,
		adapter:    adapter.Claude{HomeDir: homeDir},
		clock:      defaultClock(clock),
		ids:        defaultIDGenerator(gen),
		homeDir:    homeDir,
		backupsDir: backupsDir,
	}
}

// SetTestHooks 设置同步流程中的测试钩子。
func (s *SyncService) SetTestHooks(hooks SyncTestHooks) {
	s.hooks = hooks
}

// Sync 将配置中心内容写入 Claude Code 的目标配置文件。
func (s *SyncService) Sync(ctx context.Context, options SyncOptions) (SyncResult, error) {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return SyncResult{}, err
	}

	// 如果目标文件缺失，可按需初始化骨架文件；否则直接失败，避免静默生成用户未预期的配置。
	settingsPath, claudeJSONPath := s.targetPaths()
	if err := ensureTargets(settingsPath, claudeJSONPath, s.homeDir, options.InitTarget); err != nil {
		return SyncResult{}, err
	}

	settingsData, err := os.ReadFile(settingsPath)
	if err != nil {
		return SyncResult{}, fmt.Errorf("%w: read settings.json: %v", exitcode.ErrIO, err)
	}
	claudeJSONData, err := os.ReadFile(claudeJSONPath)
	if err != nil {
		return SyncResult{}, fmt.Errorf("%w: read .claude.json: %v", exitcode.ErrIO, err)
	}

	// 先基于现有文件渲染期望内容，只改 mcfg 接管的字段，不碰其他用户自定义配置。
	desiredSettings, err := s.adapter.RenderSettings(settingsData, currentModel(cfg))
	if err != nil {
		return SyncResult{}, fmt.Errorf("%w: parse settings.json: %v", exitcode.ErrBusiness, err)
	}
	desiredClaudeJSON, err := s.adapter.RenderClaudeJSON(claudeJSONData, enabledMCPs(cfg))
	if err != nil {
		return SyncResult{}, fmt.Errorf("%w: parse .claude.json: %v", exitcode.ErrBusiness, err)
	}

	changedPaths, err := adapter.DiffManagedPaths(settingsData, desiredSettings, claudeJSONData, desiredClaudeJSON, s.homeDir)
	if err != nil {
		return SyncResult{}, fmt.Errorf("%w: compare managed fields: %v", exitcode.ErrBusiness, err)
	}
	if options.DryRun {
		return SyncResult{ChangedPaths: changedPaths}, nil
	}

	// 真正写盘前先做备份并落索引，后续任一步失败都能回滚。
	backupID, err := s.createBackup(settingsPath, claudeJSONPath, settingsData, claudeJSONData, &cfg)
	if err != nil {
		return SyncResult{}, err
	}
	if err := s.store.Save(ctx, cfg); err != nil {
		return SyncResult{}, err
	}

	// 写入前校验原文件校验和，防止同步过程中被外部进程并发改写。
	settingsHash := checksum(settingsData)
	claudeHash := checksum(claudeJSONData)
	if err := writeAtomicChecked(settingsPath, desiredSettings, settingsHash, s.hooks.BeforeWriteSettings); err != nil {
		return SyncResult{}, err
	}
	if err := writeAtomicChecked(claudeJSONPath, desiredClaudeJSON, claudeHash, s.hooks.BeforeWriteClaudeJSON); err != nil {
		return SyncResult{}, s.rollbackAfterFailure(backupID, settingsPath, claudeJSONPath, err)
	}

	cfg.ClaudeBinding.LastSyncAt = nowRFC3339(s.clock)
	cfg.ClaudeBinding.LastSyncResult = "success"
	if err := s.store.Save(ctx, cfg); err != nil {
		return SyncResult{}, err
	}
	return SyncResult{ChangedPaths: changedPaths, BackupID: backupID}, nil
}

func (s *SyncService) rollbackAfterFailure(backupID, settingsPath, claudeJSONPath string, originalErr error) error {
	// settings.json 已写成功但 claude.json 失败时，需要把两个文件一起恢复到同步前状态。
	settingsBackupPath := filepath.Join(s.backupsDir, backupID, "settings.json")
	claudeBackupPath := filepath.Join(s.backupsDir, backupID, "claude.json")

	settingsBackup, err := os.ReadFile(settingsBackupPath)
	if err != nil {
		return fmt.Errorf("%w: sync failed: %v; rollback read failed: %v", exitcode.ErrIO, originalErr, err)
	}
	claudeBackup, err := os.ReadFile(claudeBackupPath)
	if err != nil {
		return fmt.Errorf("%w: sync failed: %v; rollback read failed: %v", exitcode.ErrIO, originalErr, err)
	}
	if err := writeAtomic(settingsPath, settingsBackup); err != nil {
		return fmt.Errorf("%w: sync failed: %v; rollback failed: %v", exitcode.ErrIO, originalErr, err)
	}
	if err := writeAtomic(claudeJSONPath, claudeBackup); err != nil {
		return fmt.Errorf("%w: sync failed: %v; rollback failed: %v", exitcode.ErrIO, originalErr, err)
	}
	return originalErr
}

func (s *SyncService) createBackup(settingsPath, claudeJSONPath string, settingsData, claudeJSONData []byte, cfg *model.ConfigRoot) (string, error) {
	meta, err := createBackupSnapshot(settingsPath, claudeJSONPath, settingsData, claudeJSONData, s.backupsDir, s.clock, s.ids, "sync")
	if err != nil {
		return "", err
	}
	cfg.BackupIndex = append(cfg.BackupIndex, meta)
	autoPruneBackups(cfg, s.backupsDir, DefaultBackupKeep)
	return meta.ID, nil
}

func (s *SyncService) targetPaths() (string, string) {
	return filepath.Join(s.homeDir, ".claude", "settings.json"), filepath.Join(s.homeDir, ".claude.json")
}

func ensureTargets(settingsPath, claudeJSONPath, homeDir string, initTarget bool) error {
	// 只检查 mcfg 需要接管的两个 Claude 配置文件是否存在。
	missing := []string{}
	if _, err := os.Stat(settingsPath); err != nil {
		if os.IsNotExist(err) {
			missing = append(missing, settingsPath)
		} else {
			return fmt.Errorf("%w: stat settings.json: %v", exitcode.ErrIO, err)
		}
	}
	if _, err := os.Stat(claudeJSONPath); err != nil {
		if os.IsNotExist(err) {
			missing = append(missing, claudeJSONPath)
		} else {
			return fmt.Errorf("%w: stat .claude.json: %v", exitcode.ErrIO, err)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	if !initTarget {
		return fmt.Errorf("%w: target files missing: %v; use `mcfg sync --init-target`", exitcode.ErrBusiness, missing)
	}

	// 初始化骨架时保留 Claude 期望的数据结构，避免后续渲染逻辑处理 nil 分支。
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		return fmt.Errorf("%w: create claude directory: %v", exitcode.ErrIO, err)
	}
	if slices.Contains(missing, settingsPath) {
		if err := os.WriteFile(settingsPath, []byte("{\n  \"env\": {}\n}\n"), 0o600); err != nil {
			return fmt.Errorf("%w: create settings skeleton: %v", exitcode.ErrIO, err)
		}
	}
	if slices.Contains(missing, claudeJSONPath) {
		data, _ := json.MarshalIndent(map[string]any{
			"projects": map[string]any{
				homeDir: map[string]any{
					"mcpServers": map[string]any{},
				},
			},
		}, "", "  ")
		if err := os.WriteFile(claudeJSONPath, append(data, '\n'), 0o600); err != nil {
			return fmt.Errorf("%w: create claude.json skeleton: %v", exitcode.ErrIO, err)
		}
	}
	return nil
}

func writeAtomic(path string, data []byte) error {
	return writeAtomicChecked(path, data, "", nil)
}

func writeAtomicChecked(path string, data []byte, expectedChecksum string, beforeCheck func() error) error {
	// 和 Store.Save 一样走原子替换，但这里额外支持写前钩子与并发修改检测。
	tmp, err := os.CreateTemp(filepath.Dir(path), "*.tmp")
	if err != nil {
		return fmt.Errorf("%w: create temp file: %v", exitcode.ErrIO, err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("%w: chmod temp file: %v", exitcode.ErrIO, err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("%w: write temp file: %v", exitcode.ErrIO, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("%w: close temp file: %v", exitcode.ErrIO, err)
	}
	if beforeCheck != nil {
		if err := beforeCheck(); err != nil {
			return err
		}
	}
	if expectedChecksum != "" {
		// 二次读取目标文件，对比写前快照，发现外部改动时中止覆盖。
		current, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("%w: external modification detected for %s; run `mcfg validate` and retry", exitcode.ErrBusiness, path)
			}
			return fmt.Errorf("%w: re-read target file %s: %v", exitcode.ErrIO, path, err)
		}
		if checksum(current) != expectedChecksum {
			return fmt.Errorf("%w: external modification detected for %s; run `mcfg validate` and retry", exitcode.ErrBusiness, path)
		}
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("%w: replace target file: %v", exitcode.ErrIO, err)
	}
	cleanup = false
	return nil
}

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func currentModel(cfg model.ConfigRoot) *model.ModelProfile {
	for _, item := range cfg.Models {
		if item.ID == cfg.ClaudeBinding.CurrentModelID {
			// 返回副本而不是原切片元素地址，避免调用方持有悬挂引用。
			copy := item
			return &copy
		}
	}
	return nil
}

func enabledMCPs(cfg model.ConfigRoot) []model.MCPServer {
	// 按配置顺序筛出已启用的 MCP，渲染时只输出当前绑定集合。
	enabled := make([]model.MCPServer, 0, len(cfg.ClaudeBinding.EnabledMCPIDs))
	for _, server := range cfg.MCPServers {
		if slices.Contains(cfg.ClaudeBinding.EnabledMCPIDs, server.ID) {
			enabled = append(enabled, server)
		}
	}
	return enabled
}
