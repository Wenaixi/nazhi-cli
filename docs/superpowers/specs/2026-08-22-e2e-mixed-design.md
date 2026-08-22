# 究极大测试：读真写模拟（可随时开真）统一套件

## 1. 上下文与目标

合并三套分散验证（pkg/client 单元 httptest / test/integration HAR 回放 / TestReal_FullChain 真域 + CLI 手动）为单套件 test/e2e，满足：
- 默认 读走真域、写走本地 httptest 模拟（不污染线上数据）
- `NAZHI_E2E_LIVE_WRITE=1` 一键切全真，满足“想要随时激活、开源随时可用”
- file upload/download 默认可走真文件域 `http://doc.nazhisoft.com`（无 token，安全）
- 不限速（无 sleep/backoff），FetchTasks 并发上限 8 保留
- 复用现有 pkg/client、pkg/types、test/integration/har_fixtures/*.json 与 helpers
- 无 secret 时 t.Skip，CI 仅跑离线部分永不挂

已验证：硅基流动 Qwen3-Omni 登录 + 20+ 只读接口（whoami/task list/submitted/teacher/withdrawn/public/dimensions/circle-types/honor/typical/self-eval/file download）全绿。

## 2. 方案对比（2 选 1，推荐 A）

### A. 单套件双后端（推荐）
test/e2e 内一个 harness 同时持有 liveClient（真 bizBase）和 mockWriteServer（httptest）。读直调 liveClient；写直调 mockClient（baseURL=mock.URL）。mock 按 pkg/types + 前端真实源码字段表严格校验，返回 `{"code":1}`。切换由 NAZHI_E2E_LIVE_WRITE 控制。
- 优点：复用全部方法签名，无需两套测试；file 真域自然可切；实现量小
- 缺点：需维护读写路由表

### B. 纯录制回放（不推荐）
全写录一次真响应存 JSON，回放比对。
- 缺点：首次录制即污染线上数据；回放易过期；不满足随时开真


选 A。

## 3. 架构

```
test/e2e/
  harness_test.go       # TestMain：模式决策、登录一次复用 token、启动 mockWriteServer
  token_cache.go        # TokenCache：读/写/判过期（exp 前 10min 有效）
  read_live_test.go     # 真读用例 table-driven
  write_mock_test.go    # 写模拟用例 构造 types.*Input → mockClient → 断言 recordedWrites
  file_live_test.go     # File 真上传/下载（默认真，可切 mock）
  fixtures/
    write_expectations.json
  internal/             # 可选：抽取 creds helpers 供 e2e 复用
```

运行时：TestMain 读 env → 决定 liveAvailable/liveWrite/liveUpload → 若需真读则 TokenCache 命中则复用否则 Login(硅基流动 Omni)+ActivateSession → New liveClient(真) + Start mockWriteServer(httptest) → m.Run() → 清理。

## 4. 组件详解

### 4.1 Harness (harness_test.go)
- 包 `package e2e`，包级 var：liveClient,mockClient *client.Client; liveToken string; liveAvailable,liveWrite,liveUpload bool; mockSrv *httptest.Server; recordedWrites []RecordedWrite + sync.Mutex
- 读 env：NAZHI_USERNAME/NAZHI_PASSWORD/NAZHI_SSO_BASE/NAZHI_BASE_URL/NAZHI_UPLOAD_URL/NAZHI_SILICONFLOW_API_KEY/NAZHI_SILICONFLOW_BASE_URL/NAZHI_E2E_LIVE_WRITE/NAZHI_E2E_LIVE_UPLOAD/NAZHI_E2E_TOKEN_CACHE
- Key 优先 env，其次回读 Nazhi-auto/backend/data/settings.yaml 的 ai.providers[0].api_key（可选不阻塞）
- mockWriteServer：httptest.NewServer(NewServeMux) 注册全部 15 条写路径，每 handler ReadAll→可选 json 校验→记录→返回 `{"code":1,"msg":"成功","returnData":{}}`（comment 返回 id/content）
- 暴露 helper：requireLive(t), isLiveWrite() bool

### 4.2 TokenCache (token_cache.go)
- JSON {username,token,exp,savedAt} 落盘 .e2e_token 0600，命中条件 username 一致且 Now+10m < exp

### 4.3 Mock 路由表（15 条，逐字）
- POST /api/studentCircleNew/addCircle, POST /editCircle, GET /deleteCircle, POST /addCircleComment, GET /setCircleLikeById
- POST /addHonor, POST /updateHonor, GET /deleteHonorById
- POST /addTypicalCase, POST /updateTypicalCase, GET /deleteTypicalCase, POST /deleteBatchTypicalCase
- POST /addSelfEvaluation, POST /addSelfGradEvaluation
- POST /updateMyInfo

### 4.4 ReadLive / WriteMock / FileLive
- ReadLive：TestE2E_ReadLive table-driven，覆盖下表读列，无 secret t.Skip，429/5xx t.Skip 不 Fail
- WriteMock：TestE2E_WriteMock 构造最小合法 types.*Input 调 mockClient，断言 err==nil 且 recordedWrites 匹配，负向可选
- FileLive：16x16 JPEG 走真文件域断言 attachmentId>0，download 落盘断言大小>0，LIVE_UPLOAD=0 时走 mock

## 5. 读写判定表

| 方向 | 方法 |
|---|---|
| 读真 | FetchTasks/GetDimensions/GetMyInfo/GetSubmitted/GetTeacher/GetWithdrawn/GetPublic/GetCircleTypes/Tasks/Images/GetDictList/GetHonorTypes/GetHonorTypeOptions/GetHonorTypeForSelect/GetHonorLevels/GetHonorList/GetTypicalCaseList/QuerySelfEvaluation/QuerySelfGradEvaluation/GetCircleTypeByTaskID |
| 写模拟 | SubmitTask/EditCircle/DeleteCircle/AddCircleComment/SetCircleLike/AddHonor/UpdateHonor/DeleteHonor/AddTypicalCase/UpdateTypicalCase/DeleteTypicalCase/DeleteBatch/SubmitSelfEval*/UpdateMyInfo* |
| 特殊 | UploadFile/DownloadFile 默认真，LIVE_UPLOAD=0 时走 mock |

