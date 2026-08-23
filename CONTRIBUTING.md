# 贡献指南

感谢你考虑为 nazhi-cli 贡献代码！

## 开发流程

1. Fork 本仓库
2. 创建功能分支：`git checkout -b feat/your-feature`
3. 提交变更（遵循[提交规范](#提交规范)）
4. 推送分支并创建 Pull Request

## 环境要求

- Go 1.26+（见 `go.mod`）
- 交叉编译无需 CGO 或本地模型依赖；验证码识别由运行时注入视觉模型完成
- 首次贡献推荐本地跑通 `make build` 与 `make test`

## 当前版本

仓库 `internal/version/version.go` 是版本号唯一写入处，CI 与 Makefile 从这里读。当前活跃版本：

| 版本 | 状态 | 备注 |
|---|---|---|
| `1.3.0` | 当前活跃维护 | 前端字段对齐、视觉识别依赖注入、纯 Go 构建与文档同步 |
| `0.4.0` | 历史版本 | 架构深化与安全修复 |
| `< 0.3` | 不再支持 | 强制升级 |

新功能开发默认向 `main` 提 PR，因为修复横跨多个 commit，main 上的版本号由发版时统一切。

## 本地开发

```bash
# 构建（纯 Go；登录时通过 NAZHI_SILICONFLOW_API_KEY 注入视觉模型）
go build -o bin/nazhi.exe ./cmd/nazhi

# 运行测试（race 检测）
make test

# 代码检查
make vet
make lint
make fmt
```

> **验证码识别说明**：SDK 不内置本地识别器。`nazhi login` 必须通过 `NAZHI_SILICONFLOW_API_KEY` 注入 Nazhi-auto 同款硅基流动 Qwen3-Omni 视觉模型；未配置时返回 `ErrOCRNotConfigured`。CLI 兼容 `NAZHI_OCR_API_KEY` 和 `SILICONFLOW_API_KEY` 旧别名。

## 提交规范

提交信息遵循 [Conventional Commits](https://www.conventionalcommits.org/)。
本仓库常用 scope：`pkg/client`、`pkg/types`、`pkg/tokenparse`、`cmd/nazhi`、`docs`、`ci`、`deps`。

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

- `refactor(captcha): 统一视觉识别器测试语义`
- `chore(captcha): 移除本地识别依赖与构建标签`
- `docs: 同步视觉识别注入与纯 Go 构建说明`

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

## push 前必跑 CI 6 步

`go test` 不能替代完整 CI。CI 有 6 个独立 gate，每个都可能单独 fail。
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
