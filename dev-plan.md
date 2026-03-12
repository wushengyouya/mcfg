# mcfg — TDD 开发规划

> Claude Code 配置中心 CLI 工具 · V1 · Go + Cobra + Bubble Tea
> 基于 design.md v1.0，采用 **测试驱动开发（TDD）** 原则

---

## 0. TDD 总体原则

本项目全程遵循 **Red → Green → Refactor** 循环：

1. **Red**：先编写描述预期行为的失败测试
2. **Green**：用最少代码让测试通过
3. **Refactor**：在测试保护下优化结构、消除重复

附加约定：

- 每个功能模块 **先写测试文件，再写实现文件**
- 单元测试覆盖核心逻辑（数据模型、校验、服务层、适配器）
- 集成测试覆盖 CLI 命令链路（输入 → 业务执行 → 输出 + 退出码）
- 所有文件系统操作使用 **临时目录 / 接口抽象** 隔离，不依赖真实 `~/.mcfg/` 或 `~/.claude/`
- TUI 测试以 Bubble Tea `tea.Model` 的消息驱动测试为主

---

## 1. 项目结构（推荐）

```text
mcfg/
├── cmd/                        # Cobra 命令入口
│   ├── root.go
│   ├── init.go
│   ├── import.go
│   ├── status.go
│   ├── model.go
│   ├── mcp.go
│   ├── validate.go
│   ├── sync.go
│   ├── backup.go
│   └── tui.go
├── internal/
│   ├── model/                  # 数据模型定义
│   │   ├── config.go           # ConfigRoot, ModelProfile, MCPServer, ClaudeBinding, BackupMeta
│   │   └── config_test.go
│   ├── store/                  # 本地持久化（读写 ~/.mcfg/config.json）
│   │   ├── store.go
│   │   └── store_test.go
│   ├── id/                     # ULID 生成与前缀匹配
│   │   ├── ulid.go
│   │   └── ulid_test.go
│   ├── service/                # 业务服务层（Model / MCP / Binding / Import / Sync / Backup）
│   │   ├── model_service.go
│   │   ├── model_service_test.go
│   │   ├── mcp_service.go
│   │   ├── mcp_service_test.go
│   │   ├── binding_service.go
│   │   ├── binding_service_test.go
│   │   ├── import_service.go
│   │   ├── import_service_test.go
│   │   ├── sync_service.go
│   │   ├── sync_service_test.go
│   │   ├── backup_service.go
│   │   └── backup_service_test.go
│   ├── validator/              # 三层校验逻辑
│   │   ├── validator.go
│   │   └── validator_test.go
│   ├── adapter/                # 目标配置适配器
│   │   ├── claude.go           # Claude Code 适配器
│   │   └── claude_test.go
│   ├── lock/                   # 全局运行锁
│   │   ├── flock.go
│   │   └── flock_test.go
│   ├── scanner/                # Claude Code 配置扫描器
│   │   ├── scanner.go
│   │   └── scanner_test.go
│   ├── exitcode/               # 统一退出码
│   │   └── exitcode.go
│   └── tui/                    # Bubble Tea TUI
│       ├── app.go
│       ├── app_test.go
│       ├── pages/
│       │   ├── overview.go
│       │   ├── models.go
│       │   ├── mcps.go
│       │   ├── sync_preview.go
│       │   └── backups.go
│       └── components/
│           ├── nav.go
│           ├── statusbar.go
│           └── form.go
├── go.mod
├── go.sum
├── main.go
├── design.md
└── dev-plan.md                 # 本文件
```

---

## 2. 迭代规划

### 迭代一：配置中心基础能力

**迭代目标**：建立数据模型、本地存储、ID 机制、Model/MCP 管理 CLI，交付最小可用闭环。

#### Phase 1-1：数据模型定义与序列化

**测试先行清单**（`internal/model/config_test.go`）：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T1.1.1 | `TestConfigRoot_NewEmpty` | 空 ConfigRoot 的 `schema_version` 为 1，`models`/`mcp_servers` 为空数组，`claude_binding` 为零值对象 |
| T1.1.2 | `TestConfigRoot_SerializeRoundTrip` | JSON 序列化 → 反序列化后结构完全一致 |
| T1.1.3 | `TestModelProfile_Fields` | ModelProfile 包含 `id`/`name`/`env`/`source`/`description`/`created_at`/`updated_at` |
| T1.1.4 | `TestModelProfile_EnvPreservesUnknownKeys` | `env` 中的扩展字段在序列化后仍保留 |
| T1.1.5 | `TestMCPServer_Fields` | MCPServer 包含 `id`/`name`/`transport`/`command`/`args`/`env`/`source`/`description`/`created_at`/`updated_at` |
| T1.1.6 | `TestClaudeBinding_Fields` | ClaudeBinding 包含 `current_model_id`/`enabled_mcp_ids`/`last_sync_at`/`last_sync_result` |
| T1.1.7 | `TestBackupMeta_Fields` | BackupMeta 包含 `id`/`target`/`files`/`reason`/`created_at`/`source_hash` |
| T1.1.8 | `TestConfigRoot_DeserializeInvalidJSON` | 非法 JSON 输入时返回明确解析错误 |
| T1.1.9 | `TestTimeFields_RFC3339` | `created_at`/`updated_at` 以 RFC3339 格式序列化 |
| T1.1.10 | `TestSource_OnlyManualOrImported` | `source` 字段仅接受 `manual` / `imported` |

