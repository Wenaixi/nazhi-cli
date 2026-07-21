# SDK 参考（pkg/client / pkg/types / pkg/tokenparse）

nazhi-cli 的 Go SDK 完整开放为三个公开包，可以被任何 Go 项目 `go get` 后直接调用。

| 包 | 作用 | 文档入口 |
|---|---|---|
| [`pkg/client`](https://github.com/Wenaixi/nazhi-cli/tree/main/pkg/client) | 核心 SDK：Client 构造 + 公开方法 + Option + 哨兵错误 | 本文 |
| [`pkg/types`](https://github.com/Wenaixi/nazhi-cli/tree/main/pkg/types) | 领域类型（请求/响应/任务/用户等）+ 统一响应泛型解码 | [types.go](https://github.com/Wenaixi/nazhi-cli/blob/main/pkg/types/types.go) |
| [`pkg/tokenparse`](https://github.com/Wenaixi/nazhi-cli/tree/main/pkg/tokenparse) | SSO token 从 302 Location 头 / ReturnData JSON 字节提取 | [tokenparse.go](https://github.com/Wenaixi/nazhi-cli/blob/main/pkg/tokenparse/tokenparse.go) |

---

## 目录

- [安装](#安装)
- [快速开始](#快速开始)
- [Client 构造与 Option 模式](#client-构造与-option-模式)
- [SDK 方法签名速查](#sdk-方法签名速查)
- [CLI 输出 vs SDK 返回值](#cli-输出-vs-sdk-返回值)
- [认证域（auth.go）](#认证域authgo)
- [Session 域（session.go）](#session-域sessiongo)
- [用户域（user.go）](#用户域usergo)
- [任务域（task.go）](#任务域taskgo)
- [自我评价域（self_eval.go）](#自我评价域self_evalgo)
- [已提交写实记录域（submitted.go）](#已提交写实记录域submittedgo)
- [荣誉申报域（honor.go）](#荣誉申报域honorgo)
- [文件域（file.go）](#文件域filego)
- [资源释放（Close）](#资源释放close)
- [错误处理](#错误处理)
- [pkg/tokenparse 单独使用](#pkgtokenparse-单独使用)
- [pkg/types 类型索引](#pkgtypes-类型索引)
- [pkg/types/response.go 泛型辅助](#pkgtypesresponsego-泛型辅助)

---

## 安装

```bash
go get github.com/Wenaixi/nazhi-cli/pkg/client
go get github.com/Wenaixi/nazhi-cli/pkg/types
go get github.com/Wenaixi/nazhi-cli/pkg/tokenparse
```

Go 版本要求见仓库 `go.mod`：当前 1.26.1。

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
	c, err := client.New(
		client.WithSSOBase("https://www.nazhisoft.com"),
		client.WithBaseURL("http://139.159.205.146:8280"),
		client.WithUploadURL("http://doc.nazhisoft.com"),
		client.WithTimeout(30*time.Second),
	)
	if err != nil {
		log.Fatalf("Client 初始化失败：%v", err)
	}
	defer c.Close()

	ctx := context.Background()

	resp, err := c.Login(ctx, types.LoginRequest{
		Username: os.Getenv("NAZHI_USERNAME"),
		Password: os.Getenv("NAZHI_PASSWORD"),
	})
	if err != nil {
		log.Fatalf("登录失败：%v", err)
	}
	token := resp.Token

	info, err := c.ActivateSession(ctx, token)
	if err != nil {
		log.Fatalf("激活 Session 失败：%v", err)
	}
	log.Printf("已登录：%s，%s", info.Name, info.ClassName)

	tasks, err := c.FetchTasks(ctx, token)
	if err != nil {
		log.Printf("获取任务失败（部分成功 %d 个）：%v", len(tasks), err)
	}
	log.Printf("共 %d 个任务", len(tasks))
}
```

---

## Client 构造与 Option 模式

```go
func New(opts ...Option) (*Client, error)
```

构造函数返回 `(*Client, error)`——`error` 在 `syncCookieToken` 失败时返回（典型场景：用 `WithHTTPClient` 传入的自定义 `*http.Client` 未配备 cookie jar）。默认配置下 `err` 始终为 `nil`。

### 全部 Option

| Option | 类型 | 默认 | 行为约定 / 陷阱 |
|---|---|---|---|
| `WithSSOBase(url)` | `string` | `https://www.nazhisoft.com` | 空字符串被拒绝（warn）并保留当前值；非空则赋值 |
| `WithBaseURL(url)` | `string` | `http://139.159.205.146:8280` | 同上 |
| `WithUploadURL(url)` | `string` | `http://doc.nazhisoft.com` | 同上 |
| `WithTimeout(d)` | `time.Duration` | `15s` | `d<=0` 拒绝 |
| `WithHTTPClient(hc)` | `*http.Client` | 默认自带 cookie jar 的客户端 | `nil` 拒绝；替换后由调用方负责 Jar 兼容性 |
| `WithLogger(l)` | `*slog.Logger` | stderr WARN 级别 | `nil` 拒绝 |
| `WithToken(t)` | `string` | 无 | 同时设置 Header + Cookie；空字符串拒绝 |
| `WithCustomOCR(r)` | `CaptchaRecognizer` | `ocr.NewPool(0)`（`-tags ddddocr`）/ `nil`（`!ddddocr`） | `nil` 拒绝 |
| `WithOCRConcurrency(n)` | `int` | `min(4, NumCPU)` | `n<=0` 拒绝 |
| `WithSessionBackoff(d)` | `time.Duration` | `5s` | `d<=0` 拒绝 |
| `WithSubmittedPageSize(n)` | `int` | `100` | `n<=0` 拒绝；服务端上限 500 |

---

## SDK 方法签名速查

| 方法 | 签名 | 返回 |
|---|---|---|
| `InitSession` | `(ctx) error` | — |
| `GetSchoolID` | `(ctx, username) (*SchoolInfo, error)` | `schoolID` + `schoolName` |
| `Login` | `(ctx, req) (*LoginResponse, error)` | Token / ExpiresAt |
| `ActivateSession` | `(ctx, token) (*UserInfo, error)` | 13 字段用户资料 |
| `GetMyInfo` | `(ctx, token) (*UserInfo, error)` | 13 字段用户资料 |
| `FetchTasks` | `(ctx, token) ([]Task, error)` | 21 字段任务条目 |
| `SubmitTask` | `(ctx, token, input) (*TaskResult, error)` | Code / Msg |
| `GetDimensions` | `(ctx, token) ([]Dimension, error)` | ID / Name |
| `SubmitSelfEvaluation` | `(ctx, token, comment) error` | — |
| `QuerySelfEvaluation` | `(ctx, token) (*SelfEvalStatus, error)` | ID + 双向评语 |
| `QuerySelfGradEvaluation` | `(ctx, token) (*map[string]any, error)` | 泛型 map |
| `GetSubmittedCircles` | `(ctx, token, key) ([]CircleRecord, error)` | 原始 JSON 写实记录（含姓名学号）；key 可空 |
| `GetSubmittedCirclesLimitJSON` | `(ctx, token, offset, limit, key) (json.RawMessage, *PageBean, error)` | 原始 JSON 数组 + 分页信息；key 可空 |
| `PeekSubmittedTotal` | `(ctx, token, key) (int, error)` | 总记录数（轻量，只拉 1 条）；key 可空 |
| `GetTeacherCircles` | `(ctx, token, key) ([]CircleRecord, error)` | 教师代写写实记录；key 可空 |
| `GetTeacherCirclesLimitJSON` | `(ctx, token, offset, limit, key) (json.RawMessage, *PageBean, error)` | 教师代写原始 JSON 数组 + 分页；key 可空 |
| `PeekTeacherTotal` | `(ctx, token, key) (int, error)` | 教师代写记录总数；key 可空 |
| `GetWithdrawnCircles` | `(ctx, token, key) ([]CircleRecord, error)` | 被撤回写实记录；key 可空 |
| `GetWithdrawnCirclesLimitJSON` | `(ctx, token, offset, limit, key) (json.RawMessage, *PageBean, error)` | 被撤回原始 JSON 数组 + 分页；key 可空 |
| `PeekWithdrawnTotal` | `(ctx, token, key) (int, error)` | 被撤回记录总数；key 可空 |
| `GetPublicCircles` | `(ctx, token, key) ([]CircleRecord, error)` | 公示写实记录（全班）；key 可空 |
| `GetPublicCirclesLimitJSON` | `(ctx, token, offset, limit, key) (json.RawMessage, *PageBean, error)` | 公示原始 JSON 数组 + 分页；key 可空 |
| `PeekPublicTotal` | `(ctx, token, key) (int, error)` | 公示记录总数；key 可空 |
| `EditCircle` | `(ctx, token, input) (*TaskResult, error)` | 修改已提交的写实记录 |
| `GetHonorTypes` | `(ctx, token) ([]HonorType, error)` | 5 字段荣誉类型 |
| `GetHonorTypeForSelect` | `(ctx, token) ([]HonorSelectOption, error)` | Label / Value |
| `GetHonorLevel` | `(ctx, token, honorTypeID) ([]HonorSelectOption, error)` | Label / Value |
| `GetHonorList` | `(ctx, token, pageNo, pageSize, key) (*HonorListResult, error)` | `records` + `page`；key 可空 |
| `AddHonor` | `(ctx, token, payload) error` | — |
| `DeleteHonor` | `(ctx, token, honorID) error` | — |
| `AddTypicalCase` | `(ctx, token, payload) error` | — |
| `GetTypicalCaseList` | `(ctx, token, pageNo, pageSize, status...int) (*types.TypicalCaseListResult, error)` | `records` + `page`；status 默认 3=全部 |
| `GetTypicalCaseListJSON` | `(ctx, token, pageNo, pageSize, status...int) (json.RawMessage, error)` | 原始 JSON `{records, page}` |
| `UploadFile` | `(ctx, filePath) (*UploadFileResult, error)` | `attachmentID` |
| `DownloadFile` | `(ctx, attachmentID, dst) error` | — |
| `DeleteCircle` | `(ctx, token, circleID) error` | — |
| `AddCircleComment` | `(ctx, token, circleID, content) error` | — |
| `SetCircleLike` | `(ctx, token, circleID) error` | — |
| `GetCircleImages` | `(ctx, token, pageNo, pageSize) ([]CircleImage, *PageBean, error)` | 图片列表+分页 |
| `GetCircleTasks` | `(ctx, token, typeID) ([]Task, error)` | 任务列表 |
| `GetCircleTypes` | `(ctx, token, dimensionID) ([]Dimension, error)` | 类别列表 |
| `GetDimensionsBySchool` | `(ctx, token) ([]Dimension, error)` | 学校维度 |
| `GetDictList` | `(ctx, token, cateCode) ([]HonorSelectOption, error)` | 字典选项 |
| `UpdateHonor` | `(ctx, token, payload) error` | — |
| `UpdateMyInfo` | `(ctx, token, updates map) error` | 成功后失效 cachedUserInfo |
| `UpdateMyInfoStructured` | `(ctx, token, UserUpdateInput) error` | 友好键 remap + 清缓存 |
| `InvalidateCachedUserInfo` | `()` | 主动清空 GetMyInfo 缓存 |

---

## CLI 输出 vs SDK 返回值

本节专门说明：CLI `envelope.data` 和 SDK 返回值之间是否严格 1:1。

| 命令 | SDK 方法 | 是否 1:1 | 说明 |
|---|---|---|---|
| `nazhi whoami` | `GetMyInfoJSON` | 是 | CLI 直接透传 SDK 原始 JSON |
| `nazhi session activate` | `ActivateSessionJSON` | 是 | CLI 直接透传 SDK 原始 JSON |
| `nazhi task list` | `FetchTasks` | 否 | CLI 输出 SDK 业务模型的 `Task[]`（含 submitted/needPic） |
| `nazhi task submitted` / `task done` | `GetSubmittedCirclesJSON` / `GetSubmittedCirclesLimitJSON` | 是 | CLI 直接透传 SDK 原始 JSON 数组；支持 `--key`；`--limit/--offset/--count` 时输出 `{records,total}` |
| `nazhi task teacher` | `GetTeacherCirclesJSON` / `GetTeacherCirclesLimitJSON` | 是 | CLI 直接透传 SDK 原始 JSON 数组；支持 `--key`；`--limit/--offset/--count` 时输出 `{records,total}` |
| `nazhi task withdrawn` | `GetWithdrawnCirclesJSON` / `GetWithdrawnCirclesLimitJSON` | 是 | CLI 直接透传 SDK 原始 JSON 数组；支持 `--key`；`--limit/--offset/--count` 时输出 `{records,total}` |
| `nazhi task public` | `GetPublicCirclesJSON` / `GetPublicCirclesLimitJSON` | 是 | CLI 直接透传 SDK 原始 JSON 数组；支持 `--key`；`--limit/--offset/--count` 时输出 `{records,total}` |
| `nazhi task edit` | `EditCircle` | 否 | SDK 成功返回 `(*TaskResult, error)`，CLI 用 envelope.Success 表达成功 |
| `nazhi self-eval status` | `QuerySelfEvaluationJSON` | 是 | CLI 直接透传 SDK 原始 JSON |
| `nazhi honor types` | `GetHonorTypesJSON` | 是 | CLI 直接透传 SDK 原始 JSON 数组 |
| `nazhi honor list` | `GetHonorListJSON` | 是 | CLI 直接透传 SDK 拼装的 `{records,page}` JSON；支持 `--key` |
| `nazhi file upload` | `UploadFile` | 是 | CLI 直接输出 SDK 返回对象 `{attachmentID}` |
| `nazhi self-eval submit` | `SubmitSelfEvaluation` | 否 | SDK 成功返回 `nil`，CLI 用空 envelope 表达成功 |
| `nazhi honor add` | `AddHonor` | 否 | SDK 成功返回 `nil`，CLI 用空 envelope 表达成功 |
| `nazhi honor add` | `AddHonor` | 否 | SDK 成功返回 `nil`，CLI 用空 envelope 表达成功 |
| `nazhi honor delete` | `DeleteHonor` | 否 | SDK 成功返回 `nil`，CLI 用空 envelope 表达成功 |
| `nazhi typical-case submit` | `AddTypicalCase` | 否 | SDK 成功返回 `nil`，CLI 用空 envelope 表达成功 |
| `nazhi typical-case list` | `GetTypicalCaseListJSON` | 是 | CLI 直接透传 SDK 拼装的 `{records,page}` JSON |
| `nazhi file download` | `DownloadFile` | 否 | SDK 成功返回 `nil`，CLI 用空 envelope 表达成功 |

---

## 认证域（auth.go）

### `InitSession(ctx context.Context) error`

访问 SSO 登录页，建立 JSESSIONID Cookie。`Login()` 内部已调用，一般无需手动调用。

### `GetSchoolID(ctx context.Context, username string) (*types.SchoolInfo, error)`

学号查学校 ID 和名称。这是一个公开 API，无需登录即可使用。

请求示例：

```go
info, err := c.GetSchoolID(ctx, "G350181200912110035")
if err != nil {
	if errors.Is(err, client.ErrInvalidPayload) {
		log.Fatal("school_id 字段缺失或非数字")
	}
	log.Fatalf("查学校 ID 失败：%v", err)
}
log.Printf("学校 ID：%s，名称：%s", info.SchoolID, info.SchoolName)
```

SDK 响应示例：

```json
{
  "schoolID": "173",
  "schoolName": "示例中学"
}
```

真实平台原始响应（脱敏）：

```json
{
  "code": 1,
  "msg": "成功",
  "returnData": null,
  "note": null,
  "pageBean": null,
  "dataList": [
    {
      "school_id": 173,
      "student_number": "G350181200912110035",
      "NAME": "示例中学"
    }
  ],
  "dataMap": null,
  "dataInt": 0,
  "dataString": null,
  "insertID": 0,
  "updateCount": 0,
  "isAttendance": 0
}
```

### `Login(ctx context.Context, req types.LoginRequest) (*types.LoginResponse, error)`

完整 SSO 登录，自动处理 OCR 验证码。

请求示例：

```go
resp, err := c.Login(ctx, types.LoginRequest{
	Username: os.Getenv("NAZHI_USERNAME"),
	Password: os.Getenv("NAZHI_PASSWORD"),
})
if err != nil {
	if errors.Is(err, client.ErrLoginRejected) {
		log.Fatal("学号/密码/验证码错误")
	}
	if errors.Is(err, client.ErrOCRNotConfigured) {
		log.Fatal("OCR 未配置，请用预编译 release 或注入自定义识别器")
	}
	log.Fatalf("登录失败：%v", err)
}
token := resp.Token
```

SDK 响应示例：

```json
{
  "token": "eyJhbGciOiJIUzUxMiJ9.eyJzdWIiOiJHMzUwMTgxMjAwOTEyMTEwMDM1I...",
  "expiresAt": "2026-07-25T16:36:01+08:00",
}
```

---

## Session 域（session.go）

### `ActivateSession(ctx context.Context, token string) (*types.UserInfo, error)`

激活业务 Session。必须按以下 4 步顺序执行（HAR 抓包验证）：

```
1. GET /                                    （建后端 Session）
2. GET /api/studentInfo/getMenu             （Referer: /homepage?token=xxx）
3. GET /api/studentInfo/getMenu             （Referer: /home）
4. GET /api/studentInfo/getMyInfo           （返回完整 UserInfo）
```

请求示例：

```go
info, err := c.ActivateSession(ctx, token)
if err != nil {
	if errors.Is(err, client.ErrSessionBackoff) {
		log.Println("Session 激活冷却中，请稍后重试")
		return
	}
	if errors.Is(err, client.ErrEmptyUserInfo) {
		log.Println("业务成功但无用户数据")
		return
	}
	log.Fatalf("激活 Session 失败：%v", err)
}
log.Printf("欢迎 %s，%s", info.Name, info.ClassName)
```

SDK 响应示例：

```json
{
  "id": 327053,
  "name": "张同学",
  "studentNumber": "G350***********",
  "studentId": 387020,
  "studyNumber": "250801****",
  "nationalStudentNumber": "G350***********",
  "schoolId": 173,
  "schoolName": "示例中学",
  "gradeId": 27900,
  "gradeName": "高一",
  "classId": 162647,
  "className": "某班",
  "seat": 29
}
```

---

## 用户域（user.go）

### `GetMyInfo(ctx context.Context, token string) (*types.UserInfo, error)`

获取用户资料。会触发 ActivateSession（复用其步骤 4 缓存），同 token 第一次调用做 4 步激活，第二次调用纯缓存零 HTTP。

`UpdateMyInfo` / `UpdateMyInfoStructured` 成功后会自动 `InvalidateCachedUserInfo()`，下次 `GetMyInfo` 会重新拉取而非返回更新前快照。

请求示例：

```go
info, err := c.GetMyInfo(ctx, token)
if err != nil {
	if errors.Is(err, client.ErrEmptyUserInfo) {
		log.Println("业务成功但暂无数据")
		return
	}
	log.Fatalf("获取用户信息失败：%v", err)
}
log.Printf("欢迎 %s，%s", info.Name, info.ClassName)
```

SDK 响应示例：

```json
{
  "id": 327053,
  "name": "张同学",
  "studentNumber": "G350***********",
  "studentId": 387020,
  "studyNumber": "250801****",
  "nationalStudentNumber": "G350***********",
  "schoolId": 173,
  "schoolName": "示例中学",
  "gradeId": 27900,
  "gradeName": "高一",
  "classId": 162647,
  "className": "某班",
  "seat": 29
}
```

### `UpdateMyInfo(ctx context.Context, token string, updates map[string]any) error`

POST `/api/studentInfo/updateMyInfo`。`updates` 只传需要修改的字段（API 原始 key，如 `telephone`、`gender`）。成功后失效 session 缓存的 UserInfo。

### `UpdateMyInfoStructured(ctx context.Context, token string, input types.UserUpdateInput) error`

面向用户的友好字段更新：`GenderName="男"` → `gender=1`，`YouthLeague` / `NationName` / `IdCardType` 同理。零值/空串跳过。内部调 `UpdateMyInfo`，故同样清缓存。

### `InvalidateCachedUserInfo()`

主动清空 `ActivateSession` 步骤 4 缓存的 UserInfo。绕过 `UpdateMyInfo` 改服务端资料后可调用。

---

## 任务域（task.go）

### `FetchTasks(ctx context.Context, token string) ([]types.Task, error)`

拉全部维度的任务。流程：ActivateSession → getDimensions → 遍历维度并发拉 getCircleStatistics → 聚合。

请求示例：

```go
tasks, err := c.FetchTasks(ctx, token)
if err != nil {
	log.Printf("获取任务失败（部分成功 %d 个）：%v", len(tasks), err)
}
for _, t := range tasks {
	fmt.Printf("任务：%s（%s，%.1f 学时）\n", t.Name, t.DimensionName, t.Hours)
}
```

SDK 响应示例（2 条）：

```json
[
  {
    "id": 18160,
    "name": "2026年\"青春唱响逐新章，美育涵养润芳华\"班班有歌声",
    "typeName": "参加的艺术活动项目",
    "dimensionName": "艺术素养",
    "hours": 4,
    "score": 1,
    "remark": "2026年\"青春唱响逐新章，美育涵养润芳华\"班班有歌声4个小时",
    "submitted": false,
    "needPic": true,
    "startDateStr": "2026-06-30T00:00:00+08:00",
    "endDateStr": "2026-07-30T00:00:00+08:00",
    "auditStartDateStr": "2026-07-31T00:00:00+08:00",
    "auditEndDateStr": "2026-09-30T00:00:00+08:00",
    "creatorName": "管理员",
    "roleName": "班主任",
    "creationTime": [2026, 6, 30, 11, 39, 19],
    "creationTimeStr": "2026-06-30T00:00:00+08:00",
    "termId": 18,
    "pushNum": 1,
    "scopeType": 2,
    "scopeTypeName": "年段任务"
  },
  {
    "id": 18162,
    "name": "2025-2026第二学期调查表8小时",
    "typeName": "社会调查",
    "dimensionName": "社会实践",
    "hours": 8,
    "score": 1,
    "remark": "2025-2026第二学期调查表8小时",
    "submitted": false,
    "needPic": false,
    "startDateStr": "2026-06-30T00:00:00+08:00",
    "endDateStr": "2026-07-30T00:00:00+08:00",
    "auditStartDateStr": "2026-07-31T00:00:00+08:00",
    "auditEndDateStr": "2026-09-30T00:00:00+08:00",
    "creatorName": "管理员",
    "roleName": "班主任",
    "creationTime": [2026, 6, 30, 11, 45, 22],
    "creationTimeStr": "2026-06-30T00:00:00+08:00",
    "termId": 18,
    "pushNum": 1,
    "scopeType": 2,
    "scopeTypeName": "年段任务"
  }
]
```

### `SubmitTask(ctx context.Context, token string, input types.TaskSubmitInput) (*types.TaskResult, error)`

提交任务。公开输入只保留最少必要字段，SDK 内部自动完成：

1. `getCircleTypeByTaskId(taskId)` 获取 `circleTypeId / dimensionId / hours`（任务预设）
2. 合并 `imageIds` 与 `UploadFile()` 上传 `imagePaths` 得到的附件 ID，组装成 `pictureList`
3. 组装完整 `addCircle` 请求体并提交

**学时 `Hours`（对齐前端 `hoursStatus`）**：

| 任务元数据 hours | 用户 `Hours` | 行为 |
|-----------------|-------------|------|
| `> 0` | 空 | SDK 用任务预设（前端只读自动填） |
| `≤ 0` | 空 | `ErrInvalidPayload`（须手填） |
| 任意 | 非空合法数 | 用用户值 |
| 任意 | 非空非法 | `ErrInvalidPayload` |

**Address / OrgName / Level**：与前端一致——用户填什么发什么；**空串原样**，不再自动填学校名或默认 `"5"`。部分活动类型前端 `checkData` 会要求非空，调用方须按任务类型自行填写。

**CircleDate / TermName**：前端 form 有键但**无 v-model**，用户从不手填。结构体仍保留以兼容旧调用方，推荐保持空串，不要当作用户必填字段。

请求示例：

```go
result, err := c.SubmitTask(ctx, token, types.TaskSubmitInput{
	TaskID:     18155,
	Content:    "手握扫帚净校园，春意盎然拂面来。每一次躬身劳动，都是对责任与成长的最好诠释。",
	ImagePaths: []string{"./photo.jpg"},
	ImageIDs:   []int64{123456}, // 可选；已上传过的附件 ID，可与 ImagePaths 混用
	Address:    "操场",           // 部分活动类型必填；空串不会自动变学校名
	Level:      "5",             // 部分活动类型必填；空串不会默认 5
	PlayRole:   "1",             // 部分活动类型必填
	// Hours: 省略 → 任务预设>0 时自动；任务无预设时必须传，如 Hours: "2"
	ActivityName: "校园劳动实践",  // 按活动类型填写
	HostName:     "班主任",
})
if err != nil {
	var bErr *types.BusinessError
	if errors.As(err, &bErr) {
		log.Printf("业务拒绝：code=%d msg=%s", bErr.Code, bErr.Msg)
	}
	log.Fatalf("提交失败：%v", err)
}
log.Printf("提交成功，result=%+v", result)
```

SDK 响应示例：

```json
{
  "code": 1,
  "msg": "成功"
}
```

失败示例（该任务仅允许提交 1 次）：

```json
{
  "code": -1,
  "msg": "发表写实失败，限制本写实活动只能发表1次"
}
```

---

## 自我评价域（self_eval.go）

### `SubmitSelfEvaluation(ctx context.Context, token, comment string) error`

提交自我评价文本。

请求示例：

```go
err := c.SubmitSelfEvaluation(ctx, token, "这学期我尽量保持学习的专注，每天按时完成作业，课堂上也坚持记笔记。虽然有些科目像数学和物理理解起来有点吃力，但我已经试着提前预习并多问老师同学了。")
if err != nil {
	log.Fatalf("提交自我评价失败：%v", err)
}
```

SDK 响应示例：

```json
null
```

### `QuerySelfEvaluation(ctx context.Context, token string) (*types.SelfEvalStatus, error)`

查询自我评价 + 教师评语。

空数据契约：服务端业务成功（code=1）但尚未提交评价时返回 `(nil, nil)`，与 `QuerySelfEvaluationJSON` 一致；**不是**错误。调用方应先判 `err`，再判 `status == nil`。

**字段命名**：平台 `dataMap` 主路径为 `student_comment` / `teacher_comment`（前端 mainLeft、selfgaintloss 读取方式）；`SelfEvalStatus` 自定义 `UnmarshalJSON` 兼容 snake 与 camel。导出结构体字段仍是 `StudentComment`/`TeacherComment`，`json` 序列化输出 camelCase（与提交键 `studentComment` 一致）。

请求示例：

```go
status, err := c.QuerySelfEvaluation(ctx, token)
if err != nil {
	log.Fatalf("查询自我评价失败：%v", err)
}
if status == nil {
	log.Println("尚未提交自我评价")
	return
}
log.Printf("自评：%s", status.StudentComment)
log.Printf("师评：%s", status.TeacherComment)
```

SDK 响应示例（Go 结构体序列化后；原始 API 可能为 snake_case）：

```json
{
  "id": 372235,
  "studentComment": "这学期我尽量保持学习的专注，每天按时完成作业，课堂上也坚持记笔记...",
  "teacherComment": "你开朗、乐观，在课上你认真听讲，积极发言，独立、分析问题的能力较强..."
}
```

### `QuerySelfGradEvaluation(ctx context.Context, token string) (*map[string]any, error)`

查询毕业相关评价状态。返回值保持平台原结构，不做字段裁剪，适合高级调用方直接探查原始键值。

请求示例：

```go
grad, err := c.QuerySelfGradEvaluation(ctx, token)
if err != nil {
	log.Fatalf("查询毕业评价失败：%v", err)
}
if grad != nil {
	log.Printf("原始结果：%v", *grad)
}
```

SDK 响应示例：

```json
{
  "graduated": true
}
```

---

## 已提交写实记录域（submitted.go）

### `GetSubmittedCircles(ctx context.Context, token string) ([]types.CircleRecord, error)`

获取同班同学的全部已提交写实记录（含姓名、学号、正文、图片、审核状态）。内部自动翻页合并。

请求示例：

```go
records, err := c.GetSubmittedCircles(ctx, token)
if err != nil {
	log.Printf("获取写实记录失败（部分成功 %d 条）：%v", len(records), err)
}
for _, r := range records {
	fmt.Printf("记录：%s（%.1f 学时）\n", r.Name, r.Hours)
}
```

SDK 响应示例（通过 `GetSubmittedCirclesJSON` 原始 JSON 路径获取，1 条）：

```json
[
  {
    "id": 5400001,
    "name": "20260309高一（8）班致敬\"她付出\"——从感恩到理解的深度对话（感恩教育）",
    "content": "在2026年3月9日高一（8）班\"致敬'她付出'——从感恩到理解的深度对话\"主题班会中，我深受触动。通过老师的引导与同学们的分享，我深刻体会到母亲及身边女性长辈们日复一日的默默付出与无私奉献。",
    "type": 1,
    "type_name": "主题班会",
    "circle_type_id": 9256,
    "dimension_id": 9,
    "hours": 0.5,
    "studentId": 380001,
    "student_num": "G350181200X00000001",
    "operator_id": 380001,
    "operator_name": "赵明轩",
    "creator": 380001,
    "creator_name": "赵明轩",
    "class_id": 162647,
    "class_name": "八班",
    "grade_id": 27900,
    "grade_name": "高一",
    "school_id": 173,
    "term_id": 18,
    "role_name": "班主任",
    "status": 0,
    "up_pic": 1,
    "circle_date": "2026-07-12",
    "creation_time": 1783862109000,
    "creationTimeStr": "2026-07-12 21:15",
    "circle_task_id": 18001,
    "circle_task_name": "20260309高一（8）班致敬\"她付出\"——从感恩到理解的深度对话（感恩教育）",
    "scope_type": 1,
    "scope_type_name": "班级任务",
    "state_type": 3,
    "remark": "心得+照片",
    "start_date": "2026-07-10",
    "end_date": "2026-07-18",
    "audit_start_date": "2026-07-19",
    "audit_end_date": "2026-07-22",
    "address": "班级",
    "imgList": [
      {
        "id": 5000001,
        "circle_id": 5400001,
        "class_id": 162647,
        "task_id": 18001,
        "attachment_id": 5000001,
        "imgPath": ".jpg"
      }
    ],
    "imgPreViewList": [
      "http://www.nazhisoft.com/common/attachment/getImg?id=5000001"
    ]
  }
]
```

### `GetSubmittedCirclesLimitJSON(ctx context.Context, token string, offset, limit int, key string) (json.RawMessage, *types.PageBean, error)`

按偏移和条数限制拉取已提交写实记录（原始 JSON）。v1.1.2 新增。

参数 `offset=0, limit=0` 时等于 `GetSubmittedCirclesJSON`（全量）。`offset`/`limit` 超出实际数据量时返回空数组，不报错。

返回数据列表的原始 JSON 数组 + 分页信息（含 `TotalNum`，可用于获取总条数）。

### `PeekSubmittedTotal(ctx context.Context, token string, key string) (int, error)`

轻量获取已提交写实记录总数。v1.2.2 新增。

内部调用 `getStudentCircle?type=1&pageNo=1&pageSize=1`，只提取 `PageBean.TotalNum`。
只拉 1 条记录获取分页信息，不拉全量列表。

请求示例：

```go
total, err := c.PeekSubmittedTotal(ctx, token)
if err != nil {
	log.Fatalf("获取记录总数失败：%v", err)
}
fmt.Printf("共有 %d 条写实记录\n", total)
```

SDK 响应示例：

```json
{"total": 23}
```

---

## 荣誉申报域（honor.go）

### `GetHonorTypes(ctx context.Context, token string) ([]types.HonorType, error)`

获取所有可申报的荣誉类型列表。

请求示例：

```go
types, err := c.GetHonorTypes(ctx, token)
if err != nil {
	log.Fatalf("获取荣誉类型失败：%v", err)
}
for _, t := range types {
	fmt.Printf("荣誉：%s（%s，%s）\n", t.Name, t.LevelName, t.DimensionName)
}
```

SDK 响应示例（前 5 条）：

```json
[
  {
    "id": 1147,
    "name": "校学生优秀干部",
    "levelName": "",
    "level": 5,
    "dimensionName": ""
  },
  {
    "id": 1148,
    "name": "校三好学生",
    "levelName": "",
    "level": 5,
    "dimensionName": ""
  },
  {
    "id": 1150,
    "name": "二级运动员",
    "levelName": "",
    "level": 1,
    "dimensionName": ""
  },
  {
    "id": 1151,
    "name": "校优秀团干部",
    "levelName": "",
    "level": 5,
    "dimensionName": ""
  },
  {
    "id": 1251,
    "name": "福清市\"三好学生\"",
    "levelName": "",
    "level": 4,
    "dimensionName": ""
  }
]
```

### `GetHonorTypeForSelect(ctx context.Context, token string) ([]types.HonorSelectOption, error)`

获取荣誉类型下拉选项，返回 `label/value` 对。

请求示例：

```go
opts, err := c.GetHonorTypeForSelect(ctx, token)
if err != nil {
	log.Fatalf("获取荣誉类型下拉失败：%v", err)
}
for _, opt := range opts {
	fmt.Printf("%s => %d\n", opt.Label, opt.Value)
}
```

SDK 响应示例：

```json
[
  {"label": "校三好学生", "value": 1148},
  {"label": "校学生优秀干部", "value": 1147}
]
```

### `GetHonorLevel(ctx context.Context, token string, honorTypeID int64) ([]types.HonorSelectOption, error)`

获取某个荣誉类型可选的级别下拉选项。

请求示例：

```go
levels, err := c.GetHonorLevel(ctx, token, 1148)
if err != nil {
	log.Fatalf("获取荣誉级别失败：%v", err)
}
for _, lv := range levels {
	fmt.Printf("%s => %d\n", lv.Label, lv.Value)
}
```

SDK 响应示例：

```json
[
  {"label": "校", "value": 5},
  {"label": "区/县/街道/社区", "value": 4}
]
```

### `GetHonorList(ctx context.Context, token string, pageNo, pageSize int, key string) (*types.HonorListResult, error)`

获取已申报的荣誉记录。同时返回分页信息。

请求示例：

```go
result, err := c.GetHonorList(ctx, token, 1, 20, "")
if err != nil {
	log.Fatalf("获取荣誉记录失败：%v", err)
}
fmt.Printf("共 %d 条（第 %d/%d 页）\n", result.Page.TotalNum, result.Page.PageNo, result.Page.TotalPage)
for _, r := range result.Records {
	fmt.Printf("荣誉：%s（%s，%s）\n", r.TypeName, r.LevelName, r.ApprovedName)
}
```

SDK 响应示例：

```json
{
  "records": [
    {
      "id": 56241,
      "typeName": "阅读之星",
      "levelName": "校",
      "level": 5,
      "dimensionName": "学业水平",
      "approved": true,
      "approvedName": "审核通过",
      "getDate": "2025-11-21T00:00:00+08:00",
      "evaluationAgency": "示例中学"
    }
  ],
  "page": {
    "pageNo": 1,
    "pageSize": 20,
    "totalNum": 1,
    "totalPage": 1
  }
}
```

### `AddHonor(ctx context.Context, token string, payload types.AddHonorPayload) error`

申报一条荣誉。

请求示例：

```go
err := c.AddHonor(ctx, token, types.AddHonorPayload{
	Name:             "校三好学生",
	TypeID:           1148,
	TypeName:         "校三好学生",
	Level:            5,
	EvaluationAgency: "示例中学",
	GetDate:          "2026-07-01",
})
if err != nil {
	log.Fatalf("申报荣誉失败：%v", err)
}
```

SDK 响应示例：

```json
null
```

### `DeleteHonor(ctx context.Context, token string, honorID int64) error`

按 ID 删除一条已申报的荣誉记录。接口为 GET 请求，ID 通过查询参数传递。

请求示例：

```go
err := c.DeleteHonor(ctx, token, 62702)
if err != nil {
	log.Fatalf("删除荣誉失败：%v", err)
}
```

SDK 响应示例：

```json
null
```

---

## 典型案例域（typical_case.go）

### `AddTypicalCase(ctx context.Context, token string, payload types.AddTypicalCasePayload) error`

用户只需传 type/role/level **代码**；`typeName`/`roleName`/`levelName` 为空时 SDK 按 classiccanter.vue el-option 自动补全。已填展示名不覆盖。

`UpdateTypicalCase` 使用 `map[string]any`：`type`/`role`/`level` 支持 **string 或 number**（列表回填常为 number），对应 `*Name` 为空时同样自动补全。

提交一条典型案例。

请求示例（展示名可省略，由 SDK 补全）：

```go
err := c.AddTypicalCase(ctx, token, types.AddTypicalCasePayload{
    Title:          "论国内外各领域AI大模型能力对比",
    Type:           "1", // → typeName 研究性学习报告
    TeacherName:    "王老师",
    PartnerName:    "庄同学等",
    Role:           "1", // → roleName 负责人
    Remark:         "基于2026年前沿大语言模型全栈实测数据的深度解析",
    Content:        "经过本课题组数周的协作攻坚...",
    Level:          "5", // → levelName 学校
    AttachmentID:   5139876,
    AttachmentName: "example.jpg",
})
if err != nil {
    log.Fatalf("提交典型案例失败：%v", err)
}
```

payload 字段说明：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `title` | string | ✅ | 标题（用户） |
| `type` | string | ✅ | 类别代码：`"1"` 研究性学习报告 / `"2"` 社会调查报告 / `"3"` 艺术创作作品 / `"4"` 其他 |
| `typeName` | string | — | 类别名；空则 SDK 按 type 补全 |
| `teacherName` | string | — | 指导教师（用户） |
| `partnerName` | string | — | 合作者（用户） |
| `role` | string | ✅ | 角色代码：`"1"` 负责人 / `"2"` 参与者 |
| `roleName` | string | — | 角色名；空则 SDK 补全 |
| `remark` | string | — | 备注（用户） |
| `content` | string | ✅ | 正文（用户） |
| `level` | string | ✅ | 级别代码：`"1"` 国际 / `"2"` 省 / `"3"` 市 / `"4"` 区县 / `"5"` 学校 |
| `levelName` | string | — | 级别名；空则 SDK 补全 |
| `attachmentId` | int64 | — | 附件 ID（先 `UploadFile`） |
| `attachmentName` | string | — | 附件文件名 |

SDK 响应示例：

```json
null
```

失败示例：

```json
{
  "code": -1,
  "msg": "标题不能为空"
}
```

### `GetTypicalCaseList(ctx context.Context, token string, pageNo, pageSize int, status ...int) (*types.TypicalCaseListResult, error)`

查询典型案例列表（分页）。`status` 为可选变参，默认 `3`（全部），与前端下拉一致：

| status | 含义 |
|--------|------|
| 0 | 未审核 |
| 1 | 通过 |
| 2 | 驳回 |
| 3 | 全部（默认） |

请求示例：

```go
// 默认全部
result, err := c.GetTypicalCaseList(ctx, token, 1, 20)
// 仅已通过
result, err := c.GetTypicalCaseList(ctx, token, 1, 20, 1)
if err != nil {
    log.Fatalf("获取典型案例列表失败：%v", err)
}
fmt.Printf("共 %d 条（第 %d/%d 页）\n", result.Page.TotalNum, result.Page.PageNo, result.Page.TotalPage)
for _, r := range result.Records {
    fmt.Printf("案例：%s（%s，%s）\n", r.Title, r.TypeName, r.StatusName)
}
```

SDK 响应示例：

```json
{
  "records": [
    {
      "id": 20034,
      "title": "论国内外各领域AI大模型能力对比",
      "typeName": "研究性学习报告",
      "teacherName": "王老师",
      "partnerName": "庄同学等",
      "roleName": "负责人",
      "remark": "基于2026年前沿大语言模型全栈实测数据的深度解析",
      "content": "经过本课题组数周的协作攻坚...",
      "attachmentId": 5139876,
      "attachmentName": "example.jpg",
      "status": 0,
      "statusName": "未审核",
      "termId": 18,
      "termName": "2025-2026学年下学期",
      "gradeName": "高一",
      "className": "某班",
      "studentName": "陈同学"
    }
  ],
  "page": {
    "pageNo": 1,
    "pageSize": 20,
    "totalNum": 1,
    "totalPage": 1
  }
}
```

### `GetTypicalCaseListJSON(ctx context.Context, token string, pageNo, pageSize int, status ...int) (json.RawMessage, error)`

返回典型案例列表的原始 JSON，CLI 1:1 对齐用途。`status` 语义同 `GetTypicalCaseList`（默认 3=全部）。
返回拼装后的完整 JSON 对象 `{"records":..., "page":...}`，records 和 page 字段值都是平台原始字节。

---

## 文件域（file.go）

### `UploadFile(ctx context.Context, filePath string) (*types.UploadFileResult, error)`

上传图片到文件服务器，返回附件 ID。上传前自动预处理：任意格式 → JPG + 透明合成 + 缩放/质量级联 → ≤5MB。支持 JPEG / PNG / GIF（自动取首帧）/ WEBP；BMP 需先转换。

请求示例：

```go
result, err := c.UploadFile(ctx, "./photo.jpg")
if err != nil {
	if errors.Is(err, client.ErrFileTooLarge) {
		log.Fatal("图片压缩后仍超 5MB")
	}
	if errors.Is(err, client.ErrUploadRejected) {
		log.Fatalf("上传被拒：%v", err)
	}
	log.Fatalf("上传失败：%v", err)
}
log.Printf("上传成功，图片 ID：%d", result.AttachmentID)
```

SDK 响应示例：

```json
{
  "attachmentID": 5041963
}
```

### `DownloadFile(ctx context.Context, attachmentID int64, dst string) error`

按附件 ID 下载图片到本地。入口 `ssoBaseURL/common/attachment/getImg?id=X`，跟随 302 到 FastDFS 真实存储。不发鉴权头。

请求示例：

```go
err := c.DownloadFile(ctx, 5041963, "./downloaded.jpg")
if err != nil {
	log.Fatalf("下载失败：%v", err)
}
```

SDK 响应示例：

```json
null
```

---

## 资源释放（Close）

```go
func (c *Client) Close() error
```

释放 OCR session、HTTP keep-alive 连接与 session backoff 状态。

请求示例：

```go
c, err := client.New()
if err != nil {
	log.Fatalf("构造 Client 失败：%v", err)
}
defer func() {
	if err := c.Close(); err != nil {
		log.Printf("资源释放失败：%v", err)
	}
}()
```

SDK 响应示例：

```json
null
```

---

## 错误处理

所有 SDK 错误都是 `error` 类型，可通过 `errors.Is` 精确判定：

```go
_, err := c.Login(ctx, req)
switch {
case errors.Is(err, client.ErrLoginRejected):
	// 账号/密码/验证码错误
case errors.Is(err, client.ErrOCRNotConfigured):
	// !ddddocr 构建且未注入 WithCustomOCR
case errors.Is(err, client.ErrOCRPanic):
	// OCR 识别器 panic 被 recover
case errors.Is(err, client.ErrNetwork):
	// 网络层失败
case errors.Is(err, client.ErrTimeout):
	// 超时
case errors.Is(err, client.ErrRateLimited):
	// HTTP 429
case errors.Is(err, client.ErrServiceUnavailable):
	// HTTP 5xx
case errors.Is(err, client.ErrInvalidResponse):
	// HTTP 4xx（排除 429）
case errors.Is(err, client.ErrUploadRejected):
	// 文件被拒绝
case errors.Is(err, client.ErrFileTooLarge):
	// 图片压缩后仍 > 5MB
case errors.Is(err, client.ErrInvalidPayload):
	// 任务 payload 字段缺失
case errors.Is(err, client.ErrBusinessRejected):
	// 业务请求被拒绝
case errors.Is(err, client.ErrSessionBackoff):
	// session 激活冷却中
case errors.Is(err, client.ErrEmptyUserInfo):
	// 业务成功但无数据
case errors.Is(err, client.ErrRetryable):
	// ctx 取消导致的可重试错误
}
```

---

## pkg/tokenparse 单独使用

```go
import "github.com/Wenaixi/nazhi-cli/pkg/tokenparse"

location := "https://www.nazhisoft.com/uiStudentLogin/login?token=eyJhbGc...&expires_in=3600"
token, expiresAt, err := tokenparse.ExtractFromLocation(location)

raw := []byte(`{"code":1,"returnData":{"token":"xxx","expires_in":3600}}`)
token, expiresAt, err = tokenparse.ExtractFromReturnData(raw)
```

---

## pkg/types 类型索引

| 类型 | 字段数 | 文件 | 说明 |
|---|---|---|---|
| `LoginRequest` | 3 | `login.go` | SchoolID / Username / Password |
| `LoginResponse` | 3 | `login.go` | Token / ExpiresAt / FallbackUsed |
| `SchoolInfo` | 2 | `login.go` | SchoolID / SchoolName |
| `UploadFileResult` | 1 | `login.go` | AttachmentID |
| `BusinessError` | 2 | `response.go` | Code / Msg |
| `UserInfo` | 13 | `user.go` | 用户身份/学校/班级/学号资料 |
| `Task` | 21 | `task.go` | 任务条目（含 Submitted/NeedPic 计算字段） |
| `TaskSubmitInput` | 7 | `task.go` | 最小任务提交输入（TaskID / Content / ImagePaths / ImageIDs / PlayRole / Address / Level） |
| `TaskAddCirclePayload` | 30 | `task.go` | SDK 内部组装的 addCircle 完整请求体 |
| `TaskResult` | 2 | `task.go` | Code / Msg |
| `HonorType` | 5 | `honor.go` | ID / Name / LevelName / Level / DimensionName |
| `HonorRecord` | 9 | `honor.go` | 荣誉记录 |
| `HonorListResult` | 2 | `honor.go` | Records / Page |
| `AddHonorPayload` | 7 | `honor.go` | 荣誉申报请求体 |
| `HonorSelectOption` | 2 | `honor.go` | Label / Value |
| `CircleRecord` | 多 | `circle.go` | 已提交写实；混用 tag；`PlayRole` 为 `PlayRoleCode`（数字/字符串） |
| `CircleImage` | 6 | `circle.go` | 写实图片附件 |
| `PageBean` | 4 | `circle.go` | PageNo / PageSize / TotalNum / TotalPage |
| `PlayRoleCode` | — | `flexjson.go` | 承担角色码，JSON number/string → `"1"`/`"2"`/`"3"` |
| `SelfEvalStatus` | 3 | `self_eval.go` | ID / StudentComment / TeacherComment（解码兼容 snake） |
| `Dimension` | 2 | `dimension.go` | ID / Name |
| `AddTypicalCasePayload` | 13 | `typical_case.go` | 典型案例提交请求体 |
| `TypicalCaseRecord` | 16 | `typical_case.go` | 典型案例列表记录 |
| `TypicalCaseListResult` | 2 | `typical_case.go` | Records / Page |

---

## pkg/types/response.go 泛型辅助

```go
resp, err := types.DecodeResponse(bodyBytes)
if err := types.CheckCode(resp); err != nil {
	// code != 1 -> *BusinessError
}

userInfo, err := types.DecodeReturnData[types.UserInfo](resp)
tasks, err := types.DecodeDataList[types.Task](resp)
selfEval, err := types.DecodeDataMap[types.SelfEvalStatus](resp)
pb, err := types.DecodePageBean(resp)
```
