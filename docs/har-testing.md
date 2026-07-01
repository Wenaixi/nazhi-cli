# HAR 驱动集成测试

## 背景

`test/integration/har_fixtures/` 下的 JSON 文件是从 Nazhi-auto 旧版 v1 的真实 HAR 抓包中提取的 API 响应 fixture。这些 fixtures 让 SDK 集成测试**无需真实期末数据**就能测 FetchTasks、SubmitTask 等任务相关方法。

意义：

- 不依赖「当前是几月、是否有未完成任务」
- 不依赖特定学生的个人数据
- 一次抓包永久复用，业务字段变了再重新抓
- 离线可跑（CI 不需要连真服务）

## 工作原理

```
HAR 抓包 → Python 解析 → 关键端点 fixtures JSON
                                   ↓
                         httptest.Server 按路径路由
                                   ↓
                         真 SDK 客户端调用
                                   ↓
                         验证 SDK 解析 + 请求格式
```

抓包在 NaZHi-auto v1 时期一次性完成，提取的 fixtures 覆盖主要业务路径。**fixture 不会自动更新**——若服务端 API 变更（字段重命名、返回值结构变了），需要重新抓包。

## Fixture 文件清单

| 文件 | 来源 HAR | 端点数 | 用途 |
|---|---|---|---|
| `task_flow.json` | `获取任务列表提交一次任务.har` | 5 | FetchTasks + SubmitTask 通用 |
| `self_eval.json` | `上传自我评价.har` | 6 | SubmitSelfEvaluation + QuerySelfEvaluation |
| `military.json` | `军训的提交.har` | 2 | SubmitTask 军训类型 |
| `class_meeting.json` | `班会的提交.har` | 2 | SubmitTask 班会类型 |
| `labor.json` | `完成一次劳动的提交.har` | 4 | SubmitTask 劳动类型 |

## 端点提取逻辑

一次性 Python 脚本（已在 v1 时期完成，HAR fixtures 本身已经提取好）：

```python
import json
for entry in har['log']['entries']:
    if 'nazhisoft' in entry['request']['url'] or '139.159' in entry['request']['url']:
        fixtures[key] = {
            'method': method,
            'path': path,
            'request_body': body,
            'response_status': status,
            'response_body': resp_body,
        }
```

## 测试运行方式

```bash
# 单元测试（mock server，无需环境变量）
go test -race -count=1 ./pkg/...

# 真实环境集成（需 NAZHI_USERNAME / NAZHI_PASSWORD）
NAZHI_USERNAME=学号 NAZHI_PASSWORD=密码 \
  go test -tags=integration -run TestReal_FullChain -v ./test/integration/...

# HAR 驱动测试（无需环境变量，但需要 build tag=integration）
go test -tags=integration -run TestHAR_ -v ./test/integration/...

# 编译验证（确保 integration 测试树能编译）
go test -tags=integration -run=^$ ./test/integration/...
```

**HAR 驱动测试用 build tag=integration**：因为这些测试需要 fixture 文件 + HTTP server，但不要真实凭据。
和真实环境集成测试共用 tag 但 `run TestHAR_` 即可区分。

## 添加新 Fixture

1. **抓取 HAR**：用 Chrome DevTools 录制真实请求
   - 打开 Network → 右键 → "Save all as HAR with content"
2. **过滤端点**：只保留 `nazhisoft.com` 或 `139.159.205.146` 的请求
3. **运行 Python 脚本**生成 JSON（脚本可在 git history 里找，或写一个一次性脚本）
4. **放置到** `test/integration/har_fixtures/<name>.json`
5. **写测试**用 `loadFixturesByName(t, "<name>.json")` 加载

```go
// loadFixturesByName 加载 fixture 并按 HTTP path 路由
func loadFixturesByName(t *testing.T, name string) map[string]httptest.Handler {
    t.Helper()
    f := loadFixtureFile(t, "test/integration/har_fixtures/"+name)
    handlers := map[string]httptest.Handler{}
    for _, ep := range f.Endpoints {
        ep := ep
        handlers[ep.Path] = func(w http.ResponseWriter, r *http.Request) {
            // 校验 method、body 字段
            // 返回 fixture 的 response_body
        }
    }
    return handlers
}
```

6. **不要提交**真实凭据 / 敏感数据（fixture 也走 PII 守卫扫描）

## 已验证任务类型字段差异

HAR 验证的真实场景（不同任务类型字段差异）：

| 字段 | 劳动 | 军训 | 班会 | 通用 |
|---|---|---|---|---|
| `name` | 任务原名 | `""` | `"班会"` | 任务原名 |
| `level` | `"5"` | `""` | `""` | `""` |
| `checkResult` | `""` | `"1"` | `""` | `""` |
| `address` | 学校名 | 学校名 | 班级名 | `""` |
| `orgName` | 学校名 | 学校名 | `""` | `""` |
| `playRole` | `""` | `""` | `"3"` | `"3"` |
| `hours` | `2.0` | `32.0` | `1.0` | `0.5` |
| `circleTaskId` | 16512 | 16513 | 16324 | — |
| `circleTypeId` | 9275 | 3691 | 9256 | — |
| `dimensionId` | 14 | 13 | 9 | — |

这些字段差异是 HAR 抓的真实样本，CLI / SDK 任务提交流程不能假设它们。

## HAR 驱动测试发现并修复的 Bug