#### Phase 1-2：ULID 生成与前缀匹配

**测试先行清单**（`internal/id/ulid_test.go`）：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T1.2.1 | `TestGenerate_ReturnsValidULID` | 生成的 ID 符合 ULID 格式 |
| T1.2.2 | `TestGenerate_Unique` | 连续生成 1000 个 ID，无重复 |
| T1.2.3 | `TestMatchByPrefix_ExactMatch` | 完整 ID 精确匹配 |
| T1.2.4 | `TestMatchByPrefix_8CharPrefix` | 8 位前缀唯一匹配时返回唯一结果 |
| T1.2.5 | `TestMatchByPrefix_Ambiguous` | 前缀匹配到多个 ID 时返回歧义错误 |
| T1.2.6 | `TestMatchByPrefix_NoMatch` | 前缀无匹配时返回未找到错误 |
| T1.2.7 | `TestMatchByPrefix_TooShort` | 少于 8 位前缀时返回参数错误 |

#### Phase 1-3：本地存储能力

**测试先行清单**（`internal/store/store_test.go`）：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T1.3.1 | `TestInitStore_CreatesDirectories` | 初始化后 `<tmpdir>/.mcfg/` 与 `backups/` 目录存在 |
| T1.3.2 | `TestInitStore_CreatesEmptyConfig` | 初始化后 `config.json` 存在，内容为空的 ConfigRoot |
| T1.3.3 | `TestInitStore_FilePermissions` | `config.json` 权限为 `0600` |
| T1.3.4 | `TestLoad_ValidConfig` | 加载合法 config.json 后返回正确的 ConfigRoot |
| T1.3.5 | `TestLoad_FileNotFound` | 文件不存在时返回明确错误 |
| T1.3.6 | `TestLoad_CorruptedJSON` | 文件内容损坏时返回解析错误 |
| T1.3.7 | `TestSave_AtomicWrite` | 写入过程中断后不会产生半写文件 |
| T1.3.8 | `TestSave_PreservesPermissions` | 保存后文件权限仍为 `0600` |
| T1.3.9 | `TestSave_RoundTrip` | Save → Load 后数据一致 |

#### Phase 1-4：Model 服务层

**测试先行清单**（`internal/service/model_service_test.go`）：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T1.4.1 | `TestModelAdd_Success` | 传入合法参数后 models 列表新增一条记录，ID 为 ULID，source 为 `manual` |
| T1.4.2 | `TestModelAdd_MissingName` | 缺少 name 时返回参数错误 |
| T1.4.3 | `TestModelAdd_MissingToken` | 缺少 ANTHROPIC_AUTH_TOKEN 时返回参数错误 |
| T1.4.4 | `TestModelAdd_MissingModel` | 缺少 ANTHROPIC_MODEL 时返回参数错误 |
| T1.4.5 | `TestModelAdd_InvalidBaseURL` | base_url 非合法 HTTP/HTTPS URL 时返回参数错误 |
| T1.4.6 | `TestModelAdd_ExtraEnvPreserved` | 传入额外 env 键值对后保留在 `env` 中 |
| T1.4.7 | `TestModelList_Empty` | 空列表返回空数组 |
| T1.4.8 | `TestModelList_WithItems` | 添加多条后列表完整返回 |
| T1.4.9 | `TestModelEdit_UpdateName` | 仅修改 name 时其他字段不变 |
| T1.4.10 | `TestModelEdit_UpdateToken` | 修改 token 后 env 中 token 更新 |
| T1.4.11 | `TestModelEdit_UpdateBaseURL` | 修改 base_url 后 env 更新 |
| T1.4.12 | `TestModelEdit_UpdateModel` | 修改 model 后 env 更新 |
| T1.4.13 | `TestModelEdit_ClearEnv` | 使用 clear-env 标记后自定义 env 被清空 |
| T1.4.14 | `TestModelEdit_ReplaceEnv` | 传入新 env 整体替换自定义 env |
| T1.4.15 | `TestModelEdit_ReservedEnvRejected` | 通过 --env 传入保留字段时返回错误 |
| T1.4.16 | `TestModelEdit_NotFound` | ID 不存在时返回未找到错误 |
| T1.4.17 | `TestModelEdit_UpdatedAtRefreshed` | 编辑后 `updated_at` 更新 |
| T1.4.18 | `TestModelRemove_Success` | 未被绑定的模型可正常删除 |
| T1.4.19 | `TestModelRemove_Bound_NoForce` | 被 current_model_id 引用的模型，无 --force 时拒绝删除 |
| T1.4.20 | `TestModelRemove_Bound_WithForce` | --force 时先清空 current_model_id 再删除 |
| T1.4.21 | `TestModelRemove_NotFound` | ID 不存在时返回未找到错误 |
| T1.4.22 | `TestModelUse_Success` | 成功切换 current_model_id |
| T1.4.23 | `TestModelUse_NotFound` | ID 不存在时返回未找到错误 |

#### Phase 1-5：MCP 服务层

