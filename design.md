# Claude Code 配置中心 CLI 工具设计文档

## 1. 文档信息

- 项目名称：Claude Code 配置中心
- 文档类型：需求分析 + 系统设计文档
- 技术栈：Go + Bubble Tea TUI + Cobra CLI
- 目标平台：macOS / Linux（Windows 后续支持）
- 文档版本：v1.0
- 更新时间：2026-03-10

---

## 2. 项目背景

为了降低在 **Claude Code** 中维护模型配置和 MCP Server 配置的成本，计划开发一个本地命令行程序，使用 **Go** 实现，并通过 **TUI** 提供简洁高效的交互界面。

该工具用于统一管理：

- 多个 Model 配置
- 多个 MCP Server 配置
- Claude Code 的目标绑定关系

并支持：

- 在多个模型之间快速切换
- 给 Claude Code 配置多个 MCP Server
- 一键将配置同步到 Claude Code
- 首次启动时扫描并导入 Claude Code 用户级现有配置

---

## 3. 项目目标

### 3.1 核心目标

构建一个本地 AI 配置中心工具，在第一阶段实现以下能力：

1. 统一维护多个模型配置
2. 统一维护多个 MCP Server 配置
3. 为 Claude Code 绑定当前模型
4. 为 Claude Code 绑定多个 MCP Server
5. 支持模型快速切换
6. 支持一键同步到 Claude Code 配置文件
7. 首次启动时自动扫描 Claude Code 用户级现有配置并导入
8. 支持基于 Bubble Tea 的 TUI 可视化管理
9. 支持基于 Cobra 的 CLI 脚本化操作
10. 在内部架构上预留未来扩展到 Codex 或其他目标工具的能力

### 3.2 产品定位

该工具本质上是一个：

> 面向 Claude Code 的本地统一配置中心 + 快速切换器 + 配置分发器

它负责：

- 配置集中管理
- 现有 Claude Code 配置发现与导入
- 目标工具绑定管理
- 配置格式适配
- 配置写入、备份与恢复

它不负责：

- 发起模型推理请求
- 实现 MCP Server 本身
- 替代 Claude Code 或 Codex 的产品能力

---

## 4. 用户画像

目标用户主要是：

- 使用 Claude Code 的开发者
- 需要频繁切换模型配置的用户
- 需要统一管理多个 MCP Server 的开发者
- 偏好终端工具和快捷操作的技术用户

---

## 5. 核心使用场景

### 5.1 多模型快速切换

用户维护了多个模型，例如：

- Claude Sonnet
- GPT-4.1
- GPT-4o
- 内部代理模型

用户希望快速完成：

- 查看当前活动模型
- 选择另一个模型
- 指定切换到 Claude Code
- 保存并同步

首次使用时，用户还希望工具能够自动发现本机已有 Claude Code 配置，避免手工重新录入。

第一阶段的扫描范围限定为 Claude Code 用户级配置，不处理项目级配置。

### 5.2 多 MCP Server 配置

用户希望为 Claude Code 挂载多个 MCP Server，例如：

- filesystem
- git
- postgres
- browser
- internal-docs

并支持：

- 多选启用
- 批量禁用
- 按 Claude Code 当前绑定关系启用/停用
- 一键同步

### 5.3 目标工具配置不同

第一阶段仅支持 Claude Code，但后续版本可能扩展为多目标工具模式，例如：

- Claude Code 使用模型 A + MCP 1/2/3
- Codex 使用模型 B + MCP 2/4/5

因此第一阶段虽然只落地 Claude Code，同步架构仍建议保持“统一资源池 + 目标适配器”的扩展思路，避免后续接入其他工具时推翻现有实现。

---

## 6. 需求范围

### 6.1 功能范围

本期功能包括：

- Model 配置管理
- MCP Server 配置管理
- Claude Code 目标绑定管理
- 首次启动扫描并导入 Claude Code 用户级现有配置
- 模型快速切换
- 多 MCP Server 挂载
- 一键同步到 Claude Code
- 基于 Bubble Tea 的 TUI 界面
- 基于 Cobra 的 CLI 命令
- 配置备份

### 6.2 非功能范围

本期不包括：

- Codex 支持
- 其他第三方 AI 工具支持
- 云端同步
- 多用户协作
- 远程下发配置
- 在线调用模型测试
- MCP Server 生命周期托管
- Web 管理后台

### 6.3 关键非功能要求

- 安全性：本地配置文件默认仅当前用户可读写，敏感 token 采用明文保存在本地配置中心，CLI/TUI 输出中对敏感 token 做脱敏显示
- 可靠性：每次写入 Claude Code 配置前必须创建备份，写入失败时自动回滚
- 一致性：TUI 与 CLI 必须共享同一套配置服务、校验逻辑和持久化模型
- 运行锁约束：V1 采用“读共享、写独占”锁模型；写命令与 TUI 必须获取独占锁，只读命令获取共享锁，未获取到锁时立即失败并提示当前占用者信息
- 兼容性：V1 仅支持 macOS / Linux 上 Claude Code 官方用户级 `~/.claude/settings.json` 与 `~/.claude.json` 配置格式
- 可观测性：CLI 命令返回明确退出码，TUI/CLI 对扫描、校验、同步失败给出可操作错误信息

### 6.4 本地持久化约束

- 本地配置中心目录固定为 `~/.mcfg/`
- 主配置文件固定为 `~/.mcfg/config.json`
- 备份文件固定存放于 `~/.mcfg/backups/`
- 运行锁文件固定为 `~/.mcfg/run.lock`
- 本地持久化采用明文 JSON 存储，不接系统钥匙串

---

## 7. 核心设计结论

从需求出发，系统必须采用下面的设计模式：

### 7.1 配置资源池

统一保存：

- 所有 models
- 所有 mcp_servers

### 7.2 统一数据模型

第一阶段内部数据模型以 Claude Code 官方 `settings.json` 中的模型环境变量片段为参考，典型结构如下：

```json
{
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "...",
    "ANTHROPIC_BASE_URL": "https://api.minimaxi.com/anthropic",
    "ANTHROPIC_MODEL": "MiniMax-M2.5"
  }
}
```

V1 采用以下统一模型：

#### ConfigRoot

`~/.mcfg/config.json` 在 V1 中固定采用以下根结构：

```json
{
  "schema_version": 1,
  "models": [],
  "mcp_servers": [],
  "claude_binding": {
    "current_model_id": "",
    "enabled_mcp_ids": [],
    "last_sync_at": "",
    "last_sync_result": ""
  },
  "backup_index": []
}
```

字段说明：

- `schema_version`：配置文件结构版本号，V1 固定为 `1`
- `models`：ModelProfile 列表
- `mcp_servers`：MCPServer 列表
- `claude_binding`：Claude Code 当前绑定关系
- `backup_index`：备份元数据索引列表，作为程序查询备份记录的唯一元数据来源

