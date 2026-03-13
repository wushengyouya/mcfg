package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"mcfg/internal/exitcode"
	"mcfg/internal/id"
	"mcfg/internal/model"
)

// DefaultBackupKeep 表示默认保留的有效备份数量。
const DefaultBackupKeep = 3

// BackupService 负责创建、列出、恢复和清理备份。
type BackupService struct {
	store      ConfigStore
	clock      Clock
	ids        id.Generator
	homeDir    string
	backupsDir string
	hooks      BackupTestHooks
}

// BackupRecord 表示备份及其完整性状态。
type BackupRecord struct {
	Meta      model.BackupMeta `json:"meta"`
	Corrupted bool             `json:"corrupted"`
}

// BackupTestHooks 提供测试时插入恢复故障的钩子。
type BackupTestHooks struct {
	BeforeRestoreSettings   func() error
	BeforeRestoreClaudeJSON func() error
}

// NewBackupService 创建备份服务实例。
func NewBackupService(store ConfigStore, homeDir, backupsDir string, clock Clock, gen id.Generator) *BackupService {
	return &BackupService{
		store:      store,
		clock:      defaultClock(clock),
		ids:        defaultIDGenerator(gen),
		homeDir:    homeDir,
		backupsDir: backupsDir,
	}
}

// SetTestHooks 设置备份恢复流程中的测试钩子。
func (s *BackupService) SetTestHooks(hooks BackupTestHooks) {
	s.hooks = hooks
}

// Create 为当前受管目标文件创建一次备份。
func (s *BackupService) Create(ctx context.Context, reason string) (model.BackupMeta, error) {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return model.BackupMeta{}, err
	}

	settingsPath, claudeJSONPath := backupTargetPaths(s.homeDir)
	settingsData, claudeJSONData, err := readRequiredTargets(settingsPath, claudeJSONPath)
	if err != nil {
		return model.BackupMeta{}, err
	}

	meta, err := createBackupSnapshot(settingsPath, claudeJSONPath, settingsData, claudeJSONData, s.backupsDir, s.clock, s.ids, reason)
	if err != nil {
		return model.BackupMeta{}, err
	}
	cfg.BackupIndex = append(cfg.BackupIndex, meta)
	autoPruneBackups(&cfg, s.backupsDir, DefaultBackupKeep)
	if err := s.store.Save(ctx, cfg); err != nil {
		return model.BackupMeta{}, err
	}
	return meta, nil
}

// List 返回全部备份记录，并标记磁盘上已损坏的备份。
func (s *BackupService) List(ctx context.Context) ([]BackupRecord, error) {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return nil, err
	}

	records := make([]BackupRecord, 0, len(cfg.BackupIndex))
	for _, meta := range cfg.BackupIndex {
		corrupted := false
		for _, file := range meta.Files {
			if _, err := os.Stat(file.BackupPath); err != nil {
				corrupted = true
				break
			}
		}
		records = append(records, BackupRecord{Meta: meta, Corrupted: corrupted})
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Meta.CreatedAt > records[j].Meta.CreatedAt
	})
	return records, nil
}

// Restore 根据 ID 前缀恢复指定备份。
func (s *BackupService) Restore(ctx context.Context, prefix string) (model.BackupMeta, error) {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return model.BackupMeta{}, err
	}

	index, err := findBackupIndex(cfg.BackupIndex, prefix)
	if err != nil {
		return model.BackupMeta{}, err
	}
	meta := cfg.BackupIndex[index]

	targetChecksums := map[string]string{}
	for _, file := range meta.Files {
		data, err := os.ReadFile(file.TargetPath)
		if err != nil {
			if os.IsNotExist(err) {
				return model.BackupMeta{}, fmt.Errorf("%w: restore target missing: %s", exitcode.ErrBusiness, file.TargetPath)
			}
			return model.BackupMeta{}, fmt.Errorf("%w: read restore target %s: %v", exitcode.ErrIO, file.TargetPath, err)
		}
		targetChecksums[file.TargetPath] = checksum(data)
	}

	for _, file := range meta.Files {
		data, err := os.ReadFile(file.BackupPath)
		if err != nil {
			return model.BackupMeta{}, fmt.Errorf("%w: read backup file %s: %v", exitcode.ErrIO, file.BackupPath, err)
		}
		var hook func() error
		switch file.TargetPath {
		case filepath.Join(s.homeDir, ".claude", "settings.json"):
			hook = s.hooks.BeforeRestoreSettings
		case filepath.Join(s.homeDir, ".claude.json"):
			hook = s.hooks.BeforeRestoreClaudeJSON
		}
		if err := writeAtomicChecked(file.TargetPath, data, targetChecksums[file.TargetPath], hook); err != nil {
			return model.BackupMeta{}, err
		}
	}
	return meta, nil
}