**测试先行清单**（`internal/service/mcp_service_test.go`）：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T1.5.1 | `TestMCPAdd_Success` | 传入合法参数后 mcp_servers 新增一条，transport 固定 `stdio` |
| T1.5.2 | `TestMCPAdd_MissingName` | 缺少 name 时返回参数错误 |
| T1.5.3 | `TestMCPAdd_MissingCommand` | 缺少 command 时返回参数错误 |
| T1.5.4 | `TestMCPAdd_WithArgsAndEnv` | args 和 env 正确保留 |
| T1.5.5 | `TestMCPAdd_EnvKeyValidation` | env key 不匹配 `[A-Za-z_][A-Za-z0-9_]*` 时返回错误 |
| T1.5.6 | `TestMCPAdd_EmptyArgsItem` | args 中包含空字符串时返回参数错误 |
| T1.5.7 | `TestMCPList_Empty` | 空列表返回空数组 |
| T1.5.8 | `TestMCPList_WithItems` | 添加多条后列表完整返回 |
| T1.5.9 | `TestMCPEdit_UpdateName` | 仅修改 name 时其他字段不变 |
| T1.5.10 | `TestMCPEdit_ReplaceArgs` | 传入新 args 整体替换 |
| T1.5.11 | `TestMCPEdit_ClearArgs` | clear-args 后 args 为空 |
| T1.5.12 | `TestMCPEdit_ReplaceEnv` | 传入新 env 整体替换 |
| T1.5.13 | `TestMCPEdit_ClearEnv` | clear-env 后 env 为空 |
| T1.5.14 | `TestMCPEdit_NotFound` | ID 不存在时返回未找到错误 |
| T1.5.15 | `TestMCPRemove_Success` | 未被启用的 MCP 可正常删除 |
| T1.5.16 | `TestMCPRemove_Enabled_NoForce` | 被 enabled_mcp_ids 引用时，无 --force 拒绝删除 |
| T1.5.17 | `TestMCPRemove_Enabled_WithForce` | --force 时先从 enabled_mcp_ids 移除再删除 |
| T1.5.18 | `TestMCPRemove_NotFound` | ID 不存在时返回未找到错误 |
| T1.5.19 | `TestMCPEnable_Success` | 启用后 ID 出现在 enabled_mcp_ids |
| T1.5.20 | `TestMCPEnable_AlreadyEnabled` | 已启用时返回成功 + `already enabled` |
| T1.5.21 | `TestMCPEnable_NotFound` | ID 不存在时返回未找到错误 |
| T1.5.22 | `TestMCPDisable_Success` | 禁用后 ID 从 enabled_mcp_ids 移除 |
| T1.5.23 | `TestMCPDisable_AlreadyDisabled` | 已禁用时返回成功 + `already disabled` |
| T1.5.24 | `TestMCPDisable_NotFound` | ID 不存在时返回未找到错误 |

#### Phase 1-6：CLI 命令冒烟测试

**测试先行清单**（`cmd/*_test.go` 或独立 `integration_test.go`）：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T1.6.1 | `TestCLI_ModelList_EmptyOutput` | `mcfg model list` 输出空列表，退出码 0 |
| T1.6.2 | `TestCLI_ModelAdd_Success` | `mcfg model add --name ... --base-url ... --model ... --auth-token ...` 退出码 0 |
| T1.6.3 | `TestCLI_ModelAdd_MissingRequired` | 缺少必填参数时退出码 4 |
| T1.6.4 | `TestCLI_ModelEdit_Success` | 编辑后 list 输出更新 |
| T1.6.5 | `TestCLI_ModelRemove_Success` | 删除后 list 不再包含该条目 |
| T1.6.6 | `TestCLI_ModelRemove_BoundNoForce` | 被绑定时无 --force 退出码 1 |
| T1.6.7 | `TestCLI_ModelUse_Success` | `mcfg model use <id>` 后 status 展示新模型 |
| T1.6.8 | `TestCLI_MCPAdd_Success` | `mcfg mcp add --name ... --command ...` 退出码 0 |
| T1.6.9 | `TestCLI_MCPEnable_Success` | 启用后 status 展示 |
| T1.6.10 | `TestCLI_MCPDisable_Success` | 禁用后 status 不展示 |
| T1.6.11 | `TestCLI_Status_Output` | `mcfg status` 输出当前模型、MCP 数量、同步状态 |
| T1.6.12 | `TestCLI_ExitCodes` | 各类错误场景返回正确退出码（0/1/3/4） |
| T1.6.13 | `TestCLI_ModelAdd_TokenMutualExclusion` | 同时传入 `--auth-token` 和 `--auth-token-stdin` 时退出码 4 |

#### Phase 1-7：字段校验