约束说明：

- V1 不再使用顶层 `version` 字段，统一使用 `schema_version`
- `backup_index` 记录元数据，`~/.mcfg/backups/` 保存实际备份文件内容，二者共同组成完整备份体系
- `backup list` 以 `backup_index` 为准；如果索引存在但备份文件缺失，命令应明确提示该备份记录损坏
- 后续版本如引入迁移机制，以 `schema_version` 作为迁移入口

#### ModelProfile

- `id`：模型配置唯一 ID
- `name`：模型配置显示名称
- `env`：模型环境变量映射，至少支持 `ANTHROPIC_AUTH_TOKEN`、`ANTHROPIC_BASE_URL`、`ANTHROPIC_MODEL`
- `source`：来源，取值为 `manual` 或 `imported`
- `description`：备注说明，可选
- `created_at`：创建时间
- `updated_at`：更新时间

说明：

- V1 的模型配置以 `env` 作为核心载体，便于和 Claude Code 官方配置格式直接映射
- 后续若扩展到 Codex 或其他工具，可在统一模型上增加 target-specific 字段或适配层映射规则
- `env` 中未识别的扩展字段允许保留，以避免丢失用户自定义配置

#### MCPServer

- `id`：MCP 配置唯一 ID
- `name`：MCP Server 显示名称
- `transport`：传输类型，V1 固定为 `stdio`
- `command`：启动命令
- `args`：命令参数列表
- `env`：MCP Server 环境变量映射
- `description`：备注说明，可选
- `source`：来源，取值为 `manual` 或 `imported`
- `created_at`：创建时间
- `updated_at`：更新时间

说明：

- V1 的扫描与同步范围仅覆盖 Claude Code 用户级 MCP 配置
- V1 仅支持 `stdio`
- `http` / `sse` 放入后续版本，不在本期 CLI、校验和同步范围内
- `~/.claude.json` 中的用户级 MCP 配置按“当前用户 home 绝对路径”组织，V1 读取 `<当前用户 home 绝对路径>` 节点下的 `mcpServers`
- 该节点的典型结构如下：

```json
{
  "<当前用户 home 绝对路径>": {
    "allowedTools": [],
    "mcpContextUris": [],
    "mcpServers": {
      "github": {
        "type": "stdio",
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-github"],
        "env": {
          "GITHUB_PERSONAL_ACCESS_TOKEN": "..."
        }
      }
    }
  }
}
```

- V1 仅处理 `<当前用户 home 绝对路径>` 节点下的 `mcpServers`
- 单个 `<当前用户 home 绝对路径>.mcpServers.<name>` 在 V1 中至少包含：`type`、`command`
- `args`、`env` 允许为空；为空时可以在源文件中缺省
- 其中 `type` 在 V1 中固定为 `stdio`
- `type` 和 `command` 不能为空；为空时视为非法配置
- 程序内部 `MCPServer.name` 与 `mcpServers` 的对象 key 一一对应
- `source` 仅用于展示和审计，例如列表页展示“手动创建”或“导入自 Claude Code”；V1 中不影响编辑、删除、去重、同步和校验逻辑

命名约定：

- 产品文案、需求描述、TUI 文案中统一使用 `MCP Server`
- CLI 命令、子命令、短语义标识统一使用 `mcp`
- 配置文件字段和数据结构列表统一使用 `mcp_servers`

#### ClaudeBinding

- `current_model_id`：Claude Code 当前绑定的模型 ID
- `enabled_mcp_ids`：Claude Code 当前启用的 MCP ID 列表
- `last_sync_at`：最近一次同步时间
- `last_sync_result`：最近一次同步结果摘要

#### BackupMeta

- `id`：备份唯一 ID
- `target`：备份目标，V1 固定为 `claude_code`
- `files`：备份文件列表，固定同时包含 `~/.claude/settings.json` 与 `~/.claude.json` 的备份信息
- `reason`：备份原因，如 `sync`、`manual`
- `created_at`：备份创建时间
- `source_hash`：备份时源文件摘要，可选

说明：

- 单个 `backup-id` 对应一次完整快照
- 一次快照同时备份 `~/.claude/settings.json` 与 `~/.claude.json`
- `files` 中每一项至少包含 `target_path`、`backup_path`、`exists_before_backup`
- 备份前如果任一目标文件不存在，则提示缺失文件路径，并中止本次备份操作
- 恢复前如果任一目标文件不存在，则提示缺失文件路径，并中止本次恢复操作

#### ID 生成与引用规则

V1 中 `ModelProfile.id`、`MCPServer.id`、`BackupMeta.id` 统一采用不可变的 opaque ID，规则如下：

- ID 生成算法统一采用 `ULID`
- ID 一旦生成，后续编辑名称、描述、env、args 等字段时均不得变更
- `name` 仅作为展示字段，不承担主键语义
- CLI 中所有 `<id>` 参数均支持传入完整 ID，或传入最少 8 位的唯一前缀
- 若前缀匹配到多个对象，命令必须报错并提示存在歧义，不得自动猜测
- TUI 默认展示 `name`，并在详情视图或复制操作中展示完整 ID

这样可以同时满足：

- 持久化主键稳定，不受名称变更影响
- CLI 输入成本可控
- 后续支持导入、去重、迁移时不需要重写主键体系

#### 备份保留与清理策略

V1 必须定义备份保留策略，避免 `~/.mcfg/backups/` 无限增长。

默认规则如下：

- 自动保留最近 `3` 个完整快照
- 每次 `sync` 或 `backup create` 成功创建新备份后，立即执行一次自动清理
- 自动清理按 `created_at` 从旧到新排序，删除超出保留上限的旧快照
- 自动清理时必须同时删除 `backup_index` 中对应元数据和 `~/.mcfg/backups/` 中对应备份文件
- 如果某条备份记录对应的文件已缺失，清理过程应将其视为损坏记录并一并移除

V1 同时提供手动清理命令：

- `mcfg backup prune`
- 默认行为：按当前系统默认保留数量执行清理
- 可选参数：`--keep <n>`，允许用户临时指定本次保留数量，`n` 必须大于等于 `1`
- `backup prune` 不创建新备份，不修改目标文件，只清理历史快照与索引

这样可以避免：

- 每次同步都生成新备份，长期运行后目录无限膨胀
- 备份索引和实际备份文件越积越多且逐渐漂移
- 用户只能手工删目录，导致索引与文件状态不一致

### 7.3 目标绑定关系

第一阶段保存：

- Claude Code 当前绑定哪个 model
- Claude Code 当前启用了哪些 mcp

后续版本可扩展为：

- 不同目标工具各自的 model 绑定
- 不同目标工具各自的 mcp 绑定

### 7.4 适配器分发

采用统一内部模型，然后通过适配器生成 Claude Code 所需配置格式；后续如接入 Codex 或其他工具，仅新增对应适配器：

