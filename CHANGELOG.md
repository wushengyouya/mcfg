# Changelog

## Unreleased

### Added

- 基于 Cobra 的 CLI，覆盖 `init`、`import`、`status`、`validate`、`sync`、`backup`、`model`、`mcp`、`tui`
- 基于 Bubble Tea 的默认 TUI 入口，支持模型切换、MCP 启停、同步预览、备份恢复、模型与 MCP 的增删改
- 本地配置中心与统一数据模型，使用 `~/.mcfg/config.json` 持久化
- Claude Code 配置扫描、导入、同步、校验与备份能力
- 运行锁、外部修改检测、同步失败回滚、备份恢复保护
- `status`、`validate`、`model list`、`mcp list`、`sync --dry-run`、`backup` 等命令的 JSON 输出
- 命令帮助示例与 README 使用说明

### Changed

- `status` 的人类可读输出补充为模型、MCP 名称、同步状态、最近同步时间四段摘要
- `remove --force` 成功输出现在明确提示是否自动解除当前引用关系
- `model remove` / `mcp remove` 在未加 `--force` 且被引用时，错误消息会给出下一步建议命令

### Quality

- 按 TDD 持续补齐单元测试与 CLI 集成测试
- 固定 `--help` 示例和 JSON 输出字段契约，降低后续回归风险
