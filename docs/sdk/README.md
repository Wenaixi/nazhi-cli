# SDK 参考

Go SDK：`pkg/client` + `pkg/types` + `pkg/tokenparse`。当前文档按**功能域分册**；总览只保留安装、构造、索引与错误处理。

| 包 | 作用 |
|----|------|
| `pkg/client` | Client、业务方法、Option、哨兵错误 |
| `pkg/types` | 请求/响应领域类型、统一响应解码 |
| `pkg/tokenparse` | SSO token 从 Location / ReturnData 提取 |

## 分册目录

| 域 | 文档 |
|----|------|
| 认证登录 | [auth.md](./auth.md) |
| Session | [session.md](./session.md) |
| 用户 | [user.md](./user.md) |
| 任务 / 写实提交 | [task.md](./task.md) |
| 写实列表 | [circle-list.md](./circle-list.md) |
| 写实互动与元数据 | [circle-action.md](./circle-action.md) |
| 荣誉 | [honor.md](./honor.md) |
| 典型案例 | [typical-case.md](./typical-case.md) |
| 自我评价 | [self-eval.md](./self-eval.md) |
| 文件 | [file.md](./file.md) |
| 原始 JSON 透传 | [raw-json.md](./raw-json.md) |
| **自动补全总表** | [autofill.md](./autofill.md) |

CLI 见 [../cli/README.md](../cli/README.md)。  
**学号补学校、任务元数据、荣誉 typeName、典型案例 *Name 等**一律以 [autofill.md](./autofill.md) 为准。

---

## 安装

```bash
go get github.com/Wenaixi/nazhi-cli/pkg/client
go get github.com/Wenaixi/nazhi-cli/pkg/types
```

Go 版本见仓库 `go.mod`（当前 1.26.1）。SDK 不内置本地验证码识别器，`Login` 必须通过 `WithCustomOCR` 注入视觉识别器；CLI 使用 `NAZHI_SILICONFLOW_API_KEY` 接入 Nazhi-auto 同款硅基流动 Qwen3-Omni。

---

## 快速开始

```go
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

func main() {
	// 实现 CaptchaRecognizer 的视觉模型或远程识别器由调用方提供。
	recognizer := newMyCaptchaRecognizer()
	c, err := client.New(
		client.WithSSOBase("https://www.nazhisoft.com"),
		client.WithBaseURL("http://139.159.205.146:8280"),
		client.WithUploadURL("http://doc.nazhisoft.com"),
		client.WithTimeout(30*time.Second),
		client.WithCustomOCR(recognizer),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx := context.Background()
	resp, err := c.Login(ctx, types.LoginRequest{
		Username: os.Getenv("NAZHI_USERNAME"),
		Password: os.Getenv("NAZHI_PASSWORD"),
	})
	if err != nil {
		log.Fatal(err)
	}
	if _, err := c.ActivateSession(ctx, resp.Token); err != nil {
		log.Fatal(err)
	}
	info, err := c.GetMyInfo(ctx, resp.Token)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("hello %s", info.Name)
}
```

---

## Client 构造与常用 Option

```go
c, err := client.New(
	client.WithSSOBase("https://www.nazhisoft.com"),
	client.WithBaseURL("http://139.159.205.146:8280"),
	client.WithUploadURL("http://doc.nazhisoft.com"),
	client.WithTimeout(30*time.Second),
	client.WithCustomOCR(myOCR), // Login 必需
)
```

| Option | 作用 |
|--------|------|
| `WithSSOBase` | SSO 根地址 |
| `WithBaseURL` | 业务 API 根地址 |
| `WithUploadURL` | 文件上传根地址 |
| `WithTimeout` | HTTP 超时 |
| `WithCustomOCR` | 注入视觉模型或其它验证码识别器（Login 必填） |
| `WithLogger` | 注入 `log/slog` 风格日志；推荐用 `pkg/logx.NewLogger(level, format, writers...)` 组装 |
| `WithSessionBackoff` | Session 失败冷却 |

构造失败常见原因：cookie jar 初始化失败。用完 `defer c.Close()` 释放视觉识别器和 HTTP 资源。

### 结构化日志与全链追踪（pkg/logx）

```go
import (
  "context"
  "log/slog"
  "github.com/Wenaixi/nazhi-cli/pkg/logx"
)
// 按级别/格式/落盘组装 logger
lg := logx.NewLogger(slog.LevelDebug, "json", os.Stderr)
// 或同时落盘：lg := logx.NewLogger(slog.LevelDebug, "json", os.Stderr, fw) fw, _ := logx.NewFileWriter("/tmp/nazhi.log")
c, _ := client.New(client.WithLogger(lg))
// 每次业务调用携带 trace_id 串联全链
ctx := logx.WithTraceID(context.Background(), logx.NewTraceID())
_, _ = c.ActivateSession(ctx, token)
_, _ = c.GetMyInfo(ctx, token)
// 日志每行含 trace_id；敏感字段（token/password/captcha 与 Referer token=）自动脱敏
```

