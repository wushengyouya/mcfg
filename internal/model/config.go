package model

import (
	"encoding/json"
	"fmt"
)

const SchemaVersion = 1

type Source string

const (
	SourceManual   Source = "manual"
	SourceImported Source = "imported"
)

func (s Source) Valid() bool {
	return s == SourceManual || s == SourceImported
}

func (s *Source) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	source := Source(value)
	if !source.Valid() {
		return fmt.Errorf("invalid source %q", value)
	}
	*s = source
	return nil
}

type ConfigRoot struct {
	SchemaVersion int            `json:"schema_version"`
	Models        []ModelProfile `json:"models"`
	MCPServers    []MCPServer    `json:"mcp_servers"`
	ClaudeBinding ClaudeBinding  `json:"claude_binding"`
	BackupIndex   []BackupMeta   `json:"backup_index"`
}

type ModelProfile struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Env         map[string]string `json:"env"`
	Source      Source            `json:"source"`
	Description string            `json:"description,omitempty"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

type MCPServer struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Transport   string            `json:"transport"`
	Command     string            `json:"command"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Source      Source            `json:"source"`
	Description string            `json:"description,omitempty"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

type ClaudeBinding struct {
	CurrentModelID string   `json:"current_model_id"`
	EnabledMCPIDs  []string `json:"enabled_mcp_ids"`
	LastSyncAt     string   `json:"last_sync_at"`
	LastSyncResult string   `json:"last_sync_result"`
}

type BackupMeta struct {
	ID         string       `json:"id"`
	Target     string       `json:"target"`
	Files      []BackupFile `json:"files"`
	Reason     string       `json:"reason"`
	CreatedAt  string       `json:"created_at"`
	SourceHash string       `json:"source_hash,omitempty"`
}

type BackupFile struct {
	TargetPath         string `json:"target_path"`
	BackupPath         string `json:"backup_path"`
	ExistsBeforeBackup bool   `json:"exists_before_backup"`
}

func NewConfigRoot() ConfigRoot {
	return ConfigRoot{
		SchemaVersion: SchemaVersion,
		Models:        []ModelProfile{},
		MCPServers:    []MCPServer{},
		ClaudeBinding: ClaudeBinding{
			EnabledMCPIDs: []string{},
		},
		BackupIndex: []BackupMeta{},
	}
}

func (c *ConfigRoot) Normalize() {
	if c.SchemaVersion == 0 {
		c.SchemaVersion = SchemaVersion
	}
	if c.Models == nil {
		c.Models = []ModelProfile{}
	}
	if c.MCPServers == nil {
		c.MCPServers = []MCPServer{}
	}
	if c.ClaudeBinding.EnabledMCPIDs == nil {
		c.ClaudeBinding.EnabledMCPIDs = []string{}
	}
	if c.BackupIndex == nil {
		c.BackupIndex = []BackupMeta{}
	}
}

func ParseConfigRoot(data []byte) (ConfigRoot, error) {
	cfg := NewConfigRoot()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ConfigRoot{}, err
	}
	cfg.Normalize()
	return cfg, nil
}

func (c ConfigRoot) Marshal() ([]byte, error) {
	cfg := c
	cfg.Normalize()
	return json.MarshalIndent(cfg, "", "  ")
}