**测试先行清单**（`internal/validator/validator_test.go` 部分）：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T1.7.1 | `TestValidate_ModelNameEmpty` | name 为空时返回校验错误 |
| T1.7.2 | `TestValidate_ModelTokenEmpty` | ANTHROPIC_AUTH_TOKEN 为空时返回校验错误 |
| T1.7.3 | `TestValidate_ModelModelEmpty` | ANTHROPIC_MODEL 为空时返回校验错误 |
| T1.7.4 | `TestValidate_ModelBaseURL_Invalid` | base_url 非法时返回校验错误 |
| T1.7.5 | `TestValidate_ModelBaseURL_FTP` | scheme 为 ftp 时返回校验错误 |
| T1.7.6 | `TestValidate_MCPNameEmpty` | name 为空时返回校验错误 |
| T1.7.7 | `TestValidate_MCPCommandEmpty` | command 为空时返回校验错误 |
| T1.7.8 | `TestValidate_MCPTransportNotStdio` | transport 不为 stdio 时返回校验错误 |
| T1.7.9 | `TestValidate_MCPArgsContainsEmpty` | args 中含空字符串时返回校验错误 |
| T1.7.10 | `TestValidate_MCPEnvKeyInvalid` | env key 不匹配正则时返回校验错误 |
| T1.7.11 | `TestValidate_SourceInvalid` | source 非 manual/imported 时返回校验错误 |
| T1.7.12 | `TestValidate_TimeFieldInvalid` | 时间字段不符合 RFC3339 时返回校验错误 |

---

### 迭代二：初始化、导入、校验与同步

**迭代目标**：打通 Claude Code 配置发现、导入、校验、同步全链路。

#### Phase 2-1：Scanner — Claude Code 配置扫描

**测试先行清单**（`internal/scanner/scanner_test.go`）：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T2.1.1 | `TestScan_SettingsExists_ExtractModel` | `settings.json` 存在且含 env 时，提取出 ModelProfile |
| T2.1.2 | `TestScan_SettingsNotExists` | `settings.json` 不存在时返回提示信息而非致命错误 |
| T2.1.3 | `TestScan_SettingsCorrupted` | `settings.json` 内容损坏时返回损坏提示 |
| T2.1.4 | `TestScan_ClaudeJSON_ExtractMCPs` | `~/.claude.json` 中 `<home>.mcpServers` 正确提取多个 MCPServer |
| T2.1.5 | `TestScan_ClaudeJSON_NotExists` | `~/.claude.json` 不存在时返回提示 |
| T2.1.6 | `TestScan_ClaudeJSON_Corrupted` | `~/.claude.json` 损坏时返回提示 |
| T2.1.7 | `TestScan_BothCorrupted` | 两个文件都损坏时不写入任何结果 |
| T2.1.8 | `TestScan_DuplicateModel_Skipped` | 已存在相同 model+base_url 的模型时跳过 |
| T2.1.9 | `TestScan_DuplicateMCP_Skipped` | 已存在相同 transport+command+args+env 的 MCP 时跳过 |
| T2.1.10 | `TestScan_SameNameDifferentContent` | 同名但内容不同时视为不同配置，正常导入 |
| T2.1.11 | `TestScan_MCPMissingCommand` | MCP 缺少 command 时视为非法配置，跳过 |
| T2.1.12 | `TestScan_MCPMissingType` | MCP 缺少 type 时视为非法配置，跳过 |
| T2.1.13 | `TestScan_ImportedSourceMarked` | 导入的记录 source 为 `imported` |

#### Phase 2-2：Init 流程

**测试先行清单**（`internal/service/` 或 `cmd/init_test.go`）：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T2.2.1 | `TestInit_FirstRun_CreatesAll` | 首次执行创建 `~/.mcfg/`、`backups/`、`config.json` |
| T2.2.2 | `TestInit_FirstRun_TriggersImport` | 首次执行后触发一次扫描导入 |
| T2.2.3 | `TestInit_AlreadyInitialized` | config.json 已存在且合法时返回 `already initialized`，退出码 0 |
| T2.2.4 | `TestInit_DirExistsConfigMissing` | `~/.mcfg/` 存在但 config.json 缺失时补齐 config.json |
| T2.2.5 | `TestInit_ConfigCorrupted` | config.json 存在但损坏时返回非 0，拒绝覆盖 |
| T2.2.6 | `TestInit_DoesNotDeleteBackups` | 重复执行不删除已有 backups 目录内容 |

#### Phase 2-3：Import 流程

**测试先行清单**（`internal/service/import_service_test.go`）：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T2.3.1 | `TestImport_NoConfigCenter` | config.json 不存在时返回非 0 并提示先 init |
| T2.3.2 | `TestImport_NewModelsImported` | 扫描到新模型时正确写入 models |
| T2.3.3 | `TestImport_NewMCPsImported` | 扫描到新 MCP 时正确写入 mcp_servers |
| T2.3.4 | `TestImport_DuplicatesSkipped` | 重复项跳过并报告 skipped 数量 |
| T2.3.5 | `TestImport_RepeatedExecution` | 多次执行 import 不产生重复记录 |
| T2.3.6 | `TestImport_PreservesExistingConfig` | 不覆盖已有手动配置 |

#### Phase 2-4：Claude Code 适配器