通过 HAR 驱动测试发现：

1. **Task.StartDate 字段错配**：JSON tag 用了 `startDate`（数组），但平台返回 `startDateStr`（字符串）。修复后 SDK 能正确解析所有任务。
2. **extractTokenFromLocation 脆弱**：原代码用 `strings.Index`，无法处理 fragment + URL 编码。改用 `net/url.Parse`（最终归入 `pkg/tokenparse`）。
3. **FetchTasks 静默失败**：单维度失败被吞，改用 `c.logDebug` 记录 + 错误链 propagate（v0.4.0+ 全维度错误聚合到 `ErrBusinessRejected`）。
4. **Cookie token 同步缺失**：`WithToken` 之前只写 Header，业务接口返回空——补 Cookie 同步。
5. **OCR 池非线程安全**：`Pool.Recognize` 内部 ONNX session 是不可重入，加上 `sync.Mutex` 串行化。

### 扩展 PII 守卫（v0.3.5 重写为 SHA-256 哈希方案）

`test/integration/har_pii_redacted_test.go` 的 `TestNoRealPII` 从仅扫描 HAR fixtures 扩展到全仓库 `*_test.go` + `har_fixtures/*.json`，通过 Go AST + JSON 遍历扫描所有字符串字面量。**默认 tag 运行**（无 build tag），确保 `go test ./...` 必跑。

**双检策略**：
1. **模式匹配**：正则捕获 `G\d{15}`（学号）、18 位身份证号等已知格式，捕获新增值。
2. **哈希比对**：对 AST 解析出的每个字符串字面量 / JSON 字符串值算 SHA-256 hex 摘要，与预计算的 PII 哈希集比对（见源码中 `piiHexMap`）。

#### 自反性陷阱与修复

v0.3.5 之前的守卫把真实姓名、学号、身份证号等明文写在 `forbidden` 常量里（"用 PII 防御 PII"）。结果是守卫自身成了新的泄露源——任何能看仓库源码的人都能直接读到这些明文 PII（包括 hex 字符串拼接绕过 AST 自检的变体）。

修复后 `piiHexMap` 只存 64 字符 SHA-256 hex 摘要（单向不可逆）：

```go
var piiHexMap = map[string]string{
    // 真实学号的 SHA-256 hex 摘要，不可反推原文
    "8a3f0d9c2e1b4a7f6c5d8e9b1a2f3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b": "学号",
    "1f2e3d4c5b6a7f8e9d0c1b2a3f4e5d6c7b8a9f0e1d2c3b4a5f6e7d8c9b0a1f2e": "姓名",
    "4b3c2d1e0f9a8b7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e2f1a0b9c8d7e6f5a4b3": "身份证号",
}
```

**教训**：「用 PII 的不可逆表示，而非 PII 本身」；字符串拼接只防「自己看自己」，防不住「别人看你」。

#### 扫描范围

- `pkg/**/*.go` 中字符串字面量
- `cmd/**/*.go` 中字符串字面量
- `internal/**/*.go` 中字符串字面量
- `test/integration/har_fixtures/*.json` 中所有字符串值
- `docs/**/*.md` 中示例学号（保留 `TEST2025001` / `S1234567890` 等占位符）

`make lint` 顺带跑（golangci-lint 自定义规则或脚本），CI 中作为必过 gate。

## 历史 fixture vs 当前服务端

**fixture 不会自动同步服务端**——它是 v1 时期一次性抓的固定快照。

如果发现：

- SDK 测试通过但生产环境真实接口失败 → 可能是 fixture 与当前接口签名不一致
- 修复方法：重新抓 HAR → 更新 fixture → 修复 SDK

**关键原则**：fixture 反映 SDK 当时能正确解析的**某一时刻**服务端形态，不反映服务端演化。

## 局限性

- **中文乱码**：HAR 里的中文是 GBK 编码，Python 读取后输出会乱码，但 JSON 结构完整
- **过期数据**：HAR 是历史快照，平台 API 可能已变更
- **覆盖率有限**：只覆盖了抓包时用到的端点，新增端点需重新抓
- **不模拟服务端状态**：mock server 只返回固定响应，不处理状态变更
- **不模拟错误路径**：fixture 都是 `code=1` 成功响应，业务错误路径用其他手段覆盖（见 `pkg/client/*_test.go` 的 httptest 用例）

## 不依赖 HAR 的单元测试

| 测试类型 | 文件 | 作用 |
|---|---|---|
| HTTP 方法 / 路径 / 头 / body 校验 | `pkg/client/*_test.go` | 每个 SDK 方法都用 `httptest.Server` mock 服务端，校验 HTTP 细节 |
| Cookie jar 独立性 | `pkg/client/concurrency_test.go` | 多 Client 并发调用互不污染 |
| 错误分支 | `pkg/client/*_test.go` | 网络错、业务拒绝、超时、5xx 等用 httptest.Server 模拟 |
| Sentinel 触发 | `pkg/client/errors_test.go` | 验证每个 `ErrXxx` 被正确包装 |
| OCR mock | `pkg/client/ocr_*_test.go` | `WithCustomOCR(mock)` 注入 mock，验证 Login 的 99 张图重试、退避、ctx cancel |

HAR 驱动测试只是「业务逻辑 + fixture 校验」，不做 HTTP 协议级校验。所以两套测试互补，不重复。
