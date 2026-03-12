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

type SyncService struct {
	store      ConfigStore
	adapter    adapter.Claude
	clock      Clock
	ids        id.Generator
	homeDir    string
	backupsDir string
	hooks      SyncTestHooks
}

type SyncOptions struct {
	DryRun     bool
	InitTarget bool
}

type SyncResult struct {
	ChangedPaths []string `json:"changed_paths"`
	BackupID     string   `json:"backup_id,omitempty"`
}

type SyncTestHooks struct {
	BeforeWriteSettings   func() error
	BeforeWriteClaudeJSON func() error
}

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

func (s *SyncService) SetTestHooks(hooks SyncTestHooks) {
	s.hooks = hooks
}

func (s *SyncService) Sync(ctx context.Context, options SyncOptions) (SyncResult, error) {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return SyncResult{}, err
	}

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

	backupID, err := s.createBackup(settingsPath, claudeJSONPath, settingsData, claudeJSONData, &cfg)
	if err != nil {
		return SyncResult{}, err
	}
	if err := s.store.Save(ctx, cfg); err != nil {
		return SyncResult{}, err
	}

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
			homeDir: map[string]any{
				"mcpServers": map[string]any{},
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
			copy := item
			return &copy
		}
	}
	return nil
}

func enabledMCPs(cfg model.ConfigRoot) []model.MCPServer {
	enabled := make([]model.MCPServer, 0, len(cfg.ClaudeBinding.EnabledMCPIDs))
	for _, server := range cfg.MCPServers {
		if slices.Contains(cfg.ClaudeBinding.EnabledMCPIDs, server.ID) {
			enabled = append(enabled, server)
		}
	}
	return enabled
}
