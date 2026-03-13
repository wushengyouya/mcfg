package service

import (
	"context"
	"fmt"

	"mcfg/internal/exitcode"
	"mcfg/internal/scanner"
)

// ImportService 负责将 Claude 现有配置导入本地配置中心。
type ImportService struct {
	store   ConfigStore
	scanner *scanner.Scanner
}

// ImportSummary 描述一次导入操作的汇总结果。
type ImportSummary struct {
	ImportedModels int               `json:"imported_models"`
	ImportedMCPs   int               `json:"imported_mcps"`
	Skipped        int               `json:"skipped"`
	Warnings       []scanner.Warning `json:"warnings"`
}

// NewImportService 创建导入服务实例。
func NewImportService(store ConfigStore, scan *scanner.Scanner) *ImportService {
	return &ImportService{store: store, scanner: scan}
}

// Import 扫描 Claude 配置并将新条目写入本地配置中心。
func (s *ImportService) Import(ctx context.Context) (ImportSummary, error) {
	cfg, err := s.store.Load(ctx)
	if err != nil {
		return ImportSummary{}, fmt.Errorf("%w: run `mcfg init` first", exitcode.ErrBusiness)
	}

	result, err := s.scanner.Scan(ctx, cfg)
	if err != nil {
		return ImportSummary{}, err
	}

	cfg.Models = append(cfg.Models, result.Models...)
	cfg.MCPServers = append(cfg.MCPServers, result.MCPServers...)
	if len(result.Models) > 0 || len(result.MCPServers) > 0 {
		if err := s.store.Save(ctx, cfg); err != nil {
			return ImportSummary{}, err
		}
	}

	return ImportSummary{
		ImportedModels: len(result.Models),
		ImportedMCPs:   len(result.MCPServers),
		Skipped:        result.Skipped,
		Warnings:       result.Warnings,
	}, nil
}
