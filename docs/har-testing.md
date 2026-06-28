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

### 扩展 PII 守卫（v0.3.5+）

`test/integration/har_pii_redacted_test.go` 从仅扫描 HAR fixtures 扩展到全仓库 `*_test.go` 文件，通过 Go AST 扫描字符串字面量，捕获数字型学生 ID 等 PII（如 <学生ID> / <用户ID> / <学号ID> 等格式）。**默认 tag 运行**（无 build tag），确保 `go test ./...` 必跑。

## 局限性

- **中文乱码**：HAR 里的中文是 GBK 编码，Python 读取后输出会乱码，但 JSON 结构完整
- **过期数据**：HAR 是历史快照，平台 API 可能已变更
- **覆盖率有限**：只覆盖了抓包时用到的端点
- **不模拟服务端状态**：mock server 只返回固定响应，不处理状态变更