```text
统一配置中心 -> Target Binding -> Adapter -> 目标配置文件
```

### 7.5 首次启动扫描

系统首次启动时应自动扫描 Claude Code 官方用户级配置路径，并执行配置发现流程：

- 用户级 settings 路径：`~/.claude/settings.json`
- 用户级 MCP 路径：`~/.claude.json`
- V1 仅读取 `~/.claude.json` 中 `<当前用户 home 绝对路径>` 节点下的 `mcpServers`
- 官方参考文档：
  - Settings: `https://docs.anthropic.com/en/docs/claude-code/settings`
  - MCP: `https://docs.anthropic.com/en/docs/claude-code/mcp`

- 如果 `~/.claude/settings.json` 或 `~/.claude.json` 存在，则读取当前配置并尝试导入
- 如果 `~/.claude/settings.json` 不存在，则提示用户尚未发现 Claude Code 模型配置文件，并引导用户手动创建模型配置
- 如果 `~/.claude.json` 不存在，则提示用户当前未发现用户级 MCP 配置，并继续进入空 MCP 配置模式
- 如果 `~/.claude/settings.json` 格式损坏或无法解析，则提示该文件损坏；`~/.claude.json` 正常时继续导入用户级 MCP 配置
- 如果 `~/.claude.json` 格式损坏或无法解析，则提示该文件损坏；`~/.claude/settings.json` 正常时继续导入模型配置
- 如果两个文件都损坏，则分别提示两个文件的损坏信息，本次导入不写入任何扫描结果
- 如果扫描到重复模型或重复 MCP，则按标准化后的内容去重，只导入一份，并在结果中提示跳过数量
- 如果扫描到项目级或 local-scope 配置，则 V1 默认忽略，不导入本地配置中心

导入流程：

- 读取 `~/.claude/settings.json` 顶层 `env` 对象中的模型相关配置
- 读取 `~/.claude.json` 中 `<当前用户 home 绝对路径>` 节点下的 `mcpServers` 字段中的用户级 MCP 配置
- 转换为内部统一数据模型
- 导入本地配置中心存储
- 向用户展示导入结果与冲突提示

如果扫描失败，不应阻塞工具启动，但应提供明确错误提示，并允许用户稍后手动执行导入。

重复判定建议：

- Model 以 `env.ANTHROPIC_MODEL + env.ANTHROPIC_BASE_URL` 的标准化内容作为主判定依据
- MCP 以 `transport + command + args + env` 的标准化内容作为主判定依据
- 如果本地配置中心中已存在重复 Model，且 `ANTHROPIC_MODEL` 与 `ANTHROPIC_BASE_URL` 均一致，则跳过导入，扫描结果记录为 `skipped`
- 如果本地配置中心中已存在重复 MCP，默认保留当前程序内已有配置，扫描结果中的重复项记录为 `skipped`
- 名称相同但标准化内容不同，视为不同配置，不自动合并

### 7.5.1 `init` 幂等与重入语义

`mcfg init` 在 V1 中必须是幂等的，规则如下：

- 首次执行时，创建 `~/.mcfg/`、`~/.mcfg/backups/`、`~/.mcfg/config.json`，随后执行一次扫描导入
- 如果 `~/.mcfg/config.json` 已存在且可正常解析，则 `mcfg init` 返回退出码 `0`，并输出 `already initialized`
- 对已初始化环境重复执行 `mcfg init` 时，不覆盖本地配置，不重新扫描，不合并外部配置
- 用户如需重新扫描 Claude Code 现有配置，使用 `mcfg import`，而不是重复执行 `mcfg init`
- 如果 `~/.mcfg/` 存在但 `config.json` 缺失，则仅补齐缺失的基础文件和目录，不删除已有备份目录内容
- 如果 `~/.mcfg/config.json` 存在但损坏或无法解析，则 `mcfg init` 返回非 `0`，拒绝覆盖现有文件，并提示用户先修复或手动备份后移走损坏文件

这样可以避免：

- 用户重复执行 `init` 时意外覆盖已有配置
- 初始化行为和“重新导入”行为混在一起
- 已有备份目录被初始化流程误删

### 7.5.2 `init` 与 `import` 的职责边界

`mcfg init` 与 `mcfg import` 都涉及扫描 Claude Code 配置，但职责不同，V1 固定按下表执行：

| 维度 | `mcfg init` | `mcfg import` |
|---|---|---|
| 创建 `~/.mcfg/` 目录 | 是 | 否 |
| 创建 `~/.mcfg/config.json` | 是 | 否 |
| 扫描 Claude Code 配置 | 是 | 是 |
| 写入导入结果 | 是 | 是 |
| 重复执行行为 | 已初始化则直接返回 `already initialized` | 允许重复执行，按去重规则跳过重复项 |
| 前置条件 | 无 | `~/.mcfg/config.json` 已存在且可解析 |
| 典型用途 | 首次建库 + 首次导入 | 重新发现外部变更并导入到本地配置中心 |

补充约束：

- `import` 不负责修复、补建或重置 `~/.mcfg/` 目录结构
- `import` 在本地配置中心不存在时必须返回非 `0`，并明确提示用户先执行 `mcfg init`
- `init` 成功后无需自动再次提示执行 `import`
- 两者都遵守同一套去重、解析失败处理和冲突提示规则

这样可以避免：

- 用户把 `init` 当成“重新导入”命令反复执行
- `import` 隐式承担初始化职责，导致边界模糊
- 实现时把两个命令做成几乎同一条路径却无法解释差异

### 7.6 同步与回滚语义

V1 中，本地配置中心是 Claude Code 配置管理的唯一事实来源；`init/import` 完成后，用户的后续编辑以本地配置中心为准。

`mcfg sync` 的行为定义如下：

- 同步前必须先创建时间戳备份
- 每次备份必须同时覆盖 `~/.claude/settings.json` 与 `~/.claude.json`
- 如果 `~/.claude/settings.json` 或 `~/.claude.json` 任一不存在，则默认提示缺失文件路径，并中止本次 `sync`
- `mcfg sync --init-target` 可在目标文件缺失时自动创建最小骨架文件后继续同步
- `--init-target` 仅在目标文件缺失时生效；若目标文件已存在但内容损坏，不得覆盖，仍应报错退出
- 默认仅更新工具受管字段，尽量保留 `~/.claude/settings.json` 与 `~/.claude.json` 中未受管的其他配置内容
- 写入采用“生成目标内容 -> 校验 -> 临时文件写入 -> 原子替换”流程
- 如果任一阶段失败，则自动回滚到同步前备份，并报告失败原因
- 同步成功后更新 `last_sync_at` 和 `last_sync_result`
- `mcfg sync --dry-run` 仅展示计划变更，不执行写入；若目标文件缺失，仍按 `sync` 同样报错
- `mcfg backup create` 执行前要求 `~/.claude/settings.json` 与 `~/.claude.json` 同时存在；任一缺失时提示缺失文件路径并中止本次备份
- `mcfg backup restore` 按同一个 `backup-id` 同时恢复 `~/.claude/settings.json` 与 `~/.claude.json`
- 如果恢复前任一目标文件不存在，则提示缺失文件路径，并中止本次恢复
- `mcfg backup restore` 恢复目标文件后，不自动覆盖本地配置中心；如需对齐，以用户后续执行 `mcfg import` 为准