// Prune 删除超出保留数量的备份。
func (s *BackupService) Prune(ctx context.Context, keep int) (int, error) {
	if keep < 1 {
		return 0, fmt.Errorf("%w: keep must be >= 1", exitcode.ErrParam)
	}

	cfg, err := s.store.Load(ctx)
	if err != nil {
		return 0, err
	}
	records := make([]BackupRecord, 0, len(cfg.BackupIndex))
	for _, meta := range cfg.BackupIndex {
		corrupted := false
		for _, file := range meta.Files {
			if _, err := os.Stat(file.BackupPath); err != nil {
				corrupted = true
				break
			}
		}
		records = append(records, BackupRecord{Meta: meta, Corrupted: corrupted})
	}
	sort.Slice(records, func(i, j int) bool {
		left, leftErr := time.Parse(time.RFC3339, records[i].Meta.CreatedAt)
		right, rightErr := time.Parse(time.RFC3339, records[j].Meta.CreatedAt)
		if leftErr != nil || rightErr != nil {
			return records[i].Meta.CreatedAt > records[j].Meta.CreatedAt
		}
		return left.After(right)
	})

	keepSet := map[string]struct{}{}
	kept := 0
	for _, record := range records {
		if record.Corrupted {
			continue
		}
		if kept < keep {
			keepSet[record.Meta.ID] = struct{}{}
			kept++
		}
	}

	removed := 0
	nextIndex := make([]model.BackupMeta, 0, len(cfg.BackupIndex))
	for _, meta := range cfg.BackupIndex {
		_, shouldKeep := keepSet[meta.ID]
		if shouldKeep {
			nextIndex = append(nextIndex, meta)
			continue
		}
		removed++
		_ = os.RemoveAll(filepath.Dir(meta.Files[0].BackupPath))
	}
	cfg.BackupIndex = nextIndex
	if err := s.store.Save(ctx, cfg); err != nil {
		return 0, err
	}
	return removed, nil
}

func createBackupSnapshot(settingsPath, claudeJSONPath string, settingsData, claudeJSONData []byte, backupsDir string, clock Clock, gen id.Generator, reason string) (model.BackupMeta, error) {
	backupID, err := gen.New()
	if err != nil {
		return model.BackupMeta{}, fmt.Errorf("%w: generate backup id: %v", exitcode.ErrBusiness, err)
	}
	dir := filepath.Join(backupsDir, backupID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return model.BackupMeta{}, fmt.Errorf("%w: create backup dir: %v", exitcode.ErrIO, err)
	}

	settingsBackupPath := filepath.Join(dir, "settings.json")
	claudeBackupPath := filepath.Join(dir, "claude.json")
	if err := os.WriteFile(settingsBackupPath, settingsData, 0o600); err != nil {
		return model.BackupMeta{}, fmt.Errorf("%w: write settings backup: %v", exitcode.ErrIO, err)
	}
	if err := os.WriteFile(claudeBackupPath, claudeJSONData, 0o600); err != nil {
		return model.BackupMeta{}, fmt.Errorf("%w: write claude backup: %v", exitcode.ErrIO, err)
	}

	return model.BackupMeta{
		ID:        backupID,
		Target:    "claude_code",
		Reason:    reason,
		CreatedAt: nowRFC3339(clock),
		Files: []model.BackupFile{
			{TargetPath: settingsPath, BackupPath: settingsBackupPath, ExistsBeforeBackup: true},
			{TargetPath: claudeJSONPath, BackupPath: claudeBackupPath, ExistsBeforeBackup: true},
		},
	}, nil
}

func readRequiredTargets(settingsPath, claudeJSONPath string) ([]byte, []byte, error) {
	settingsData, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("%w: target missing: %s", exitcode.ErrBusiness, settingsPath)
		}
		return nil, nil, fmt.Errorf("%w: read %s: %v", exitcode.ErrIO, settingsPath, err)
	}
	claudeJSONData, err := os.ReadFile(claudeJSONPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("%w: target missing: %s", exitcode.ErrBusiness, claudeJSONPath)
		}
		return nil, nil, fmt.Errorf("%w: read %s: %v", exitcode.ErrIO, claudeJSONPath, err)
	}
	return settingsData, claudeJSONData, nil
}

func backupTargetPaths(homeDir string) (string, string) {
	return filepath.Join(homeDir, ".claude", "settings.json"), filepath.Join(homeDir, ".claude.json")
}

func findBackupIndex(items []model.BackupMeta, prefix string) (int, error) {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	matched, err := id.MatchByPrefix(prefix, ids)
	if err != nil {
		return -1, err
	}
	for index, item := range items {
		if item.ID == matched {
			return index, nil
		}
	}
	return -1, fmt.Errorf("%w: backup %q not found", exitcode.ErrBusiness, prefix)
}

func autoPruneBackups(cfg *model.ConfigRoot, backupsDir string, keep int) {
	if keep < 1 || len(cfg.BackupIndex) <= keep {
		return
	}
	sort.Slice(cfg.BackupIndex, func(i, j int) bool {
		return cfg.BackupIndex[i].CreatedAt > cfg.BackupIndex[j].CreatedAt
	})
	toRemove := slices.Clone(cfg.BackupIndex[keep:])
	cfg.BackupIndex = slices.Clone(cfg.BackupIndex[:keep])
	for _, meta := range toRemove {
		_ = os.RemoveAll(filepath.Dir(meta.Files[0].BackupPath))
	}
}
