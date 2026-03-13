package model

import (
	"encoding/json"
	"fmt"
)

// SchemaVersion 表示当前配置文件格式版本。
const SchemaVersion = 1

// Source 表示配置条目的来源。
type Source string

const (
	// SourceManual 表示条目由用户手工创建。
	SourceManual Source = "manual"
	// SourceImported 表示条目由外部配置扫描导入。
	SourceImported Source = "imported"
)

// Valid 返回来源值是否受支持。
func (s Source) Valid() bool {
	// 只允许两种来源，避免配置文件里出现无法识别的来源值。
	return s == SourceManual || s == SourceImported
}

// UnmarshalJSON 反序列化并校验来源值。
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

// ConfigRoot 表示 mcfg 本地配置中心的完整配置树。
type ConfigRoot struct {
	SchemaVersion int            `json:"schema_version"`
	Models        []ModelProfile `json:"models"`
	MCPServers    []MCPServer    `json:"mcp_servers"`
	ClaudeBinding ClaudeBinding  `json:"claude_binding"`
	BackupIndex   []BackupMeta   `json:"backup_index"`
}

// ModelProfile 表示一个 Claude 模型配置档案。
type ModelProfile struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Env         map[string]string `json:"env"`
	Source      Source            `json:"source"`
	Description string            `json:"description,omitempty"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// MCPServer 表示一个可绑定到 Claude Code 的 MCP 服务器定义。
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

// ClaudeBinding 记录当前生效的模型和 MCP 绑定关系。
type ClaudeBinding struct {
	CurrentModelID string   `json:"current_model_id"`
	EnabledMCPIDs  []string `json:"enabled_mcp_ids"`
	LastSyncAt     string   `json:"last_sync_at"`
	LastSyncResult string   `json:"last_sync_result"`
}

// BackupMeta 描述一次备份的元信息。
type BackupMeta struct {
	ID         string       `json:"id"`
	Target     string       `json:"target"`
	Files      []BackupFile `json:"files"`
	Reason     string       `json:"reason"`
	CreatedAt  string       `json:"created_at"`
	SourceHash string       `json:"source_hash,omitempty"`
}

// BackupFile 描述单个目标文件对应的备份文件。
type BackupFile struct {
	TargetPath         string `json:"target_path"`
	BackupPath         string `json:"backup_path"`
	ExistsBeforeBackup bool   `json:"exists_before_backup"`
}

// NewConfigRoot 返回带默认值的空配置。
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

// Normalize 将配置中的零值字段修正为稳定可序列化的默认值。
func (c *ConfigRoot) Normalize() {
	// Normalize 负责把 nil 切片修正为空切片，避免后续序列化结果不稳定。
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

// ParseConfigRoot 解析配置字节并补齐默认值。
func ParseConfigRoot(data []byte) (ConfigRoot, error) {
	cfg := NewConfigRoot()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ConfigRoot{}, err
	}
	cfg.Normalize()
	return cfg, nil
}

// Marshal 将配置规范化后序列化为缩进 JSON。
func (c ConfigRoot) Marshal() ([]byte, error) {
	cfg := c
	// 序列化前再次规范化，确保写回磁盘的数据结构完整且字段顺序稳定。
	cfg.Normalize()
	return json.MarshalIndent(cfg, "", "  ")
}