受管字段定义在 V1 中明确为：

- `~/.claude/settings.json` 顶层 `env.ANTHROPIC_AUTH_TOKEN`
- `~/.claude/settings.json` 顶层 `env.ANTHROPIC_MODEL`
- `~/.claude/settings.json` 顶层 `env.ANTHROPIC_BASE_URL`
- `~/.claude.json` 中 `<当前用户 home 绝对路径>` 节点下的 `mcpServers`

字段依据说明：

- Claude Code Settings 官方文档确认用户级 settings 文件路径为 `~/.claude/settings.json`，并说明环境变量可通过顶层 `env` 对象配置
- Claude Code Settings 官方文档列出了 `ANTHROPIC_AUTH_TOKEN`、`ANTHROPIC_MODEL` 等环境变量
- Claude Code MCP 在 `~/.claude.json` 中按路径节点组织；V1 仅处理 `<当前用户 home 绝对路径>` 节点下的 `mcpServers`，不处理其他路径节点
- 如果目标文件中当前不存在 `env`，程序在 `sync` 时主动创建并写入该字段

`mcfg sync --init-target` 创建的最小骨架内容固定为：

```json
// ~/.claude/settings.json
{
  "env": {}
}
```

```json
// ~/.claude.json
{
  "<当前用户 home 绝对路径>": {
    "mcpServers": {}
  }
}
```

创建约束：

- 自动创建时应同时补齐父目录
- 新建文件权限默认设为仅当前用户可读写
- 若命令因目标缺失而失败，错误输出必须同时给出缺失路径和可使用的 `mcfg sync --init-target` 指引

### 7.6.1 外部修改与竞争保护

运行锁只能保护 `mcfg` 进程之间的互斥，不能阻止 Claude Code 或用户直接修改 `~/.claude/settings.json` 与 `~/.claude.json`。

V1 必须明确以下保护策略：

- 这是已知限制，文档和错误提示中都应明确告知用户
- 执行 `sync`、`sync --dry-run`、`backup restore` 前，应提示用户关闭 Claude Code 的配置编辑界面或其他直接编辑目标文件的进程
- `sync` 与 `backup restore` 在开始处理目标文件后，必须先读取当前文件内容并计算摘要
- 在临时文件原子替换前，必须再次确认目标文件摘要未发生变化
- 如果前后两次摘要不一致，视为检测到外部并发修改，本次操作立即失败，不覆盖目标文件，也不继续写入
- 发生外部并发修改时，错误信息必须包含受影响文件路径，并提示用户重新执行 `mcfg validate` 或稍后重试

这样可以避免：

- `mcfg` 刚生成结果时被外部修改覆盖
- 用户误以为运行锁能保护 Claude Code 自身写入
- `sync` 和 `backup restore` 在竞争条件下静默丢失外部变更

### 7.6.2 `validate` 校验语义

`mcfg validate` 在 V1 中不是“仅检查 JSON 能否解析”，而是固定执行以下三层校验：

#### A. 本地配置中心一致性校验

- `schema_version` 必须存在，且当前仅允许为 `1`
- `models`、`mcp_servers`、`backup_index` 必须为数组，`claude_binding` 必须为对象
- `models[].id`、`mcp_servers[].id`、`backup_index[].id` 在各自集合内必须唯一
- 如果 `claude_binding.current_model_id` 非空，则必须引用存在的 `models[].id`
- `claude_binding.enabled_mcp_ids` 中的每个 ID 都必须引用存在的 `mcp_servers[].id`
- `claude_binding.enabled_mcp_ids` 中不得出现重复 ID
- `backup_index[].files` 必须同时包含 `~/.claude/settings.json` 与 `~/.claude.json` 两个目标文件的备份记录

#### B. 字段合法性校验

- `ModelProfile.name` 不能为空
- `ModelProfile.env.ANTHROPIC_AUTH_TOKEN` 不能为空
- `ModelProfile.env.ANTHROPIC_MODEL` 不能为空
- `ModelProfile.env.ANTHROPIC_BASE_URL` 必须为合法的绝对 URL，且 scheme 仅允许 `http` 或 `https`
- `MCPServer.name` 不能为空
- `MCPServer.transport` 在 V1 中必须为 `stdio`
- `MCPServer.command` 不能为空
- `MCPServer.args` 如存在，必须为字符串数组，且每一项不能为空字符串
- `MCPServer.env` 如存在，必须为字符串键值映射；key 必须匹配 `[A-Za-z_][A-Za-z0-9_]*`
- `source` 仅允许为 `manual` 或 `imported`
- 时间字段如存在，必须可解析为 RFC3339 时间

#### C. 目标文件可同步性校验

- `~/.claude/settings.json` 与 `~/.claude.json` 必须同时存在；任一缺失时返回校验失败
- 两个目标文件都必须能被成功解析；解析失败时返回校验失败
- 基于当前本地配置生成“待同步目标内容”，并再次校验受管字段是否满足 Claude Code 所需结构
- 将“当前目标文件中的受管字段”与“待同步目标内容中的受管字段”做对比
- 对比结果必须明确标识为：
  - `in_sync`：当前目标文件与本地配置一致，无需同步
  - `out_of_sync`：目标文件可解析，但受管字段与本地配置不一致，需要同步

说明：

- `validate` 的 diff 范围仅限 V1 受管字段，不比较未受管字段
- `validate` 不写入任何文件，不创建备份，不更新 `last_sync_at`
- `sync --dry-run` 面向“预览将写入什么”，`validate` 面向“判断当前配置是否有效、是否可同步、是否已漂移”

#### 输出与退出码

- 无问题时返回退出码 `0`
- 只要存在任一 error，返回非 `0`
- warning 不单独导致失败，但必须出现在输出中
- 人类可读输出至少包含以下区块：
  - `Summary`：`valid` / `invalid`、error 数量、warning 数量、同步状态
  - `Errors`
  - `Warnings`
  - `Target Drift`：列出受管字段是否存在差异
- `--json` 输出固定包含：

```json
{
  "valid": true,
  "sync_status": "in_sync",
  "errors": [],
  "warnings": [],
  "checks": {
    "config_consistency": "passed",
    "field_validation": "passed",
    "target_syncability": "passed"
  },
  "drift": {
    "managed_paths_changed": []
  }
}
```

输出约束：

