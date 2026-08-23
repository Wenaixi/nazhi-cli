# 日志系统增强 — 实施计划

> **面向 Agent 执行者：** 必需子技能：使用 superpower-subagent-driven-development（推荐）或 superpower-executing-plans 按任务逐项执行本计划。步骤使用复选框（`- [ ]`）语法进行跟踪。

**目标：** 将现有 verbose/quiet 二元日志升级为全流程可追踪、结构化、分级、可落盘、可脱敏的日志体系，覆盖 SDK 与 CLI 全链路。

**架构：** 新增 pkg/logx 薄封装基于 stdlib slog 提供 Level/Format/File 解析与脱敏与 traceId 上下文；CLI 新增 --log-level/--log-format/--log-file 三旗标与对应 env 兼容旧 --verbose；SDK 在 request.go 统一 HTTP 生命周期打点并经 context 透传 traceId 到 auth/session/file 全链，错误按 ClassifyError 定级。

**技术栈：** Go 1.26.1、stdlib log/slog、cobra、httptest、golangci-lint/go vet；零新外部依赖

**规格：** `docs/superpowers/specs/2026-08-22-logging-enhancement-design.md`（由头脑风暴产出经 exit_plan_mode 确认的设计；本计划论证以该规格为准，执行者需同时阅读两者）

## 全局约束

- Go 版本 1.26.1；当前版本 1.3.0 维护于 internal/version/version.go
- 纯 Go 构建 CGO_ENABLED=0；不引入第三方日志库与 OTel 依赖
- 基于 stdlib log/slog；file 落盘用 os.OpenFile 追加写加 sync.Mutex 串行
- 输出双通道契约保持：stdout 为 envelope JSON，stderr 为错误 JSON 与日志；--quiet 仅静默 stderr 不静默文件
- 兼容旧 --verbose 等价 --log-level debug；默认 LevelWarn、text 格式、无文件落盘时不产生文件
- 敏感字段永不落地：token/X-Auth-Token/Authorization/password/passwd/captcha 大小写不敏感掩码；验证码原文永不落日志
- 截断策略：header 值超长星号掩码保留前后2字符；body 摘要先做 JSON key 掩码再截断默认256
- 退出码三分契约保持：0 成功 1 partial/业务 2 服务端 3 参数；日志失败不影响业务退出码
- 代码注释与 commit message 使用简体中文，遵循 Conventional Commits
- push 前必跑 6 步：go mod tidy 整洁、golangci-lint、go vet、gofmt、go test -race、integration 编译验证；文档与 CLAUDE.md 同步更新

---

### 任务 1：pkg/logx 基础包

**文件：**
- 新建：`pkg/logx/logx.go`
- 新建：`pkg/logx/redact.go`
- 测试：`pkg/logx/logx_test.go`

**接口：**
- 依赖输入：无
- 对外产出：`func ParseLevel(s string) (slog.Level, error)`、`func ParseFormat(s string) (string, error)`、`func NewLogger(level slog.Level, format string, writers ...io.Writer) *slog.Logger`、`func NewFileWriter(path string) (io.WriteCloser, error)`、`func WithTraceID(ctx context.Context, id string) context.Context`、`func TraceIDFrom(ctx context.Context) string`、`func NewTraceID() string`、脱敏 `func RedactHeader(k,v string) string`、`func RedactBody(s string) string`、`func RedactValue(key, val string) string`

- [ ] **步骤 1：编写失败的测试**

