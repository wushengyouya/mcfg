# Release

## Version Metadata

`mcfg` 支持以下构建注入字段：

- `version`
- `commit`
- `build_date`

运行时可通过下面命令查看：

```bash
mcfg version
mcfg version --json
mcfg --version
```

## Local Release Build

最小本地发布构建流程：

```bash
bash scripts/build-release.sh v0.1.0
```

默认输出到 `dist/`，二进制名称格式为：

```text
mcfg-<goos>-<goarch>
```

同时会生成：

```text
mcfg-<goos>-<goarch>.sha256
```

也可以指定输出目录：

```bash
bash scripts/build-release.sh v0.1.0 /tmp/mcfg-dist
```

## Optional Metadata

可通过环境变量覆盖提交信息与构建时间：

```bash
COMMIT=abc1234 BUILD_DATE=2026-03-13T00:30:00Z bash scripts/build-release.sh v0.1.0
```

## Recommended Checklist

1. 运行 `env GOCACHE=/tmp/go-build GOMODCACHE=/tmp/gomodcache go test ./...`
2. 更新 `CHANGELOG.md`
3. 执行 `bash scripts/build-release.sh <version>`
4. 校验 `dist/mcfg-<goos>-<goarch>.sha256`
5. 运行 `dist/mcfg-<goos>-<goarch> version`
6. 打 tag 并发布产物