- `errors[]` / `warnings[]` 中每一项至少包含 `code`、`message`、`path`
- `drift.managed_paths_changed` 仅列出受管字段路径，例如 `env.ANTHROPIC_MODEL`、`<home>.mcpServers`
- 当目标文件缺失或损坏时，`sync_status` 固定为 `unavailable`

### 7.7 交互与命令框架

- TUI 采用 `bubbletea` 构建，用于高频可视化管理操作
- CLI 采用 `cobra` 构建，用于脚本化、自动化和精确命令调用
- TUI 与 CLI 共享同一套配置服务和数据模型，避免逻辑重复
- 所有入口在真正执行业务逻辑前都必须先尝试获取全局运行锁；锁的生命周期覆盖整个进程存活期
- 直接执行 `mcfg` 时默认进入 TUI 界面
- `mcfg tui` 作为显式子命令保留，行为与直接执行 `mcfg` 等价

### 7.7.1 TUI 最小设计定义

由于 `mcfg` 默认直接进入 TUI，V1 必须明确最小可交付的界面结构，而不能只写“基于 Bubble Tea 实现”。

主界面布局固定为三段式：

- 左侧导航区：`Overview`、`Models`、`MCP Servers`、`Sync Preview`、`Backups`
- 右侧主面板：当前页面的列表、详情或表单
- 底部状态栏：当前锁状态、当前目标、最近同步结果、快捷键提示

V1 必须提供以下核心页面：

- `Overview`：展示当前模型、已启用 MCP 数量、最近同步结果、目标文件状态
- `Models` 列表页：展示模型名称、来源、更新时间、当前是否已绑定
- `Model` 详情/编辑页：查看或编辑 `name`、`base_url`、`model`、`description`；token 输入必须采用掩码方式
- `MCP Servers` 列表页：展示名称、command、transport、是否已启用
- `MCP` 详情/编辑页：查看或编辑 `command`、`args`、`env`、`description`
- `Sync Preview` 页：展示受管字段的变更摘要，并提供确认同步入口
- `Backups` 页：展示备份列表、时间、原因，并支持恢复确认

V1 必须覆盖以下关键交互流程：

- 模型切换：进入 `Models` -> 选中目标模型 -> 执行 `Use Model` -> 选择“仅更新本地绑定”或“立即同步”
- MCP 启停：进入 `MCP Servers` -> 选中条目 -> 切换启用状态 -> 返回列表
- 同步确认：进入 `Sync Preview` -> 查看受管字段变更摘要 -> 确认执行同步
- 备份恢复：进入 `Backups` -> 选择备份 -> 查看目标文件信息 -> 二次确认恢复

V1 默认快捷键约定：

- `j` / `k` 或方向键：上下移动
- `enter`：进入详情或确认
- `esc`：返回上一级
- `a`：新增
- `e`：编辑
- `d`：删除
- `u`：使用当前模型
- `space`：启用/禁用当前 MCP
- `s`：进入同步预览
- `r`：刷新当前页
- `q`：退出 TUI

约束：

- TUI 启动前必须先成功获取独占锁；获取失败时不允许进入空白界面
- 所有 TUI 写操作必须复用与 CLI 相同的 service / validator / adapter
- 所有敏感 token 在 TUI 中默认掩码显示，除非用户显式进入编辑态

### 7.8 全局运行锁机制

为满足“写操作互斥、读操作可并发”的要求，V1 必须实现统一运行锁机制：

- 锁文件路径固定为 `~/.mcfg/run.lock`
- 所有 `mcfg` 入口统一在进程启动阶段获取同一把运行锁
- TUI 以及所有会修改本地配置中心或目标配置文件的命令必须获取独占锁
- 只读命令获取共享锁，允许多个读命令并发执行
- 获取独占锁成功后，进程在锁文件中写入当前 `pid`、启动时间、命令行摘要，便于冲突提示
- 获取锁失败时，命令立即退出，不做任何读写、副作用操作，也不进入 TUI
- 锁释放时机为进程退出时；正常退出主动释放，异常退出依赖操作系统关闭文件描述符后自动释放
- 若检测到锁文件存在但锁已失效，程序应自动覆盖陈旧元信息并继续启动，不要求用户手动清理
- 独占锁持有期间，所有其他读写命令均不得进入业务执行
- 共享锁持有期间，只读命令可继续进入业务执行，写命令必须立即失败

推荐实现方式：

- macOS / Linux 下基于锁文件句柄执行 `flock` 或等价的进程级 advisory lock
- 锁能力封装为共享的 runtime guard，供 Cobra 根命令和 TUI 启动路径复用
- 业务初始化、配置加载、扫描、导入、同步、备份、TUI 渲染均应发生在成功持锁之后
- 独占锁冲突时应优先展示占用锁的 `pid`、启动时间和命令摘要；共享锁导致写命令失败时，至少提示存在其他只读进程占用共享锁

这样可以避免：

- 两个 CLI 命令同时修改本地配置中心
- TUI 与 CLI 并发运行导致状态覆盖
- 两次 `sync` 或 `backup restore` 并发执行导致目标文件竞争写入

### 7.9 CLI 主要命令

CLI 第一阶段至少支持以下主要命令：

- `mcfg init`：初始化本地配置中心，并在首次启动时扫描 Claude Code 配置
- `mcfg import`：手动重新扫描并导入 Claude Code 现有配置
- `mcfg status`：查看当前模型、启用的 MCP、最近同步状态
- `mcfg model list`：列出模型配置
- `mcfg model add`：新增模型配置
- `mcfg model edit`：编辑模型配置
- `mcfg model remove`：删除模型配置
- `mcfg model remove --force`：解除当前绑定后删除模型配置
- `mcfg model use`：切换 Claude Code 当前模型
- `mcfg model use --sync`：切换 Claude Code 当前模型并立即同步到目标文件
- `mcfg mcp list`：列出 MCP Server 配置
- `mcfg mcp add`：新增 MCP Server 配置
- `mcfg mcp edit`：编辑 MCP Server 配置
- `mcfg mcp remove`：删除 MCP Server 配置
- `mcfg mcp remove --force`：先禁用再删除 MCP Server 配置
- `mcfg mcp enable`：启用指定 MCP Server
- `mcfg mcp disable`：禁用指定 MCP Server
- `mcfg validate`：校验本地配置和目标同步内容是否合法
- `mcfg sync`：将当前配置同步到 Claude Code
- `mcfg sync --init-target`：在目标文件缺失时自动创建最小骨架后再同步
- `mcfg sync --dry-run`：预览同步变更但不落盘
- `mcfg backup create`：手动创建当前 Claude Code 配置备份
- `mcfg backup list`：列出备份
- `mcfg backup prune`：清理超出保留上限的历史备份
- `mcfg backup restore`：恢复指定备份
- `mcfg`：默认进入 TUI 界面
- `mcfg tui`：显式进入 TUI 界面，与 `mcfg` 等价

### 7.10 CLI 命令行为约定

