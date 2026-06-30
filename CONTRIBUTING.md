# 贡献指南

感谢你考虑为 nazhi-cli 贡献代码！

## 开发流程

1. Fork 本仓库
2. 创建功能分支：`git checkout -b feat/your-feature`
3. 提交变更（遵循[提交规范](#提交规范)）
4. 推送分支并创建 Pull Request

## 环境要求

- Go 1.26+（见 `go.mod`）
- 交叉编译无需额外依赖（ONNX Runtime DLL 已内嵌）
- 首次贡献推荐本地跑通 `make build` 与 `make test`

## 本地开发

```bash
# 构建（含 OCR，CI release 用此命令）
go build -tags=ddddocr -o bin/nazhi.exe ./cmd/nazhi

# 运行测试（race 检测）
make test

# 代码检查
make vet
make lint
make fmt
```

> ⚠️ **`make build` 不带 `-tags=ddddocr`**，产出的二进制 `c.ocr=nil`，
> `nazhi login` 会立即返回 `ErrOCRNotConfigured`。本地想跑通登录必须显式带 tag。
> Makefile / CI 的 `release` job 都已带 tag，无需手动加。

## 提交规范

提交信息遵循 [Conventional Commits](https://www.conventionalcommits.org/)。
本仓库常用 scope：`pkg/client`、`pkg/types`、`pkg/tokenparse`、`cmd/nazhi`、`internal/ocr`、`docs`、`ci`、`deps`。

```
feat(pkg/client): 添加新接口或 Option
fix(cmd/nazhi): 修复 CLI 行为
refactor(pkg/types): 整理解码辅助
test(pkg/client): 补充场景覆盖
docs: 更新用户文档
ci: 调整 workflow
chore(deps): 依赖升级
```

实战提交示例（真实历史 commit 风格）：

- `fix(ocr): 降级限定 Windows 平台，避免 Linux EIO/EPIPE 误吞`
- `fix(cookie_sync): 修正 RawData nil 测试为 group A 实现的空 body 场景`
- `merge: 修复 Windows OCR tempdir 清理 DLL 占用降级（TDD）`

> 中文描述也可以接受，但保持一行能概括为佳。BREAKING CHANGE 在 body 末尾
> 用 `BREAKING: xxx` 单独成段说明。

### 提交格式硬性约束

- **subject 必须以 `<type>(<scope>): <subject>` 开头**，例如 `fix(ocr): xxx`
- **禁止任何装饰字符、空 preamble 行、机器人标签或横幅**（如 `@bot`、机器人自动生成的脚注、CI 触发器的复读等）
- **禁止以 `@`、`[`、`(`、`<` 等非 Conventional 字符开头**——这类前缀会被 Git 工具链误判为
  mention/标签或与 changelog 生成器冲突，导致 release 工具解析失败

## Pull Request

- PR 标题同样遵循 Conventional Commits
- 附上变更说明、测试结果、关联 Issue（若有）
- 保持单次 PR 只聚焦一个功能 / 修复
- 跨多个 worktree 的修复，按 fix group 拆 commit，merge 时 `git merge --no-ff` 保留结构

## ⚠️ push 前必跑（CI 6 步铁律）

CI 含 6 个独立 gate，每个都可能单独 fail。**绝不能只跑 `go test` 就 push**。
本地一键验证（全绿才能 push）：

```bash
# 1. go mod tidy 整洁
go mod tidy && git diff --exit-code go.mod go.sum

# 2. golangci-lint
"$(go env GOPATH)/bin/golangci-lint" run --timeout=5m ./...

# 3. go vet（两个 build tag 都要）
go vet ./...
go vet -tags=integration ./test/integration/...

# 4. gofmt（无输出才算通过）
[ -z "$(gofmt -l .)" ] || { echo "FAIL: $(gofmt -l .)"; exit 1; }

# 5. 单元测试（race）
go test -count=1 -race -timeout=15m ./pkg/...

# 6. 集成测试编译验证（跑空测试树，仅验证编译）
go test -tags=integration -run=^$ ./test/integration/...
```

`test` 验逻辑、lint 验风格+死代码、vet 验类型、gofmt 验格式、mod tidy 验依赖一致——五件
事彼此独立，缺一不可。任何一步 fail 必须回到对应环节修根因，不要绕。

## 协议

贡献的代码将遵循 MIT 协议。
