# 架构总览

## 双层架构

```
┌─────────────────────────────────────┐
│  cmd/nazhi/  (CLI 层 - cobra 命令)   │
│  - 薄壳层：参数解析 + JSON 输出      │
│  - 11 个用户可见命令                  │
└────────────┬────────────────────────┘
             │ 调用
             ↓
┌─────────────────────────────────────┐
│  pkg/client/  (SDK 层 - 核心业务)    │
│  - Option 模式构造 Client            │
│  - 13 个公开方法                     │
│  - HAR 对齐的 4 步 Session 激活      │
│  - 进程级 OCR 单例                   │
└────────────┬────────────────────────┘
             │ 使用
             ↓
┌─────────────────────────────────────┐
│  pkg/types/  (共享类型)              │
│  - LoginRequest/Response, Task, ...  │
│  - UnifiedResponse 泛型辅助          │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│  internal/  (内部包)                 │
│  - internal/ocr/  跨平台 ddddocr     │
│  - internal/version/  版本号        │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│  test/integration/  (真实环境 + HAR)  │
│  - TestReal_FullChain  10 步端到端   │
│  - 6 个 HAR 驱动测试                 │
│  - 4 个回归测试                       │
└─────────────────────────────────────┘
```

## 目录结构

```
nazhi-cli/
├── cmd/
│   └── nazhi/                # CLI 入口
│       ├── main.go           # cobra root
│       ├── login.go          # SSO 登录
│       ├── school.go         # 查询学校 ID
│       ├── session.go        # 业务 Session
│       ├── whoami.go         # 用户信息
│       ├── task_list.go      # 任务列表
│       ├── task_submit.go    # 任务提交
│       ├── self_eval_*.go    # 自我评价
│       ├── file_upload.go    # 文件上传
│       ├── version.go        # 版本信息
│       ├── completion.go     # shell 自动补全
│       ├── env.go            # 环境变量加载
│       └── output.go         # JSON 输出
├── pkg/                      # 公开 SDK
│   ├── client/
│   │   ├── auth.go           # Login/GetSchoolID/InitSession
│   │   ├── session.go        # ActivateSession (4 步)
│   │   ├── task.go           # FetchTasks/SubmitTask
│   │   ├── self_eval.go      # 自我评价
│   │   ├── user.go           # GetMyInfo
│   │   ├── file.go           # UploadFile
│   │   ├── image_prep.go     # 图片压缩预处理
│   │   ├── request.go        # HTTP 封装
│   │   ├── client.go         # Client struct + Option
│   │   ├── errors.go         # 哨兵错误
│   │   └── *_test.go         # 单元测试
│   └── types/
│       ├── types.go          # 领域类型
│       └── response.go       # 泛型响应辅助
├── internal/                  # 内部包
│   ├── ocr/
│   │   ├── ocr.go            # 进程级单例
│   │   ├── onnx_*.go         # 5 平台 build tag
│   │   └── models/           # 嵌入模型
│   └── version/
│       └── version.go        # 版本号
├── test/
│   └── integration/           # 集成测试
│       ├── integration_test.go
│       └── har_fixtures/      # HAR 真实响应
├── docs/                      # 文档中心
│   ├── README.md
│   ├── cli/README.md
│   ├── sdk/README.md
│   ├── env-vars.md
│   └── har-testing.md
├── .github/workflows/ci.yml   # CI
├── CLAUDE.md                  # 项目记忆（git 忽略）
├── README.md
├── CONTRIBUTING.md
├── CHANGELOG.md
├── LICENSE
├── Makefile
├── go.mod
└── go.sum
```

## 关键架构决策

### 1. Option 模式构造 Client

每个 Client 拥有独立的 HTTP cookie jar，天然并发安全。

### 2. 跨平台 OCR

`internal/ocr/` 5 个 build-tag 隔离的 `onnx_*.go` 文件嵌入对应平台的 onnxruntime：
- windows/amd64
- windows/arm64
- linux/amd64
- linux/arm64
- darwin/arm64

### 3. 进程级 OCR 单例

`ocr.GetDefault()` 进程共享一个引擎，模型只解压一次。

### 4. HAR 验证的 Session 激活

必须按顺序执行 4 步（`/` + `getMenu` + `getMenu` + `getMyInfo`），否则业务接口返回空。

### 5. 双重 Token 注入

`WithToken()` 同时写 Header 和 Cookie，匹配服务器双重认证要求。

### 6. 上传安全隔离

`UploadFile` 使用独立 `http.Client`（无 cookie jar + 禁用重定向），防止 SSO Cookie 泄露到文件服务器。

### 7. 任务 Payload 透传

`TaskSubmitPayload` 29 字段不裁剪不处理，调用方传什么发什么。

### 8. 统一响应体解析

`UnifiedResponse` + 泛型辅助 `DecodeReturnData[T]` / `DecodeDataList[T]`。

### 9. HAR 驱动测试

把真实抓包作为 fixture 喂给 mock server，无需期末数据就能测任务流。

## 依赖关系

```
internal/ocr (dddocr + onnxruntime)  ← 跨平台二进制
        ↓
pkg/client  ← Option 模式 + 13 个公开方法
        ↓
cmd/nazhi  ← cobra 命令
        ↓
test/integration  ← 真实环境 + HAR fixtures
```

## 数据流

### 登录流

```
用户 → nazhi login
     → Login(ctx, LoginRequest)
     → InitSession (GET /uiStudentLogin/login)
     → GetSchoolID (POST getSchoolIdByStudentNumber)
     → ocrRecognizeWithRetry (GET kaptcha.jpg + 99 次重试)
     → validateCaptcha (POST /validateCaptcha)
     → Login POST (validate)
     → 302 → 提取 Location 中的 token
     → syncCookieToken (写 SSO + 业务域 Cookie)
     → 返回 LoginResponse.Token
```

### Session 激活流（4 步 HAR 对齐）

```
1. GET /                      (初始化后端 Session)
2. GET /api/studentInfo/getMenu  (Referer: /homepage?token=xxx)
3. GET /api/studentInfo/getMenu  (Referer: /home)
4. GET /api/studentInfo/getMyInfo (返回完整 UserInfo)
```

### 任务拉取流

```
FetchTasks(token)
  → ActivateSession (4 步)
  → getDimensions (返回 7 个维度)
  → 对每个非 0 维度:
      getCircleStatistics?dimensionId=X
  → 聚合所有任务，注入 dimensionName
```

### 任务提交流

```
SubmitTask(token, payload)
  → POST /api/studentCircleNew/addCircle
  → 29 字段 payload 透传
  → 返回 { code: 1, insertID: 12345 }
```