- 所有写操作命令在执行前都必须先做参数与配置校验
- 所有命令在参数校验和业务执行前都必须先获取运行锁；读命令使用共享锁，写命令使用独占锁；获取失败时返回非 `0`
- CLI 退出码在 V1 中固定分级如下：
  - `0`：成功
  - `1`：一般业务错误，如校验失败、引用冲突、目标文件漂移、同步失败
  - `2`：运行锁冲突
  - `3`：I/O 或文件系统错误，如文件不存在、权限不足、读写失败、原子替换失败
  - `4`：参数错误，如缺少必填参数、参数格式非法、互斥参数同时传入
- `status`、`list`、`validate`、`sync --dry-run` 应支持人类可读输出；后续可扩展 `--json`
- `sync`、`backup restore`、`import` 属于高风险命令，必须返回明确结果摘要
- `remove --force` 属于高风险命令，必须明确提示将自动解除引用关系
- 涉及敏感 token 的命令必须支持安全输入方式，且默认输出中不得回显 token 明文
- TUI 中的关键写操作应复用 CLI 同级服务层能力，而不是另写一套逻辑

锁冲突时的输出要求：

- 明确提示当前已有 `mcfg` 进程正在运行
- 如果冲突方为独占锁持有者，优先展示占用锁的 `pid`、启动时间和命令摘要
- 如果冲突由共享锁导致，至少提示当前存在其他只读命令正在运行
- 错误信息需要可直接用于定位冲突，例如提示用户关闭已有 TUI 或等待正在执行的 CLI 命令结束

### 7.11 CLI 参数定义

以下为 V1 必须支持的核心 CLI 参数约定：

- `mcfg`
  - 无必填参数
  - 行为：默认进入 TUI 界面
- `mcfg init`
  - 无必填参数
  - 行为：初始化 `~/.mcfg/`，随后扫描 `~/.claude/settings.json` 与 `~/.claude.json`
- `mcfg import`
  - 无额外参数
  - 默认行为：保留本地已有配置，跳过重复项
  - 前置条件：`~/.mcfg/config.json` 已存在；不存在时返回非 `0` 并提示先执行 `mcfg init`
- `mcfg status`
  - 可选参数：`--json`
- `mcfg model list`
  - 可选参数：`--json`
- `mcfg model add`
  - 必填参数：`--name`、`--base-url`、`--model`
  - token 输入方式三选一：`--auth-token`、`--auth-token-stdin`、`--auth-token-file <path>`
  - 在交互式 TTY 中，如果未提供任何 token 参数，则提示用户以无回显方式输入 token
  - `--auth-token`、`--auth-token-stdin`、`--auth-token-file` 互斥，传入多个时直接报错
  - 可选参数：`--env`、`--desc`
  - `--env` 为可重复参数；每传入一次表示一个额外的 `KEY=VALUE` 环境变量
  - 通过 `--auth-token`、`--base-url`、`--model` 传入的字段最终也写入 `env`；若通过 `--env` 传入同名保留字段，则以显式专用参数为准
- `mcfg model edit <id>`
  - 可选参数：`--name`、`--auth-token`、`--auth-token-stdin`、`--auth-token-file <path>`、`--base-url`、`--model`、`--env`、`--desc`
  - 规则：模型连接信息字段允许单独修改；未提供的连接字段保持原值不变
  - token 相关参数互斥；未提供任何 token 参数时保持原 token 不变
  - 未传 `--env` 时保持原自定义 env 不变；传入任意 `--env` 时按本次给定值整体替换自定义 env
  - 如需显式清空自定义 env，提供 `--clear-env`
  - `--env` 中不允许覆盖 `ANTHROPIC_AUTH_TOKEN`、`ANTHROPIC_BASE_URL`、`ANTHROPIC_MODEL`；这些字段仅允许通过专用参数修改
  - 错误处理：仅对本次实际传入的字段做合法性校验，校验失败时整体报错，本次命令不执行任何更新
- `mcfg model remove <id>`
  - 可选参数：`--force`
  - 默认行为：如果该模型正被 `current_model_id` 引用，则禁止删除并返回明确错误
  - 传入 `--force` 时，先将 `current_model_id` 置空，再删除模型
  - 未传 `--force` 且删除被拒绝时，错误信息必须明确提示用户可执行 `mcfg model use <other-id>` 或 `mcfg model remove <id> --force`
- `mcfg model use <id>`
  - 可选参数：`--sync`
  - 默认行为：仅更新本地 `current_model_id`
  - 传入 `--sync` 时，在同一命令中继续执行同步流程；如果同步失败，则回滚本次 `current_model_id` 变更，命令整体返回失败
- `mcfg mcp list`
  - 可选参数：`--json`
- `mcfg mcp add`
  - 必填参数：`--name`、`--command`
  - 可选参数：`--args`、`--env`、`--desc`
  - `transport` 在 V1 中不暴露为 CLI 参数，程序内部固定写入 `stdio`
  - `--args` 为可重复参数；每传入一次表示追加一个命令参数，并按传入顺序保留
  - `--env` 为可重复参数；每传入一次表示一个 `KEY=VALUE` 项
- `mcfg mcp edit <id>`
  - 可选参数：`--name`、`--command`、`--args`、`--env`、`--desc`
  - `transport` 在 V1 中不提供编辑入口，保持固定为 `stdio`
  - 未传 `--args` 时保持原值不变；传入任意 `--args` 时按本次给定值整体替换
  - 未传 `--env` 时保持原值不变；传入任意 `--env` 时按本次给定值整体替换
  - 如需显式清空，提供 `--clear-args`、`--clear-env`
- `mcfg mcp remove <id>`
  - 可选参数：`--force`
  - 默认行为：如果该 MCP 正被 `enabled_mcp_ids` 引用，则禁止删除并返回明确错误
  - 传入 `--force` 时，先从 `enabled_mcp_ids` 中移除该 MCP，再删除配置
  - 未传 `--force` 且删除被拒绝时，错误信息必须明确提示用户可执行 `mcfg mcp disable <id>` 或 `mcfg mcp remove <id> --force`
- `mcfg mcp enable <id>`
  - 无额外参数
  - 幂等语义：若该 MCP 已处于启用状态，则返回退出码 `0`，不报错，并输出 `already enabled`
- `mcfg mcp disable <id>`
  - 无额外参数
  - 幂等语义：若该 MCP 已处于禁用状态，则返回退出码 `0`，不报错，并输出 `already disabled`
- `mcfg validate`
  - 可选参数：`--json`
  - 行为：执行本地配置中心一致性校验、字段合法性校验、目标文件可同步性校验
  - 输出：返回 `valid` / `invalid` 结论、错误列表、警告列表、受管字段漂移摘要
- `mcfg sync`
  - 可选参数：`--dry-run`、`--json`、`--init-target`
  - `--init-target` 仅在目标文件缺失时自动创建最小骨架文件；不覆盖已有损坏文件
