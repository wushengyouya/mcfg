# mcfg

Claude Code 本地配置中心，使用 Go 实现，提供：

- Cobra CLI
- Bubble Tea TUI
- Model / MCP 统一管理
- Claude Code 配置导入、校验、同步、备份
- 运行锁、回滚、外部修改检测

## 环境

- Go 1.26+
- macOS / Linux

## 开发

```bash
go test ./...
go run .
```

如果本地环境对 Go 缓存目录有限制，可以这样跑：

```bash
env GOCACHE=/tmp/go-build GOMODCACHE=/tmp/gomodcache go test ./...
```

## 初始化

```bash
go run . init
```

初始化会创建：

- `~/.mcfg/config.json`
- `~/.mcfg/backups/`
- `~/.mcfg/run.lock`

首次初始化后会尝试从下面两个文件导入 Claude Code 用户级配置：

- `~/.claude/settings.json`
- `~/.claude.json`

## 常用命令

查看状态：

```bash
go run . status
go run . status --json
go run . validate
go run . validate --json
```

管理模型：

```bash
go run . model add --name "Claude Sonnet" \
  --base-url "https://example.com" \
  --model "claude-sonnet-4-0" \
  --auth-token "secret"

go run . model list
go run . model list --json
go run . model use <id>
go run . model use <id> --sync
```

管理 MCP：

```bash
go run . mcp add --name filesystem --command npx --args -y --args @modelcontextprotocol/server-filesystem
go run . mcp list --json
go run . mcp enable <id>
go run . mcp disable <id>
```

同步与预览：

```bash
go run . sync --dry-run
go run . sync --dry-run --json
go run . sync --init-target
```

备份：

```bash
go run . backup create
go run . backup list
go run . backup list --json
go run . backup restore <backup-id>
go run . backup restore <backup-id> --json
go run . backup prune --keep 3
go run . backup prune --keep 3 --json
```

帮助与示例：

```bash
go run . --help
go run . model add --help
go run . sync --help
go run . version
```

## TUI

直接启动：

```bash
go run .
```

或：

```bash
go run . tui
```

默认快捷键：

- `h/l` 或左右切页
- `j/k` 或上下移动
- `a` 新增
- `e` 编辑
- `d` 删除
- `u` 使用模型
- `space` 启停 MCP
- `s` 进入同步预览
- `r` 刷新当前页
- `q` 退出

## 说明

- 所有写命令和 TUI 使用独占锁
- 只读命令使用共享锁
- `sync` 和 `backup restore` 会检测目标文件是否被外部修改
- `sync` 失败时会回滚到本次同步前的备份

## 发布

查看版本信息：

```bash
go run . version
go run . version --json
go run . --version
```

本地构建发布包：

```bash
bash scripts/build-release.sh v0.1.0
```

更完整的发布步骤见 `RELEASE.md`。

## 存在的问题
1. 不需要同步功能，进行操作时直接同步到原始文件
2. ui设计需要重构