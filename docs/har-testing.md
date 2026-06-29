# HAR 驱动集成测试

## 背景

`test/integration/har_fixtures/` 目录下的 JSON 文件是从 Nazhi-auto 旧版 v1 的真实 HAR 抓包中提取的 API 响应数据。这些 fixtures 让我们**无需期末数据**就能测试 FetchTasks、SubmitTask 等任务相关方法。

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

## Fixture 文件

| 文件 | 来源 HAR | 端点数 | 用途 |
|------|---------|--------|------|
| `task_flow.json` | `获取任务列表提交一次任务.har` | 5 | FetchTasks + SubmitTask 通用 |
| `self_eval.json` | `上传自我评价.har` | 6 | SubmitSelfEvaluation + QuerySelfEvaluation |
| `military.json` | `军训的提交.har` | 2 | SubmitTask 军训类型 |
| `class_meeting.json` | `班会的提交.har` | 2 | SubmitTask 班会类型 |
| `labor.json` | `完成一次劳动的提交.har` | 4 | SubmitTask 劳动类型 |

## 端点提取逻辑

```python
# Python 脚本（一次性运行）
import json
for entry in har['log']['entries']:
    if 'nazhisoft' in entry['request']['url'] or '139.159' in entry['request']['url']:
        # 提取路径、方法、请求体、响应
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
# 单元测试（mock server）
go test -race -count=1 ./pkg/client/...

# 真实环境测试（需要环境变量）
NAZHI_USERNAME=学号 NAZHI_PASSWORD=密码 \
  go test -tags=integration -run TestReal_FullChain -v ./test/integration/...

# HAR 驱动测试（无需环境变量）
go test -tags=integration -run TestHAR_ -v ./test/integration/...
```

## 添加新 Fixture

1. **抓取 HAR**：用 Chrome DevTools 录制真实请求
2. **过滤端点**：只保留 `nazhisoft.com` 或 `139.159.205.146` 的请求
3. **运行 Python 脚本**生成 JSON
4. **放置到** `test/integration/har_fixtures/<name>.json`
5. **写测试**用 `loadFixturesByName(t, "<name>.json")` 加载
6. **不要提交**真实凭据 / 敏感数据

## 已验证任务类型字段差异

| 字段 | 劳动 | 军训 | 班会 | 通用 |
|------|------|------|------|------|
| `name` | 任务原名 | `""` | `"班会"` | 任务原名 |
| `level` | `"5"` | `""` | `""` | `""` |
| `checkResult` | `""` | `"1"` | `""` | `""` |
| `address` | 学校名 | 学校名 | 班级名 | `""` |
| `orgName` | 学校名 | 学校名 | `""` | `""` |
| `playRole` | `""` | `""` | `"3"` | `"3"` |
| `hours` | `2.0` | `32.0` | `1.0` | `0.5` |
| `circleTaskId` | 16512 | 16513 | 16324 | - |
| `circleTypeId` | 9275 | 3691 | 9256 | - |
| `dimensionId` | 14 | 13 | 9 | - |

## 发现并修复的 Bug

通过 HAR 驱动测试发现：

1. **Task.StartDate 字段错配**：JSON tag 用了 `startDate`（数组），但平台返回 `startDateStr`（字符串）。修复后 SDK 能正确解析所有任务。
2. **extractTokenFromLocation 脆弱**：原代码用 `strings.Index`，无法处理 fragment。改用 `net/url.Parse`。
3. **FetchTasks 静默失败**：单维度失败被吞，改用 `c.logDebug` 记录。

### 扩展 PII 守卫（v0.3.5 重写为 SHA-256 哈希方案）

`test/integration/har_pii_redacted_test.go` 的 `TestNoRealPII` 从仅扫描 HAR fixtures
扩展到全仓库 `*_test.go` + `har_fixtures/*.json`，通过 Go AST + JSON 遍历扫描所有
字符串字面量。**默认 tag 运行**（无 build tag），确保 `go test ./...` 必跑。

**双检策略**：
1. **模式匹配**：正则捕获 `G\d{15}`（学号）、18 位身份证号等已知格式，捕获新增值。
2. **哈希比对**：对 AST 解析出的每个字符串字面量 / JSON 字符串值算 SHA-256 hex 摘要，
   与预计算的 PII 哈希集比对（见源码中 `piiHexMap`）。

#### 自反性陷阱与修复

v0.3.5 之前的守卫把真实姓名、学号、身份证号等明文写在 `forbidden` 常量里
（"用 PII 防御 PII"）。结果是守卫自身成了新的泄露源——任何能看仓库源码的人都
能直接读到这些明文 PII（包括 hex 字符串拼接绕过 AST 自检的变体）。

修复后 `piiHexMap` 只存 64 字符 SHA-256 hex 摘要（单向不可逆）：

- 任何人看到这些 hex 也无法反推出原始 PII
- 守卫扫描时算 hash 查表，命中即报违规
- 教训：「用 PII 的不可逆表示，而非 PII 本身」；字符串拼接只防"自己看自己"，防不住"别人看你"

## 局限性

- **中文乱码**：HAR 里的中文是 GBK 编码，Python 读取后输出会乱码，但 JSON 结构完整
- **过期数据**：HAR 是历史快照，平台 API 可能已变更
- **覆盖率有限**：只覆盖了抓包时用到的端点
- **不模拟服务端状态**：mock server 只返回固定响应，不处理状态变更