**测试先行清单**（`internal/adapter/claude_test.go`）：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T2.4.1 | `TestAdapter_ModelToSettingsEnv` | ModelProfile 正确映射为 `settings.json` 的 `env` 对象 |
| T2.4.2 | `TestAdapter_MCPsToClaudeJSON` | MCPServer 列表正确映射为 `<home>.mcpServers` 结构 |
| T2.4.3 | `TestAdapter_PreservesUnmanagedFields` | settings.json 中非受管字段保留不变 |
| T2.4.4 | `TestAdapter_PreservesOtherPathNodes` | claude.json 中其他路径节点保留不变 |
| T2.4.5 | `TestAdapter_NoModelBound` | current_model_id 为空时 env 中受管字段为空或缺省 |
| T2.4.6 | `TestAdapter_EmptyMCPList` | enabled_mcp_ids 为空时 mcpServers 为空对象 |
| T2.4.7 | `TestAdapter_MCPWithEmptyArgs` | args 为空时输出中不含 args 键 |
| T2.4.8 | `TestAdapter_MCPWithEmptyEnv` | env 为空时输出中不含 env 键 |
| T2.4.9 | `TestAdapter_CreateEnvIfMissing` | 目标文件无 `env` 字段时，同步时主动创建 |

#### Phase 2-5：Validate 校验命令

**测试先行清单**（`internal/validator/validator_test.go` 补充）：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T2.5.1 | `TestValidate_ConsistencyCheck_DuplicateModelID` | models 中存在重复 ID 时返回 error |
| T2.5.2 | `TestValidate_ConsistencyCheck_DuplicateMCPID` | mcp_servers 中存在重复 ID 时返回 error |
| T2.5.3 | `TestValidate_ConsistencyCheck_BindingRefMissing` | current_model_id 引用不存在的 model 时返回 error |
| T2.5.4 | `TestValidate_ConsistencyCheck_EnabledMCPRefMissing` | enabled_mcp_ids 引用不存在的 MCP 时返回 error |
| T2.5.5 | `TestValidate_ConsistencyCheck_DuplicateEnabledMCP` | enabled_mcp_ids 有重复时返回 error |
| T2.5.6 | `TestValidate_ConsistencyCheck_SchemaVersion` | schema_version 非 1 时返回 error |
| T2.5.7 | `TestValidate_TargetSync_InSync` | 目标文件与本地配置一致时返回 `in_sync` |
| T2.5.8 | `TestValidate_TargetSync_OutOfSync` | 受管字段存在差异时返回 `out_of_sync` |
| T2.5.9 | `TestValidate_TargetSync_Unavailable` | 目标文件缺失时返回 `unavailable` |
| T2.5.10 | `TestValidate_TargetSync_Corrupted` | 目标文件损坏时返回 `unavailable` |
| T2.5.11 | `TestValidate_BackupIndex_MissingBothFiles` | backup_index 中的 files 不含两个目标文件记录时返回 error |
| T2.5.12 | `TestValidate_AllPass` | 全部校验通过时返回 valid，退出码 0 |
| T2.5.13 | `TestValidate_JSONOutput` | `--json` 输出包含 `valid`、`sync_status`、`errors`、`warnings`、`checks`、`drift` |
| T2.5.14 | `TestValidate_HumanOutput` | 人类可读输出包含 Summary / Errors / Warnings / Target Drift |

#### Phase 2-6：Sync 同步命令

**测试先行清单**（`internal/service/sync_service_test.go`）：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T2.6.1 | `TestSync_Success_WritesSettings` | 同步成功后 settings.json 的 env 受管字段已更新 |
| T2.6.2 | `TestSync_Success_WritesClaudeJSON` | 同步成功后 claude.json 的 mcpServers 已更新 |
| T2.6.3 | `TestSync_Success_PreservesUnmanaged` | 同步后非受管字段保持不变 |
| T2.6.4 | `TestSync_Success_UpdatesSyncResult` | 同步成功后 last_sync_at 和 last_sync_result 已更新 |
| T2.6.5 | `TestSync_TargetMissing_Fails` | 目标文件缺失时返回错误，提示使用 --init-target |
| T2.6.6 | `TestSync_InitTarget_CreatesFiles` | --init-target 时自动创建最小骨架文件 |
| T2.6.7 | `TestSync_InitTarget_NotOverwriteCorrupted` | --init-target 不覆盖已有损坏文件 |
| T2.6.8 | `TestSync_DryRun_NoWrite` | --dry-run 不修改任何文件 |
| T2.6.9 | `TestSync_DryRun_ShowsChanges` | --dry-run 输出变更预览 |
| T2.6.10 | `TestSync_AtomicWrite` | 写入使用临时文件 + 原子替换 |
| T2.6.11 | `TestSync_ModelUseSync_RollbackOnFailure` | model use --sync 同步失败时回滚 current_model_id |
| T2.6.12 | `TestSync_CreatesBackupBefore` | 同步前自动创建备份 |
| T2.6.13 | `TestSync_DryRun_TargetMissing_Fails` | --dry-run 时目标文件缺失仍报错 |

---

### 迭代三：可靠性、备份与并发保护

**迭代目标**：建立备份恢复、运行锁、外部修改检测、回滚机制。

#### Phase 3-1：全局运行锁