```go
package logx_test

import (
  "bytes"
  "context"
  "log/slog"
  "strings"
  "testing"

  "github.com/Wenaixi/nazhi-cli/pkg/logx"
)

func TestParseLevel(t *testing.T) {
  if got, _ := logx.ParseLevel("debug"); got != slog.LevelDebug { t.Fatalf("want debug got %v", got) }
  if got, _ := logx.ParseLevel("INFO"); got != slog.LevelInfo { t.Fatalf("want info") }
  if _, err := logx.ParseLevel("bogus"); err == nil { t.Fatal("want err") }
}

func TestParseFormat(t *testing.T) {
  if got, _ := logx.ParseFormat("json"); got != "json" { t.Fatalf("want json") }
  if got, _ := logx.ParseFormat("text"); got != "text" { t.Fatalf("want text") }
  if _, err := logx.ParseFormat("xml"); err == nil { t.Fatal("want err") }
}

func TestTraceIDContext(t *testing.T) {
  ctx := logx.WithTraceID(context.Background(), "abc123")
  if got := logx.TraceIDFrom(ctx); got != "abc123" { t.Fatalf("got %q", got) }
  if id := logx.NewTraceID(); len(id) < 8 { t.Fatalf("trace too short %q", id) }
}

func TestRedactHeaderAndBody(t *testing.T) {
  if v := logx.RedactHeader("X-Auth-Token", "eyJhbGciOiJIUzI1NiJ9.payload.signature"); !strings.Contains(v, "***") { t.Fatalf("want mask got %q", v) }
  if v := logx.RedactHeader("Content-Type", "application/json"); v != "application/json" { t.Fatalf("non-sensitive should pass") }
  body := `{"token":"secret123","name":"alice","password":"p@ss"}`
  red := logx.RedactBody(body)
  if strings.Contains(red, "secret123") || strings.Contains(red, "p@ss") { t.Fatalf("sensitive leaked %q", red) }
  if !strings.Contains(red, "***") { t.Fatalf("want mask") }
}

func TestNewLogger_JSONContainsTraceAttr(t *testing.T) {
  var buf bytes.Buffer
  lg := logx.NewLogger(slog.LevelDebug, "json", &buf)
  lg.Info("hello", slog.String("trace_id", "tid-001"))
  if !strings.Contains(buf.String(), "tid-001") { t.Fatalf("want trace in output %q", buf.String()) }
}
```

- [ ] **步骤 2：运行测试并确认其失败**

运行：`go test ./pkg/logx -run TestParseLevel -v`
预期：FAIL，提示 package logx not found / function not defined

- [ ] **步骤 3：编写最小实现**

```go
// pkg/logx/logx.go
package logx

import (
  "context"
  "crypto/rand"
  "encoding/hex"
  "fmt"
  "io"
  "log/slog"
  "os"
  "strings"
  "sync"
)

type ctxKey struct{}

func WithTraceID(ctx context.Context, id string) context.Context { return context.WithValue(ctx, ctxKey{}, id) }
func TraceIDFrom(ctx context.Context) string { v, _ := ctx.Value(ctxKey{}).(string); return v }
func NewTraceID() string { b := make([]byte, 6); _, _ = rand.Read(b); return hex.EncodeToString(b) }

func ParseLevel(s string) (slog.Level, error) {
  switch strings.ToLower(strings.TrimSpace(s)) {
  case "debug": return slog.LevelDebug, nil
  case "info": return slog.LevelInfo, nil
  case "warn", "warning": return slog.LevelWarn, nil
  case "error": return slog.LevelError, nil
  default: return slog.LevelWarn, fmt.Errorf("未知 log level %q 期望 debug/info/warn/error", s)
  }
}
func ParseFormat(s string) (string, error) {
  switch strings.ToLower(strings.TrimSpace(s)) {
  case "text", "": return "text", nil
  case "json": return "json", nil
  default: return "", fmt.Errorf("未知 log format %q 期望 text/json", s)
  }
}

type multiWriter struct { mu sync.Mutex; ws []io.Writer }
func (m *multiWriter) Write(p []byte) (int, error) { m.mu.Lock(); defer m.mu.Unlock(); for _, w := range m.ws { _, _ = w.Write(p) }; return len(p), nil }

func NewLogger(level slog.Level, format string, writers ...io.Writer) *slog.Logger {
  if len(writers)==0 { writers = []io.Writer{os.Stderr} }
  var w io.Writer
  if len(writers)==1 { w = writers[0] } else { w = &multiWriter{ws: writers} }
  opts := &slog.HandlerOptions{Level: level}
  var h slog.Handler
  if format=="json" { h = slog.NewJSONHandler(w, opts) } else { h = slog.NewTextHandler(w, opts) }
  return slog.New(h)
}
func NewFileWriter(path string) (io.WriteCloser, error) {
  // ponytail: 无轮转，文件超过 50MB 时再引入 lumberjack
  return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
}
```