- `mcfg backup create`
  - 无额外参数
  - 前置条件：`~/.claude/settings.json` 与 `~/.claude.json` 必须同时存在，否则报错退出
- `mcfg backup list`
  - 可选参数：`--json`
- `mcfg backup prune`
  - 可选参数：`--keep <n>`、`--json`
  - 默认行为：保留最近 `3` 个快照，清理更旧的备份与损坏索引
- `mcfg backup restore <backup-id>`
  - 无额外参数
- `mcfg tui`
  - 无额外参数
  - 行为：显式进入 TUI 界面，与直接执行 `mcfg` 等价

### 7.12 待讨论事项

以下内容在开发前仍需单独确认，本版文档先保留为待决策项：

- TUI 中列表页是否默认显示短 ID（前 8 位）以便和 CLI 的前缀输入习惯保持一致
- `mcfg import` 在发现“同名但内容不同”的模型或 MCP 时，是否需要增加交互式重命名提示
- 后续若接入非 Claude Code 目标，目标绑定关系是否升级为 `targets[]` 结构

## 8. 成功标准

1. 首次启动时，如果 `~/.claude/settings.json` 或 `~/.claude.json` 存在，系统可以完成扫描、解析、导入，并向用户展示导入的 model 数量、mcp 数量和 skipped 数量
2. 如果 `~/.claude/settings.json` 不存在，系统不报致命错误，而是提示用户手动创建模型配置并继续使用工具
3. 如果配置文件格式损坏，系统提示修复原文件，不覆盖、不清空原始配置
4. 用户可以在 TUI 中完成模型和 MCP 的增删改查
5. 用户可以通过一次 CLI 命令或 TUI 2~3 步完成模型切换
6. `mcfg sync` 执行前必有同时包含 `~/.claude/settings.json` 与 `~/.claude.json` 的完整备份，执行失败时可以自动回滚
7. `mcfg sync --dry-run` 可以展示计划变更且不修改目标文件
8. 同步时不破坏 `~/.claude/settings.json` 与 `~/.claude.json` 中未受管字段
9. CLI 可以覆盖初始化、导入、状态查看、模型管理、MCP 管理、校验、同步、备份恢复等主要操作
10. 直接执行 `mcfg` 可以进入 TUI，TUI 可以覆盖主要高频操作场景，CLI 子命令可以覆盖自动化和脚本化场景
11. 任意时刻仅允许一个写进程持有独占锁；读命令可并发获取共享锁；写进程遇到锁冲突时会立即失败，并输出锁占用信息

## 9. 版本规划

### 9.1 V1

- 仅支持 Claude Code
- 首次启动自动扫描并导入 Claude Code 用户级现有配置
- 聚焦本地配置管理、切换、同步、备份
- 使用 Bubble Tea 实现 TUI
- 使用 Cobra 实现 CLI
- CLI 与 TUI 覆盖高频配置管理场景

### 9.2 后续版本

- 评估支持 Codex
- 评估支持其他兼容的 AI 工具
- 在不破坏内部统一配置模型的前提下扩展目标适配器

## 10. 开发任务列表

本章节将 V1 开发工作拆分为 4 个迭代，遵循以下原则：

- 先交付 CLI 最小可用链路，再补齐导入、同步、备份、锁等高风险能力
- TUI 在 CLI 服务层稳定后接入，避免重复实现业务逻辑
- 每个迭代结束后都必须达到“可编译、可运行、可验证”的状态

### 10.1 迭代一：配置中心基础能力

迭代目标：

- 建立本地配置中心基础骨架
- 打通模型与 MCP 的本地管理能力
- 交付最小 CLI 可用闭环

任务列表：

1. 项目骨架初始化
   - 初始化 Go 模块与目录结构
   - 建立 `cmd/`、`internal/`、`pkg/` 等基础分层
   - 接入 Cobra 根命令，预留 TUI 启动入口
2. 配置模型定义
   - 定义 `ConfigRoot`、`ModelProfile`、`MCPServer`、`ClaudeBinding`、`BackupMeta`
   - 统一时间字段、来源字段、`schema_version`
   - 定义配置文件序列化与反序列化规则
3. 本地存储能力
   - 实现 `~/.mcfg/`、`~/.mcfg/config.json`、`~/.mcfg/backups/` 基础读写
   - 实现配置原子写入与文件权限控制
   - 实现空配置初始化逻辑
4. ID 与引用机制
   - 接入 ULID 生成器
   - 实现完整 ID 与前缀 ID 查找
   - 实现歧义检测与引用校验
5. Model 管理命令
   - 实现 `mcfg model list`
   - 实现 `mcfg model add`
   - 实现 `mcfg model edit`
   - 实现 `mcfg model remove`
   - 实现 `mcfg model use`
6. MCP 管理命令
   - 实现 `mcfg mcp list`
   - 实现 `mcfg mcp add`
   - 实现 `mcfg mcp edit`
   - 实现 `mcfg mcp remove`
   - 实现 `mcfg mcp enable`
   - 实现 `mcfg mcp disable`
7. 状态查看命令
   - 实现 `mcfg status`
   - 输出当前模型、启用 MCP、最近同步状态
8. 基础校验与错误码
   - 实现 CLI 参数校验
   - 实现字段基础合法性校验
   - 接入统一退出码定义
9. 迭代一测试任务
   - 为配置读写、ID 解析、Model/MCP 基础服务补充单元测试
   - 为 CLI 基础命令补充最小冒烟测试

迭代验收：

- 用户可通过 CLI 完成模型和 MCP 的增删改查
- 用户可切换当前模型并维护已启用 MCP 集合
- 所有配置可稳定持久化到 `~/.mcfg/config.json`

### 10.2 迭代二：初始化、导入、校验与同步

迭代目标：

- 打通 Claude Code 配置导入链路
- 建立同步与校验能力
- 让本地配置中心成为 Claude Code 的唯一事实来源

任务列表：

1. `init` 初始化流程
   - 实现 `mcfg init`
   - 创建基础目录与空配置文件
   - 首次执行后自动触发扫描导入
   - 重复执行返回 `already initialized`
2. `import` 导入流程
   - 实现 `mcfg import`
   - 扫描 `~/.claude/settings.json`
   - 扫描 `~/.claude.json`
   - 仅读取用户级 `<当前用户 home 绝对路径>.mcpServers`
   - 按规则去重并记录 skipped 数量
3. Claude Code 适配器
   - 实现内部模型到 `settings.json` 顶层 `env` 的映射
   - 实现内部 MCP 到 `~/.claude.json` 路径节点 `mcpServers` 的映射
   - 保留未受管字段
4. `validate` 校验命令
   - 实现本地配置一致性校验
   - 实现字段合法性校验
   - 实现目标文件可同步性校验
   - 输出 `in_sync`、`out_of_sync`、`unavailable`