**测试先行清单**（`internal/lock/flock_test.go`）：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T3.1.1 | `TestExclusiveLock_Acquire` | 独占锁获取成功 |
| T3.1.2 | `TestExclusiveLock_WritesMetadata` | 获取后锁文件包含 pid、启动时间、命令摘要 |
| T3.1.3 | `TestExclusiveLock_Conflict` | 已有独占锁时第二个独占锁获取失败 |
| T3.1.4 | `TestSharedLock_MultipleReaders` | 多个共享锁可同时获取 |
| T3.1.5 | `TestSharedLock_BlocksWriter` | 有共享锁时独占锁获取失败 |
| T3.1.6 | `TestExclusiveLock_BlocksReader` | 有独占锁时共享锁获取失败 |
| T3.1.7 | `TestLock_Release` | 释放后其他进程可获取 |
| T3.1.8 | `TestLock_StaleLock_Recovery` | 锁文件存在但进程已死时可自动恢复 |
| T3.1.9 | `TestLock_ConflictMessage` | 锁冲突时错误信息包含 pid 和命令摘要 |
| T3.1.10 | `TestLock_ExitCode2` | 锁冲突时退出码为 2 |

#### Phase 3-2：备份创建与索引

**测试先行清单**（`internal/service/backup_service_test.go`）：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T3.2.1 | `TestBackupCreate_Success` | 创建备份后 backups 目录存在备份文件，backup_index 新增记录 |
| T3.2.2 | `TestBackupCreate_BothFilesIncluded` | 单次备份同时包含 settings.json 和 claude.json |
| T3.2.3 | `TestBackupCreate_TargetMissing_Fails` | 任一目标文件缺失时中止并提示 |
| T3.2.4 | `TestBackupCreate_MetadataComplete` | BackupMeta 包含完整的 id、target、files、reason、created_at |
| T3.2.5 | `TestBackupCreate_ULID_ID` | 备份 ID 为 ULID 格式 |

#### Phase 3-3：备份查询与恢复

**测试先行清单**：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T3.3.1 | `TestBackupList_Empty` | 空列表返回空 |
| T3.3.2 | `TestBackupList_WithItems` | 列表展示所有备份记录 |
| T3.3.3 | `TestBackupList_CorruptedEntry` | 索引存在但文件缺失时标记为损坏 |
| T3.3.4 | `TestBackupRestore_Success` | 恢复后两个目标文件内容与备份一致 |
| T3.3.5 | `TestBackupRestore_NotFound` | backup-id 不存在时返回错误 |
| T3.3.6 | `TestBackupRestore_FileMissing` | 备份文件缺失时返回错误 |
| T3.3.7 | `TestBackupRestore_TargetMissing_Fails` | 恢复前目标文件不存在时中止 |
| T3.3.8 | `TestBackupRestore_DoesNotUpdateConfigCenter` | 恢复后本地配置中心不变 |

#### Phase 3-4：备份清理策略

**测试先行清单**：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T3.4.1 | `TestAutoCleanup_KeepsLatest3` | 创建第 4 个备份后最旧的被自动清理 |
| T3.4.2 | `TestAutoCleanup_RemovesBothIndexAndFile` | 清理同时删除 backup_index 记录和备份文件 |
| T3.4.3 | `TestAutoCleanup_CorruptedEntryRemoved` | 损坏记录在清理时一并移除 |
| T3.4.4 | `TestPrune_DefaultKeep3` | `backup prune` 默认保留 3 个 |
| T3.4.5 | `TestPrune_CustomKeep` | `backup prune --keep 1` 保留 1 个 |
| T3.4.6 | `TestPrune_KeepMinimum1` | `--keep 0` 时返回参数错误 |
| T3.4.7 | `TestPrune_NoBackups` | 无备份时无操作 |
| T3.4.8 | `TestPrune_DoesNotCreateBackup` | prune 不创建新备份 |

#### Phase 3-5：回滚机制

**测试先行清单**：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T3.5.1 | `TestSync_WriteFail_Rollback` | settings.json 写入失败时自动回滚 |
| T3.5.2 | `TestSync_ClaudeJSONWriteFail_Rollback` | claude.json 写入失败时自动回滚 |
| T3.5.3 | `TestSync_RollbackRestoresOriginal` | 回滚后两个目标文件恢复原始内容 |
| T3.5.4 | `TestSync_FailureReportsReason` | 同步失败时返回明确失败原因 |

#### Phase 3-6：外部并发修改保护

**测试先行清单**：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T3.6.1 | `TestSync_ExternalModification_Detected` | 生成目标后文件被外部修改，原子替换前检测到不一致 |
| T3.6.2 | `TestSync_ExternalModification_Aborts` | 检测到外部修改后立即失败，不覆盖 |
| T3.6.3 | `TestSync_ExternalModification_ErrorMessage` | 错误信息包含受影响文件路径和重试提示 |
| T3.6.4 | `TestRestore_ExternalModification_Detected` | backup restore 前后文件摘要不一致时失败 |

#### Phase 3-7：CLI 集成测试补充

**测试先行清单**：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T3.7.1 | `TestCLI_LockConflict_ExitCode2` | 写命令遇到锁冲突时退出码 2 |
| T3.7.2 | `TestCLI_BackupCreate_Success` | `mcfg backup create` 退出码 0 |
| T3.7.3 | `TestCLI_BackupList_Output` | `mcfg backup list` 输出备份记录 |
| T3.7.4 | `TestCLI_BackupRestore_Success` | 恢复后目标文件一致 |
| T3.7.5 | `TestCLI_BackupPrune_Success` | prune 后备份数量符合保留策略 |
| T3.7.6 | `TestCLI_RemoveForce_Message` | `remove --force` 输出解除引用提示 |
| T3.7.7 | `TestCLI_SensitiveToken_Masked` | 输出中 token 值已脱敏 |
| T3.7.8 | `TestCLI_IOError_ExitCode3` | I/O 错误时退出码 3 |