日志级别：`debug/info/warn/error`（默认 `warn`）；格式：`text/json`。CLI 通过 `--log-level/--log-format/--log-file`（或 `NAZHI_LOG_LEVEL/FORMAT/FILE`）透传到 SDK。


---

## 方法签名速查

| 方法 | 域文档 |
|------|--------|
| Login / InitSession / GetSchoolID | [auth](./auth.md) |
| ActivateSession | [session](./session.md) |
| GetMyInfo / UpdateMyInfo* / InvalidateCachedUserInfo | [user](./user.md) |
| FetchTasks / SubmitTask / EditCircle / GetCircleTypeByTaskID / GetDimensions | [task](./task.md) |
| Get*Circles / Peek* | [circle-list](./circle-list.md) |
| DeleteCircle / AddCircleComment / SetCircleLike / GetCircle* / GetDictList | [circle-action](./circle-action.md) |
| GetHonor* / AddHonor / UpdateHonor / DeleteHonor | [honor](./honor.md) |
| Add/Get/Update/Delete*TypicalCase | [typical-case](./typical-case.md) |
| Submit/Query SelfEval* | [self-eval](./self-eval.md) |
| UploadFile / DownloadFile | [file](./file.md) |
| *JSON 族 | [raw-json](./raw-json.md) |

---

## CLI 输出 vs SDK 返回值

| | CLI | SDK 结构化 |
|--|-----|------------|
| 外壳 | envelope：status/code/message/data | 直接返回 Go 值 |
| 列表透传 | 常用 `*JSON`，data 为平台原始 JSON | `[]T` / 结构体 |
| 错误 | stderr JSON + 退出码 0/1/2/3 | `error` + 哨兵 |

---

## 输入暴露原则（写入口）

1. 前端用户 v-model / 手选 → 公开 Input  
2. 前端/SDK 能自动填的 → 调用方可不填（任务元数据、typeName、*Name、score=0、学校信息用学号补全等）  
3. **禁止发明默认**：写实空 Address/OrgName/Level 原样发送（不填学校名、不默认 `"5"`）  
4. Hours：任务预设 >0 可空；≤0 须用户填  
5. 写实 CLI payload 对 `hours` 兼容可为小数的 number 与 string；`level`、`checkResult`、`playRole` 的未加引号 number 仅接受有限整数，并将 `1.0`、`1e0` 等合法整数规范为标准十进制代码字符串，string 按原值保留；小数、非有限值和溢出值会被拒绝。CLI 通过 `cmd/nazhi` 私有 JSON helper 解码后，SDK 按提交字段语义处理；真实前端的 `circleTaskId` / `pictureList` 分别归一为 `taskId` / `imageIDs`。
6. 用户资料 `className` 后处理遵循前端规则，仅移除首个“级”字，不按 `gradeName` 删除前缀；无“级”字时原样保留。

**完整对照表（含「学号 → 自动补 schoolId/schoolName」）见 [autofill.md](./autofill.md)。**  
各域文档内另有「用户输入 vs SDK 自动」小节。

---

## 错误处理

```go
if err != nil {
	switch {
	case errors.Is(err, client.ErrLoginRejected):
		// 重新登录
	case errors.Is(err, client.ErrOCRNotConfigured):
		// 配置视觉模型或通过 WithCustomOCR 注入
	case errors.Is(err, client.ErrSessionBackoff):
		// 等待冷却
	case errors.Is(err, client.ErrInvalidPayload):
		// 检查必填字段
	case errors.Is(err, client.ErrBusinessRejected):
		// 展示业务 msg
	case errors.Is(err, client.ErrNetwork), errors.Is(err, client.ErrTimeout):
		// 重试
	default:
		log.Print(err)
	}
}
```

| 哨兵 | 含义 |
|------|------|
| `ErrLoginRejected` | 登录被拒 |
| `ErrNetwork` | 网络错误 |
| `ErrBusinessRejected` | 业务 code≠1 |
| `ErrEmptyUserInfo` | 无用户数据 |
| `ErrSessionBackoff` | Session 冷却 |
| `ErrUploadRejected` / `ErrFileTooLarge` | 上传失败/过大 |
| `ErrInvalidPayload` | 入参非法 |
| `ErrOCRNotConfigured` / `ErrOCRPanic` | 未注入识别器 / 识别器 panic |
| `ErrRateLimited` / `ErrServiceUnavailable` / `ErrTimeout` | HTTP |
| `ErrInvalidResponse` | 异常 4xx |
| `ErrRetryable` | 可重试（如 cancel） |

也可用 `client.ClassifyError(err)` 做粗分类。

---

## tokenparse

```go
import "github.com/Wenaixi/nazhi-cli/pkg/tokenparse"

tok, err := tokenparse.ExtractFromLocation(locationHeader)
tok, err = tokenparse.ExtractFromReturnData(bodyBytes)
```

---

## 资源释放

```go
defer c.Close() // 注入的识别器、HTTP 客户端等
```

---

## 版本

文档对齐仓库 **v1.3.0** 主线及后续 Unreleased 输入暴露修复。字段与行为以源码为准。  