校验基准：pkg/types/*.go json tag + docs/sdk/README.md 30 字段表 + E:/newCC/life-new2026/nazhi/src 前端 form。

## 6. 数据流与错误处理

时序：env 解析 → TokenCache → Login+ActivateSession(若需) → 读真直连真域 → 写走 mock 校验 → File 真链路 → 汇总 PASS/SKIP。
- OCR 不可用 → t.Skip
- 真域 401 → 重登一次仍 401 则 Skip；429/5xx → Skip 不 Fail，写 mock 不受影响
- TokenCache 损坏 → 忽略重登；Mock 过严 → 以前端源码为准，handler 注释标来源

## 7. 文件变更（分组交付）

- Task1：新增 test/e2e/harness_test.go, token_cache.go；修改 .gitignore 加 .e2e_token
- Task2：新增 read_live_test.go
- Task3：新增 write_mock_test.go + fixtures/write_expectations.json
- Task4：新增 file_live_test.go；修改 Makefile 加 e2e/e2e-mixed/e2e-live targets；README/docs 加 E2E 章节
- 无 SDK 公开 API 变更；仅新增 test/e2e

## 8. 测试策略

- 自测：go test ./..., go vet ./..., golangci-lint run ./...
- E2E 离线：go test ./test/e2e -run TestE2E_WriteMock -count=1 必过
- E2E 真读：NAZHI_USERNAME=... NAZHI_PASSWORD=... go test ./test/e2e -run TestE2E_ReadLive -count=1 -v
- 一键：make e2e / make e2e-live
- 开关验收：LIVE_WRITE=1 时日志含 LIVE WRITE ENABLED，否则 mock RequestCount>0 且无真写发出

## 9. 风险与里程碑

- 前端字段迭代需同步 fixtures
- 高峰限流已通过 Skip 降级
- 里程碑：Task1 harness 可跑 → Task2 读回归闭环 → Task3 写模拟全绿 → Task4 file+文档收口达到究极大测试交付