---

### 迭代四：TUI 交互交付

**迭代目标**：交付覆盖高频操作的 Bubble Tea TUI，复用 CLI 服务层。

#### Phase 4-1：TUI 框架与启动

**测试先行清单**（`internal/tui/app_test.go`）：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T4.1.1 | `TestTUI_Init_AcquiresExclusiveLock` | TUI 启动前必须获取独占锁 |
| T4.1.2 | `TestTUI_Init_LockFail_NoBlankScreen` | 获取锁失败时不进入空白界面 |
| T4.1.3 | `TestTUI_DefaultPage_Overview` | 启动后默认展示 Overview 页面 |
| T4.1.4 | `TestTUI_Quit_ReleasesLock` | 按 `q` 退出后释放锁 |

#### Phase 4-2：导航与状态栏

**测试先行清单**：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T4.2.1 | `TestNav_HighlightOverview` | 初始状态导航栏高亮 Overview |
| T4.2.2 | `TestNav_SwitchToModels` | 按 `j` 后高亮 Models |
| T4.2.3 | `TestNav_SwitchToMCPs` | 继续按 `j` 高亮 MCP Servers |
| T4.2.4 | `TestNav_WrapAround` | 导航到底部后循环到顶部 |
| T4.2.5 | `TestStatusBar_ShowsLockStatus` | 状态栏展示锁状态 |
| T4.2.6 | `TestStatusBar_ShowsSyncResult` | 状态栏展示最近同步结果 |

#### Phase 4-3：Overview 页面

**测试先行清单**：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T4.3.1 | `TestOverview_ShowsCurrentModel` | 展示当前绑定模型名称 |
| T4.3.2 | `TestOverview_ShowsMCPCount` | 展示已启用 MCP 数量 |
| T4.3.3 | `TestOverview_ShowsSyncResult` | 展示最近同步结果 |
| T4.3.4 | `TestOverview_NoModel` | 未绑定模型时展示提示 |

#### Phase 4-4：Models 页面

**测试先行清单**：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T4.4.1 | `TestModelsPage_ListDisplays` | 展示模型名称、来源、是否绑定 |
| T4.4.2 | `TestModelsPage_AddModel` | 按 `a` 进入新增表单 |
| T4.4.3 | `TestModelsPage_EditModel` | 按 `e` 进入编辑表单 |
| T4.4.4 | `TestModelsPage_DeleteModel` | 按 `d` 触发删除确认 |
| T4.4.5 | `TestModelsPage_UseModel` | 按 `u` 执行模型切换 |
| T4.4.6 | `TestModelsPage_UseModel_SyncPrompt` | 切换后提示"仅更新绑定"或"立即同步" |
| T4.4.7 | `TestModelsPage_TokenMasked` | 详情页 token 默认掩码显示 |
| T4.4.8 | `TestModelsPage_EscGoesBack` | 按 `esc` 返回列表页 |

#### Phase 4-5：MCP Servers 页面

**测试先行清单**：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T4.5.1 | `TestMCPPage_ListDisplays` | 展示 MCP 名称、command、是否启用 |
| T4.5.2 | `TestMCPPage_ToggleEnable` | 按 `space` 切换启用/禁用 |
| T4.5.3 | `TestMCPPage_AddMCP` | 按 `a` 进入新增表单 |
| T4.5.4 | `TestMCPPage_EditMCP` | 按 `e` 进入编辑表单 |
| T4.5.5 | `TestMCPPage_DeleteMCP` | 按 `d` 触发删除确认 |

#### Phase 4-6：Sync Preview 页面

**测试先行清单**：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T4.6.1 | `TestSyncPreview_ShowsChanges` | 展示受管字段变更摘要 |
| T4.6.2 | `TestSyncPreview_Confirm` | 确认后执行同步并展示结果 |
| T4.6.3 | `TestSyncPreview_Cancel` | 取消后不执行同步 |
| T4.6.4 | `TestSyncPreview_InSync` | 无差异时提示已同步 |

#### Phase 4-7：Backups 页面

**测试先行清单**：

| # | 测试用例 | 验证点 |
|---|---------|--------|
| T4.7.1 | `TestBackupsPage_ListDisplays` | 展示备份列表、时间、原因 |
| T4.7.2 | `TestBackupsPage_RestoreConfirm` | 选择备份后二次确认恢复 |
| T4.7.3 | `TestBackupsPage_RestoreCancel` | 取消后不执行恢复 |
| T4.7.4 | `TestBackupsPage_CorruptedLabel` | 损坏备份展示损坏标记 |

---

## 3. 测试分类与策略

### 3.1 测试金字塔