```go
// pkg/logx/redact.go
package logx

import (
  "regexp"
  "strings"
)

var sensitiveKeys = map[string]bool{
  "token": true, "x-auth-token": true, "authorization": true,
  "password": true, "passwd": true, "captcha": true,
}

func isSensitiveKey(k string) bool { return sensitiveKeys[strings.ToLower(strings.TrimSpace(k))] }

func maskValue(v string) string {
  if len(v) <= 4 { return "***" }
  return v[:2] + "***" + v[len(v)-2:]
}
func RedactHeader(k, v string) string {
  if isSensitiveKey(k) { return maskValue(v) }
  // 非敏感 header 原样；敏感值已掩码
  return v
}
var kvRe = regexp.MustCompile(`(?i)"(token|x-auth-token|authorization|password|passwd|captcha)"s*:s*"[^"]*"`)
func RedactBody(s string) string {
  red := kvRe.ReplaceAllStringFunc(s, func(m string) string {
    // 将值替换为 ***
    idx := strings.Index(m, ":")
    if idx < 0 { return m }
    return m[:idx+1] + `"***"`
  })
  if len(red) > 256 { red = red[:256] + "..." }
  return red
}
func RedactValue(key, val string) string {
  if isSensitiveKey(key) { return maskValue(val) }
  return val
}
```

- [ ] **步骤 4：运行测试并确认其通过**

运行：`go test ./pkg/logx -v -count=1`
预期：PASS 全部 5 个用例

- [ ] **步骤 5：提交**

```bash
git add pkg/logx/logx.go pkg/logx/redact.go pkg/logx/logx_test.go
git commit -m "feat(logx): 新增结构化日志基础包含 Level/Format/File 与脱敏与 trace 上下文"
```

### 任务 2：Client logger 扩展与上下文贯穿

**文件：**
- 修改：`pkg/client/client.go:180-260`
- 修改：`pkg/client/request.go:260-320`
- 测试：`pkg/client/log_enhance_test.go`（新增）

**接口：**
- 依赖输入：任务 1 的 logx.NewLogger/WithTraceID/TraceIDFrom/Redact*
- 对外产出：Client 新增方法 `logInfo/logWarn/logError(logx.TraceIDFrom(ctx) 携带)`；所有现有 logDebug 调用改为携带 trace_id attr；对外保持 WithLogger 兼容，新增可选 `WithLogLevel/WithLogFormat` 或文档化直接用 logx.NewLogger 组装后 WithLogger，二选一实现时定其一

- [ ] **步骤 1：编写失败的测试**

```go
package client_test

import (
  "bytes"
  "context"
  "log/slog"
  "strings"
  "testing"

  "github.com/Wenaixi/nazhi-cli/pkg/client"
  "github.com/Wenaixi/nazhi-cli/pkg/logx"
)

