package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"mcfg/internal/adapter"
	"mcfg/internal/exitcode"
	"mcfg/internal/validator"
)

type ValidateService struct {
	store   ConfigStore
	adapter adapter.Claude
	homeDir string
}

type ValidateReport struct {
	Valid      bool              `json:"valid"`
	SyncStatus string            `json:"sync_status"`
	Errors     []validator.Issue `json:"errors"`
	Warnings   []validator.Issue `json:"warnings"`
	Checks     map[string]string `json:"checks"`
	Drift      struct {
		ManagedPathsChanged []string `json:"managed_paths_changed"`
	} `json:"drift"`
}

func NewValidateService(store ConfigStore, homeDir string) *ValidateService {
	return &ValidateService{
		store:   store,
		adapter: adapter.Claude{HomeDir: homeDir},
		homeDir: homeDir,
	}
}

func (s *ValidateService) Validate(ctx context.Context) (ValidateReport, error) {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return ValidateReport{}, err
	}

	report := ValidateReport{
		Errors:   validator.ValidateConfigRoot(cfg),
		Warnings: []validator.Issue{},
		Checks: map[string]string{
			"config_consistency": "passed",
			"field_validation":   "passed",
			"target_syncability": "passed",
		},
	}
	if len(report.Errors) > 0 {
		report.Checks["config_consistency"] = "failed"
		report.Checks["field_validation"] = "failed"
	}

	settingsPath := filepath.Join(s.homeDir, ".claude", "settings.json")
	claudeJSONPath := filepath.Join(s.homeDir, ".claude.json")
	settingsData, settingsErr := os.ReadFile(settingsPath)
	claudeJSONData, claudeErr := os.ReadFile(claudeJSONPath)
	if settingsErr != nil || claudeErr != nil {
		report.SyncStatus = "unavailable"
		report.Checks["target_syncability"] = "failed"
		if settingsErr != nil {
			report.Errors = append(report.Errors, validator.Issue{Path: settingsPath, Code: "target_missing", Message: "settings.json unavailable"})
		}
		if claudeErr != nil {
			report.Errors = append(report.Errors, validator.Issue{Path: claudeJSONPath, Code: "target_missing", Message: ".claude.json unavailable"})
		}
		report.Valid = len(report.Errors) == 0
		return report, nil
	}

	desiredSettings, err := s.adapter.RenderSettings(settingsData, currentModel(cfg))
	if err != nil {
		return ValidateReport{}, fmt.Errorf("%w: parse settings.json: %v", exitcode.ErrBusiness, err)
	}
	desiredClaudeJSON, err := s.adapter.RenderClaudeJSON(claudeJSONData, enabledMCPs(cfg))
	if err != nil {
		return ValidateReport{}, fmt.Errorf("%w: parse .claude.json: %v", exitcode.ErrBusiness, err)
	}

	drift, err := adapter.DiffManagedPaths(settingsData, desiredSettings, claudeJSONData, desiredClaudeJSON, s.homeDir)
	if err != nil {
		report.SyncStatus = "unavailable"
		report.Checks["target_syncability"] = "failed"
		report.Errors = append(report.Errors, validator.Issue{Path: "<targets>", Code: "target_corrupted", Message: err.Error()})
		report.Valid = false
		return report, nil
	}
	report.Drift.ManagedPathsChanged = drift
	if len(drift) == 0 {
		report.SyncStatus = "in_sync"
	} else {
		report.SyncStatus = "out_of_sync"
	}
	report.Valid = len(report.Errors) == 0
	return report, nil
}
