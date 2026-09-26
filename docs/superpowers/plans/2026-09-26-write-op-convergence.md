# CLI 写操作族收敛（C2 深化）实施计划

> **状态**：已完成（2026-09-26，commits 97b2734..00588df）
> **执行方式**：内联执行（executing-plans + TDD），非子代理。

**Goal**：把 7 个 CLI 写操作命令的内联 6 步骨架收敛为单一共享控制流 runner，消除三套 unknown-key helper 的行为 drift。

**Architecture**：新增 `cmd/nazhi/write_op_runner.go` 提供 `runWriteOp(cmd, mode, branch)`；命令间差异（解码器/未知键集/flag 覆盖/SDK 调用/成功 envelope）由 `writeOpMode` 配置表达，命令 Run 退化为一行委托。与既有 `circleListMode`（列表族）同构，但保留写操作族「先本地校验再建客户端」契约（P2-E 十三域审计不变式）。

**Tech Stack**：Go 1.26 + cobra + 既有 envelope/logx 输出管线。

**Spec**：架构审查报告（`%TEMP%\architecture-review-2026-09-26-nazhi-cli.html`）候选 C2；设计决策见下。

## 关键决策

1. **错误优先次序单点定义**：缺 --payload → 坏 payload → 未知键 → 建客户端 → 解码 → id 校验 → applyFlags → 调用 → envelope。重构前 typical-case submit 是「先解码后拒未知键」，统一为「先拒后解码」（与 honor add 一致；合法 payload 无可见差异）。
2. **未知键统一 `unknownUpdatePayloadKeys`**（ToLower 折叠 + 稳定排序），允许集统一小写存储。`unknownUserUpdateKeys` 删除（死代码）；`userUpdateAllowedKeys` 小写化（原 camelCase + 大小写敏感查询是真实 drift：`{"Telephone":...}` 误判未知，注释称对齐实际没对齐）。
3. **preview 用 `writeOpBranch`** 在 --edit 提交/编辑分支间切换，每个分支自洽（自己的 decode + applyFlags + call），避免跨类型 `any` 槽位硬转。
4. **update 命令用可选 `validateID` 钩子**（decode 后、applyFlags 前），honor/typical update 孪生（42 行×2）收敛。
5. **遗留不收敛**：typical-case delete/delete-batch、honor delete 是 flag 校验派（非 payload 骨架），独立保留属有意设计。

## 任务清单（TDD 红→绿）

- [x] **T1 未知键收敛**（97b2734）：`unknownUserUpdateKeys` 改调 `unknownUpdatePayloadKeys`；红：`TestUserUpdate_UnknownKeys_Drift`（断言折叠放行，重构前 FAIL）。
- [x] **T2 回归修复**（27dd8aa）：`userUpdateAllowedKeys` 小写化；红：全量 `TestUserUpdateCmd_FriendlyKeysRemap` FAIL（合法键 genderName 误判未知）→ 绿。
- [x] **T3 task 族骨架**（64c814c）：submit/edit/preview 收敛到 `runWriteOp`；契约由 `write_op_skeleton_test.go` 锁定（未知键零请求 ×3、缺 payload 优先、flag 覆盖纯逻辑）。
- [x] **T4 add 族**（fdd8fb1）：honor add / typical submit / user update 收敛；契约由 `write_op_family_test.go` 锁定。
- [x] **T5 update 孪生**（00588df）：honor/typical update 收敛（新增 `validateID` 钩子）。

## 契约测试文件

- `cmd/nazhi/unknown_keys_converged_test.go` — Drift / FoldCase / StableSort / NonObject
- `cmd/nazhi/write_op_skeleton_test.go` — 未知键零请求、缺 payload 优先、flag 覆盖纯逻辑
- `cmd/nazhi/write_op_family_test.go` — 4 命令缺 payload 优先、未知键零请求 ×3

## 验证证据

- `go build ./...`、`go vet ./...`、`gofmt -l cmd/nazhi/` 空
- `go test ./cmd/nazhi/` 全绿（含全部新契约测试）
- `go test ./pkg/... ./internal/...` 全绿（pkg/client 连续 3 次 status=0；一次偶发 FAIL 为本机 httptest flaky，见 CLAUDE.md「本机 httptest flaky」节，与本次改动无因果）
- `go test -tags=docrules ./test/docrules` 全绿

## 后续规范

新增写操作命令必须复用 `runWriteOp`，禁止再内联骨架；新增未知键拒绝必须走 `unknownUpdatePayloadKeys` + 小写 allowed 集。