func TestClientLogCarriesTraceID(t *testing.T) {
  var buf bytes.Buffer
  lg := logx.NewLogger(slog.LevelDebug, "json", &buf)
  c, _ := client.New(client.WithLogger(lg))
  ctx := logx.WithTraceID(context.Background(), "trace-xyz")
  // 通过一次 httpDo 打点验证 trace 透传：用 httptest 服
  // 这里仅验证 log helpers 含 trace_id
  c.LogDebugForTest(ctx, "hello %s", "world") // 需暴露 test helper 或直接测 request 打点
  if !strings.Contains(buf.String(), "trace-xyz") { t.Fatalf("want trace in log %q", buf.String()) }
}
func TestLogDoesNotLeakToken(t *testing.T) {
  var buf bytes.Buffer
  lg := logx.NewLogger(slog.LevelDebug, "json", &buf)
  c, _ := client.New(client.WithLogger(lg))
  ctx := logx.WithTraceID(context.Background(), "tid")
  c.LogInfoForTest(ctx, "token=%s", "supersecrettoken123")
  if strings.Contains(buf.String(), "supersecrettoken123") { t.Fatalf("token leaked %q", buf.String()) }
}
```

- [ ] **步骤 2：运行测试并确认其失败**

运行：`go test ./pkg/client -run TestClientLogCarriesTraceID -v`
预期：FAIL LogDebugForTest 未定义

- [ ] **步骤 3：编写最小实现**

在 client.go 新增：
```go
func (c *Client) logWithLevel(ctx context.Context, lvl slog.Level, format string, args ...any) {
  if c.logger == nil || !c.logger.Enabled(ctx, lvl) { return }
  tid := logx.TraceIDFrom(ctx)
  msg := fmt.Sprintf(format, args...)
  // 脱敏后再写
  msg = logx.RedactBody(msg)
  if tid != "" { c.logger.Log(ctx, lvl, msg, slog.String("trace_id", tid)) } else { c.logger.Log(ctx, lvl, msg) }
}
func (c *Client) logDebug(ctx context.Context, format string, args ...any) { c.logWithLevel(ctx, slog.LevelDebug, format, args...) }
func (c *Client) logInfo(ctx context.Context, format string, args ...any) { c.logWithLevel(ctx, slog.LevelInfo, format, args...) }
func (c *Client) logWarn(ctx context.Context, format string, args ...any) { c.logWithLevel(ctx, slog.LevelWarn, format, args...) }
func (c *Client) logError(ctx context.Context, format string, args ...any) { c.logWithLevel(ctx, slog.LevelError, format, args...) }
// 兼容旧无 ctx 调用点：重载无 ctx 版本转发为 context.Background()
```
并将现有所有 c.logDebug("...") 调用点改为 c.logDebug(ctx,"...")，对无 ctx 的调用传 context.Background()。同时暴露 test helper：LogDebugForTest 等转发到 logWithLevel。

- [ ] **步骤 4：运行测试并确认其通过**

运行：`go test ./pkg/client -run TestClientLog -v -count=1`
预期：PASS 且 buf 中含 trace_id 且 token 已掩码

- [ ] **步骤 5：提交**

```bash
git add pkg/client/client.go pkg/client/request.go pkg/client/log_enhance_test.go
git commit -m "feat(client): 日志上下文贯穿 traceId 与分级 helpers 脱敏"
```

### 任务 3：request.go 统一 HTTP 生命周期打点

**文件：**
- 修改：`pkg/client/request.go:do/httpDo/doBizGet/logRequestHeaders`
- 测试：`pkg/client/request_log_test.go`

**接口：**
- 依赖输入：任务 2 的 logWithLevel 与 logx.RedactBody/RedactHeader
- 对外产出：每次 HTTP 请求产生 start debug 与 end 定级行，attrs 含 trace_id/method/host/path/status/duration/bytes/body_snippet；错误按 ClassifyError 映射 warn/error

- [ ] **步骤 1：编写失败的测试**

```go
func TestHTTPLogLifecycleJSON(t *testing.T) {
  var buf bytes.Buffer
  lg := logx.NewLogger(slog.LevelDebug, "json", &buf)
  srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){ w.Write([]byte(`{"code":1,"msg":"ok","data":{}} `)) }))
  defer srv.Close()
  c, _ := client.New(client.WithLogger(lg), client.WithBaseURL(srv.URL))
  ctx := logx.WithTraceID(context.Background(), "tid-123")
  _, _ = c.DoBizAndDecodeForTest(ctx, "tok", "Op", "/api/x", "GET", nil)
  out := buf.String()
  if !strings.Contains(out, "tid-123") { t.Fatalf("missing trace %q", out) }
  if !strings.Contains(out, "status") { t.Fatalf("missing status %q", out) }
}
func TestLogLevelMapping(t *testing.T) {
  // 429 应为 warn，5xx 为 error，通过 handler 分别返回 429/500 断言 level 字段
}
```

- [ ] **步骤 2：运行测试并确认其失败**

运行：`go test ./pkg/client -run TestHTTPLogLifecycle -v`
预期：FAIL 未含 trace_id/status

- [ ] **步骤 3：编写最小实现**

在 request.go 的 do 方法前后加：
```go
start := time.Now()
tid := logx.TraceIDFrom(ctx)
c.logDebug(ctx, "→ %s %s trace_id=%s", method, url, tid)
resp, err := c.http.Do(req)
dur := time.Since(start)
if err != nil {
  lvl := slog.LevelError
  if isTimeoutError(err) || errors.Is(err, context.Canceled) { lvl = slog.LevelWarn }
  c.logWithLevel(ctx, lvl, "✗ %s %s dur=%s err=%v", method, url, dur, err)
  ...
}
c.logWithLevel(ctx, levelForStatus(resp.StatusCode), "← %d %s dur=%s bytes=%d body=%s", resp.StatusCode, url, dur, len(respBytes), logx.RedactBody(logSafeBody(respBytes)))
```
并定义 levelForStatus：2xx info、4xx warn（含429 warn）、5xx error。

- [ ] **步骤 4：运行测试并确认其通过**

运行：`go test ./pkg/client -run TestHTTPLog -v`
预期：PASS 且 JSON 行含 trace_id 与正确 level

- [ ] **步骤 5：提交**

```bash
git add pkg/client/request.go pkg/client/request_log_test.go
git commit -m "feat(client): 统一 HTTP 生命周期结构化打点含 trace 与分级"
```

### 任务 4：auth/session/file 全链 trace 透传与脱敏加固

**文件：**
- 修改：`pkg/client/auth.go`、`pkg/client/session.go`、`pkg/client/file.go`
- 测试：`pkg/client/auth_log_test.go` 追加脱敏断言

**接口：**
- 依赖输入：任务 2-3 的 ctx 透传
- 对外产出：Login 4步、OCR 循环、session 激活、UploadFile 均携带 trace_id；验证码原文与 password 永不落地已有测试继续通过

- [ ] **步骤 1：编写失败的测试**

```go
func TestAuthLogDoesNotLeakCaptchaAndPassword(t *testing.T) {
  var buf bytes.Buffer
  lg := logx.NewLogger(slog.LevelDebug, "json", &buf)
  // mock OCR 返回 "ABCD" 与 login 返回 200+token
  // 发起 Login 并断言 buf 不含 "ABCD" 明文与 password 明文
}
```

- [ ] **步骤 2：运行测试并确认其失败**

运行：`go test ./pkg/client -run TestAuthLogDoesNotLeak -v`
预期：FAIL 仍含明文

- [ ] **步骤 3：编写最小实现**

- auth.go：所有 c.logDebug 改为 c.logDebug(ctx,...)；OCR 成功行仅打长度不打原文已满足，再加 Redact；loginBody 序列化前对 password 脱敏
- session.go：activateSessionLocked 4步每步日志带 trace_id
- file.go：UploadFile 请求头与 body 摘要走 Redact

- [ ] **步骤 4：运行测试并确认其通过**

运行：`go test ./pkg/client -run TestAuthLog -v`
预期：PASS

- [ ] **步骤 5：提交**

```bash
git add pkg/client/auth.go pkg/client/session.go pkg/client/file.go
git commit -m "feat(client): auth/session/file 日志全链 trace 透传与脱敏加固"
```

### 任务 5：CLI flags/env 接线与落盘

**文件：**
- 修改：`cmd/nazhi/main.go`（新增 PersistentFlags 与 traceID 生成）
- 修改：`cmd/nazhi/opt_builder.go`（Level/Format/File 组装 logger）
- 修改：`cmd/nazhi/output.go`（quiet 对文件放行注释）
- 测试：`cmd/nazhi/log_flags_test.go`（新增）

**接口：**
- 依赖输入：pkg/logx
- 对外产出：flags --log-level/--log-format/--log-file；env NAZHI_LOG_LEVEL/FORMAT/FILE；--verbose 兼容为 debug；--quiet 仅静默 stderr 不静默文件；每次命令生成 traceID 注入 context 向 SDK 透传

- [ ] **步骤 1：编写失败的测试**

```go
func TestLogFlagsEnvPrecedence(t *testing.T) {
  // flag > env > default
  t.Setenv("NAZHI_LOG_LEVEL", "info")
  // 构造 cobra cmd 设 --log-level debug 断言最终 level 为 debug
}
func TestLogFileQuietStillWrites(t *testing.T) {
  // --log-file /tmp/x.log --quiet 时文件应非空 stderr 为空
}
func TestVerboseCompat(t *testing.T) {
  // --verbose 等价 --log-level debug
}
```

- [ ] **步骤 2：运行测试并确认其失败**

运行：`go test ./cmd/nazhi -run TestLogFlags -v`
预期：FAIL flags 未注册

- [ ] **步骤 3：编写最小实现**

main.go init 中：
```go
rootCmd.PersistentFlags().String("log-level", "", "日志级别 debug/info/warn/error 默认 warn 也可通过 NAZHI_LOG_LEVEL 设置")
rootCmd.PersistentFlags().String("log-format", "", "日志格式 text/json 默认 text 也可通过 NAZHI_LOG_FORMAT 设置")
rootCmd.PersistentFlags().String("log-file", "", "日志落盘路径 也可通过 NAZHI_LOG_FILE 设置")
```
opt_builder.go 中：
```go
levelStr := applyURLFlag(cmd, "log-level", "NAZHI_LOG_LEVEL")
if verbose && levelStr == "" { levelStr = "debug" } // 兼容
if levelStr == "" { levelStr = "warn" }
lvl, _ := logx.ParseLevel(levelStr)
formatStr := applyURLFlag(cmd, "log-format", "NAZHI_LOG_FORMAT")
if formatStr == "" { formatStr = "text" }
filePath := applyURLFlag(cmd, "log-file", "NAZHI_LOG_FILE")
var writers []io.Writer
if !quiet { writers = append(writers, os.Stderr) } // quiet 仅影响 stderr
if filePath != "" {
  if fw, err := logx.NewFileWriter(filePath); err == nil {
    writers = append(writers, fw)
    // 注册到 lifecycle 统一 Close
  } else {
    fmt.Fprintf(os.Stderr, "warn: 无法打开 log-file %q: %v\n", filePath, err)
    if len(writers)==0 { writers = []io.Writer{os.Stderr} }
  }
}
lg := logx.NewLogger(lvl, formatStr, writers...)
opts = append(opts, client.WithLogger(lg))
```
main.go Execute 前：
```go
traceID := logx.NewTraceID()
ctx := logx.WithTraceID(context.Background(), traceID)
// 将 ctx 透传给所有 Run 回调（通过 context.WithValue 存到 cobra 或全局）
```

- [ ] **步骤 4：运行测试并确认其通过**

运行：`go test ./cmd/nazhi -run TestLog -v`
预期：PASS

- [ ] **步骤 5：提交**

```bash
git add cmd/nazhi/main.go cmd/nazhi/opt_builder.go cmd/nazhi/output.go cmd/nazhi/log_flags_test.go
git commit -m "feat(cli): 接入 log-level/format/file 三旗标与 env 兼容 verbose 与 quiet 语义"
```

### 任务 6：端到端追踪验收与回归

**文件：**
- 测试：`test/integration/log_e2e_test.go`（build tag integration 可选）或直接在 pkg/client 中用 httptest 做 e2e
- 无新增源码文件

**接口：**
- 依赖输入：任务 1-5 全部完成
- 对外产出：一次 whoami/task 链路产生含 trace_id 的 JSON 行；quiet+file 双写验证；全量回归绿

- [ ] **步骤 1：编写失败的测试**

```go
func TestE2E_TraceAcrossSessionAndBiz(t *testing.T) {
  var buf bytes.Buffer
  lg := logx.NewLogger(slog.LevelDebug, "json", &buf)
  srv := httptest.NewServer(...)
  c, _ := client.New(client.WithLogger(lg), client.WithBaseURL(srv.URL))
  ctx := logx.WithTraceID(context.Background(), "e2e-tid")
  _, _ = c.ActivateSession(ctx, "tok")
  _, _ = c.GetMyInfo(ctx, "tok") // 或 FetchTasks
  // 断言 buf 中至少 4 行含 e2e-tid 且含 method/status/duration
}
```

- [ ] **步骤 2：运行测试并确认其失败**

运行：`go test ./pkg/client -run TestE2E_Trace -v`
预期：FAIL 缺 trace 行

- [ ] **步骤 3：编写最小实现**

补齐遗漏的 ctx 透传点（若任务 2-4 有漏则在此补）

- [ ] **步骤 4：运行测试并确认其通过**

运行：`go test -race ./... -count=1` 与 `golangci-lint run --timeout=5m ./...` 与 `go vet ./...` 与 `gofmt -l .`
预期：全部 PASS/无输出

- [ ] **步骤 5：提交**

```bash
git add -A
git commit -m "test: 端到端追踪验收与全量回归绿"
```

### 任务 7：文档与版本同步

**文件：**
- 修改：`docs/cli/README.md`
- 修改：`docs/sdk/README.md`
- 修改：`CLAUDE.md`
- 修改：`README.md`
- 修改：`CHANGELOG.md`

**接口：**
- 依赖输入：任务 5 的 flags/env 契约
- 对外产出：所有文档一致描述新日志体系

- [ ] **步骤 1：编写失败的测试**（文档规则测试）

```go
// test/docrules/log_doc_test.go
func TestLogDocsMentionNewFlags(t *testing.T) {
  // 断言 docs/cli/README.md 含 --log-level/--log-format/--log-file
  // 断言 CLAUDE.md 环境变量表含 NAZHI_LOG_LEVEL/FORMAT/FILE
  // 断言 CHANGELOG.md Unreleased 含 Added 日志条目
}
```

- [ ] **步骤 2：运行测试并确认其失败**

运行：`go test ./test/docrules -run TestLogDocs -v`
预期：FAIL 未提及新 flags

- [ ] **步骤 3：编写最小实现**

- docs/cli/README.md 新增章节：全局日志选项表格 log-level/format/file/verbose/quiet 语义与示例；加粗 quiet 不影响文件落盘
- docs/sdk/README.md 新增 WithLogger/logx 用法与 traceId 上下文示例
- CLAUDE.md 环境变量表新增三行 NAZHI_LOG_LEVEL/FORMAT/FILE；输出通道例外追加 log-file 条目
- README.md 同步环境变量与快速开始示例
- CHANGELOG.md Unreleased 新增 Added 小节描述日志增强与脱敏与追踪

- [ ] **步骤 4：运行测试并确认其通过**

运行：`go test ./test/docrules -run TestLogDocs -v`
预期：PASS

- [ ] **步骤 5：提交**

```bash
git add docs/cli/README.md docs/sdk/README.md CLAUDE.md README.md CHANGELOG.md
git commit -m "docs: 同步日志增强文档与环境变量与输出通道说明"
```