5. `sync` 同步命令
   - 实现 `mcfg sync`
   - 实现 `mcfg sync --dry-run`
   - 实现 `mcfg sync --init-target`
   - 接入“生成目标内容 -> 校验 -> 临时文件写入 -> 原子替换”流程
6. 同步结果回写
   - 成功时更新 `last_sync_at`
   - 成功时更新 `last_sync_result`
   - `model use --sync` 失败时回滚本地绑定变更
7. 人类可读输出与 JSON 输出
   - 为 `status`、`list`、`validate`、`sync --dry-run` 实现 `--json`
   - 统一高风险命令结果摘要
8. 迭代二测试任务
   - 为 `init`、`import`、`validate`、`sync`、`sync --dry-run` 补充集成测试
   - 覆盖目标文件缺失、目标文件损坏、未受管字段保留等关键场景

迭代验收：

- 已有 Claude Code 配置可成功导入到本地配置中心
- 本地配置可正确同步到 Claude Code 用户级配置文件
- 同步不会破坏未受管字段
- `validate` 可明确给出当前是否可同步、是否已漂移

### 10.3 迭代三：可靠性、备份与并发保护

迭代目标：

- 提升高风险命令的可靠性
- 确保多进程和外部修改场景下行为可预测
- 建立可恢复、可清理、可审计的运维能力

任务列表：

1. 统一运行锁
   - 实现 `~/.mcfg/run.lock`
   - 支持读共享、写独占
   - 输出锁冲突占用信息
   - 处理陈旧锁元信息恢复
2. 备份创建与索引
   - 实现 `mcfg backup create`
   - 同步前自动创建完整快照
   - 同步维护 `backup_index`
3. 备份查询与恢复
   - 实现 `mcfg backup list`
   - 实现 `mcfg backup restore`
   - 保证同一 `backup-id` 同时恢复两个目标文件
4. 备份清理策略
   - 实现自动保留最近 3 个快照
   - 实现 `mcfg backup prune`
   - 同步清理损坏索引和缺失文件记录
5. 回滚机制
   - 同步失败时自动回滚
   - 恢复失败时停止写入并输出明确错误
   - 保证异常场景不污染本地配置中心
6. 外部并发修改保护
   - 在 `sync`、`sync --dry-run`、`backup restore` 前后计算目标文件摘要
   - 检测文件被外部修改时立即失败
   - 输出受影响文件路径与重试提示
7. 高风险命令治理
   - 统一 `remove --force` 行为
   - 统一敏感字段脱敏显示
   - 完善退出码分级和错误摘要
8. 迭代三测试任务
   - 为运行锁、备份恢复、回滚、外部并发修改检测补充异常场景测试
   - 覆盖锁冲突、恢复失败、漂移检测失败、损坏备份清理等场景

迭代验收：

- 发生同步失败、锁冲突、外部并发修改时，不会破坏现有配置
- 用户可以创建、查看、恢复、清理备份
- 写操作在并发环境下具备明确失败语义

### 10.4 迭代四：TUI 交互交付

迭代目标：

- 交付覆盖高频操作的 Bubble Tea TUI
- 保证 TUI 与 CLI 共用同一套服务层逻辑

任务列表：

1. TUI 启动入口
   - 实现 `mcfg` 默认进入 TUI
   - 实现 `mcfg tui`
   - 启动前获取独占锁
2. 主布局与状态栏
   - 实现左侧导航
   - 实现右侧主面板
   - 实现底部状态栏
3. Overview 页面
   - 展示当前模型
   - 展示启用 MCP 数量
   - 展示最近同步结果
   - 展示目标文件状态
4. Models 页面
   - 实现列表页
   - 实现详情页
   - 实现编辑页
   - 实现 `Use Model`
5. MCP Servers 页面
   - 实现列表页
   - 实现详情页
   - 实现编辑页
   - 实现启用/禁用切换
6. Sync Preview 页面
   - 展示受管字段变更摘要
   - 提供确认同步入口
7. Backups 页面
   - 展示备份列表
   - 展示备份原因与时间
   - 提供恢复确认流程
8. 快捷键与交互一致性
   - 接入 `j`、`k`、方向键、`enter`、`esc`
   - 接入 `a`、`e`、`d`、`u`、`space`、`s`、`r`、`q`
   - 确保 TUI 调用与 CLI 服务层行为一致
9. 迭代四测试任务
   - 为 TUI 关键流程补充交互级测试或最小冒烟测试
   - 覆盖模型切换、MCP 启停、同步预览、备份恢复的核心路径

迭代验收：

- 用户可在 TUI 中完成模型切换、MCP 启停、同步预览、备份恢复
- TUI 与 CLI 的校验、写入、错误行为保持一致
- 敏感 token 默认掩码显示

## 11. 开发前审核

本节用于在正式编码前确认当前文档是否已具备开工条件，并记录仍需关注的风险。

### 11.1 审核结论

- 当前文档已经达到 V1 开发启动条件
- CLI 路径、数据模型、受管字段、同步语义、备份语义、锁语义已经足够明确
- TUI 也已有最小交付定义，可以在 CLI 服务层稳定后开始实现

### 11.2 可以直接开工的内容

- 本地配置中心目录与数据结构
- CLI 命令骨架与参数解析
- Model/MCP 的服务层与持久化
- Claude Code 用户级配置扫描、导入、适配、同步
- 备份、回滚、运行锁、校验能力

### 11.3 仍需重点关注的风险

1. TUI 展示短 ID 的决策未定
   - 不影响 CLI 和服务层开发
   - 影响 TUI 列表页呈现细节
2. `import` 发现“同名但内容不同”配置时的交互策略未定
   - 当前文档只明确“不自动合并”
   - 若后续要加交互式重命名，需要额外 UI 与 CLI 策略
3. `validate --json`、`status --json`、`list --json` 的最终字段稳定性需尽早冻结
   - 若过晚调整，可能引起 CLI 输出兼容性变更
4. 外部修改竞争保护依赖摘要比对实现细节
   - 需要在实现阶段明确摘要算法、比较时机、错误路径输出格式

### 11.4 建议的开发前补充动作

1. 在编码前补一份最小包结构说明
   - 明确 `store`、`service`、`validator`、`adapter`、`runtime lock` 归属
2. 在编码前补一份 CLI 输出示例
   - 至少覆盖 `status`、`validate`、`sync --dry-run`、锁冲突报错
3. 在编码前先确定 JSON 输出字段
   - 避免后续脚本化场景出现不兼容调整
4. 在进入 TUI 开发前先冻结服务层接口
   - 避免 TUI 与 CLI 分叉实现

### 11.5 审核建议

- 开发顺序固定采用“CLI 基础 -> 导入与同步 -> 可靠性治理 -> TUI”
- 迭代一结束后先做一次命令链路评审
- 迭代二结束后先做一次同步安全评审
- 迭代三结束后再进入 TUI 交互开发