```text
                ╱╲
               ╱  ╲         TUI 交互测试（tea.Model 消息驱动）
              ╱    ╲        - 页面渲染、导航、表单交互
             ╱──────╲
            ╱        ╲      CLI 集成测试
           ╱          ╲     - 完整命令链路 → 退出码 + 输出验证
          ╱────────────╲
         ╱              ╲   服务层 / 适配器单元测试
        ╱                ╲  - 业务逻辑、校验、数据转换
       ╱──────────────────╲
      ╱                    ╲ 数据模型 / 存储 / ID 单元测试
     ╱                      ╲ - 序列化、文件读写、ULID
    ╱────────────────────────╲
```

### 3.2 测试隔离策略

| 维度 | 策略 |
|------|------|
| **文件系统** | 所有测试使用 `t.TempDir()` 创建临时目录，注入路径覆盖 `~/.mcfg/` 和 `~/.claude/` |
| **存储层** | 通过接口抽象 `ConfigStore`，单元测试中使用内存实现 |
| **锁机制** | 锁文件路径可注入，测试中指向临时目录 |
| **时间** | 注入 `Clock` 接口，测试中使用固定时间 |
| **ID 生成** | 注入 `IDGenerator` 接口，测试中使用可预测 ID |
| **HOME 路径** | 通过参数/环境变量注入，避免测试依赖真实 HOME |

### 3.3 测试命名规范

```text
Test<模块>_<场景>_<预期行为>
```

示例：
- `TestModelAdd_MissingName_ReturnsParamError`
- `TestSync_TargetMissing_FailsWithInitTargetHint`
- `TestBackupPrune_Keep1_RemovesOldest`

---

## 4. 关键接口抽象（TDD 支撑）

为支持测试隔离，以下关键接口在开发早期定义：

| 接口 | 职责 | 测试用途 |
|------|------|----------|
| `ConfigStore` | 配置文件读写 | 内存实现替换真实文件 I/O |
| `IDGenerator` | ULID 生成 | 固定 ID 简化断言 |
| `Clock` | 当前时间获取 | 固定时间简化时间相关断言 |
| `FileSystem` | 文件系统操作抽象 | 模拟文件缺失、权限错误、写入失败 |
| `LockManager` | 运行锁获取释放 | 模拟锁冲突场景 |
| `TargetAdapter` | 目标配置格式适配 | 验证适配器输出而不写入真实文件 |

---

## 5. 迭代交付检查清单

### 迭代一验收

- [ ] 所有 Phase 1-1 ~ 1-7 测试通过
- [ ] `go test ./...` 全部通过
- [ ] CLI 可完成 model/mcp 的增删改查
- [ ] model use 可切换当前模型
- [ ] 配置可稳定持久化到 config.json
- [ ] 退出码分级正确

### 迭代二验收

- [ ] 所有 Phase 2-1 ~ 2-6 测试通过
- [ ] `mcfg init` 可创建配置中心并完成首次导入
- [ ] `mcfg import` 可发现并导入 Claude Code 配置
- [ ] `mcfg validate` 可输出三层校验结果
- [ ] `mcfg sync` 可正确同步到 Claude Code 配置文件
- [ ] 同步不破坏未受管字段
- [ ] `--json` 输出格式正确

### 迭代三验收

- [ ] 所有 Phase 3-1 ~ 3-7 测试通过
- [ ] 运行锁机制工作正常
- [ ] 备份可创建、查询、恢复、清理
- [ ] 同步失败时自动回滚
- [ ] 外部修改检测可中止操作
- [ ] 锁冲突退出码为 2

### 迭代四验收

- [ ] 所有 Phase 4-1 ~ 4-7 测试通过
- [ ] `mcfg` 默认进入 TUI
- [ ] TUI 可完成模型切换、MCP 启停
- [ ] Sync Preview 可预览并确认同步
- [ ] Backups 页面可恢复备份
- [ ] TUI 复用 CLI 服务层逻辑
- [ ] 敏感 token 掩码显示

---

## 6. 依赖库

| 库 | 用途 | 引入时机 |
|----|------|----------|
| `github.com/spf13/cobra` | CLI 框架 | 迭代一 Phase 1-6 |
| `github.com/charmbracelet/bubbletea` | TUI 框架 | 迭代四 |
| `github.com/charmbracelet/lipgloss` | TUI 样式 | 迭代四 |
| `github.com/charmbracelet/bubbles` | TUI 组件 | 迭代四 |
| `github.com/oklog/ulid/v2` | ULID 生成 | 迭代一 Phase 1-2 |
| `github.com/stretchr/testify` | 测试断言 | 迭代一起 |
| `golang.org/x/sys` | flock 系统调用 | 迭代三 Phase 3-1 |

---

## 7. 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| TUI 短 ID 展示决策未定 | TUI 列表页呈现 | 服务层接口支持返回完整 ID 与短 ID，TUI 侧做配置化展示 |
| import 同名不同内容交互策略未定 | import 命令行为 | V1 先按"不自动合并"实现，预留 `--interactive` 扩展点 |
| JSON 输出字段稳定性 | 脚本化用户 | 迭代二结束前冻结 JSON schema |
| 外部修改竞争 | 数据丢失 | 基于 SHA256 摘要比对 + 原子替换，迭代三重点测试 |
| Bubble Tea 测试覆盖有限 | TUI 质量 | 核心交互用 `tea.Model` 消息驱动测试覆盖，辅以手动冒烟 |
