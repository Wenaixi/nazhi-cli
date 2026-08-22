# Plan: 究极大测试 读真写模拟（可随时开真）统一套件

## 上下文
规格：docs/superpowers/specs/2026-08-22-e2e-mixed-design.md（32行精简版，细节以本文为准）。
目标：单套件 test/e2e 默认读真写模拟，NAZHI_E2E_LIVE_WRITE=1 一键切全真；file upload 默认真；不限速；复用现有 pkg/client 与 test/integration/har_fixtures；无 secret 时 t.Skip。

## 任务（顺序，子代理驱动）

### Task 1 — Harness 与 Token 缓存（阻塞后续）
- 新增: test/e2e/harness_test.go, test/e2e/token_cache.go
- 修改: .gitignore 加 .e2e_token
- 细则:
  - package e2e; 包级 var liveClient,mockClient *client.Client; liveToken string; liveAvailable,liveWrite,liveUpload bool; mockSrv *httptest.Server; recordedWrites []RecordedWrite + sync.Mutex
  - 读 env: NAZHI_USERNAME/NAZHI_PASSWORD/NAZHI_SSO_BASE/NAZHI_BASE_URL/NAZHI_UPLOAD_URL/NAZHI_SILICONFLOW_API_KEY/NAZHI_SILICONFLOW_BASE_URL/NAZHI_E2E_LIVE_WRITE/NAZHI_E2E_LIVE_UPLOAD/NAZHI_E2E_TOKEN_CACHE
  - 硅基流动 key 优先 env，其次回读 E:/newCC/life-new2026/Nazhi-auto/backend/data/settings.yaml 的 ai.providers[0].api_key（可选，不阻塞）
  - TokenCache: JSON {username,token,exp,savedAt} 落盘 0600，命中条件 username 一致且 Now+10m < exp
  - mockWriteServer: httptest.NewServer(NewServeMux) 注册全部 15 条写路径（POST /api/studentCircleNew/addCircle, POST /editCircle, GET /deleteCircle, POST /addCircleComment, GET /setCircleLikeById, POST /addHonor, POST /updateHonor, GET /deleteHonorById, POST /addTypicalCase, POST /updateTypicalCase, GET /deleteTypicalCase, POST /deleteBatchTypicalCase, POST /addSelfEvaluation, POST /addSelfGradEvaluation, POST /updateMyInfo），每 handler ReadAll→可选 json 校验→记录→返回 {"code":1,"msg":"成功","returnData":{}}（comment 返回 id/content）
  - TestMain: 解析 env→决定 liveAvailable/liveWrite/liveUpload→若 liveAvailable 则 Login(WithCustomOCR)→ActivateSession→启动 mockSrv→m.Run()→Close
  - 暴露 helper: requireLive(t), isLiveWrite() bool
- 验证: go vet ./test/e2e && go test ./test/e2e -count=1 -v 在无 secret 时 PASS（仅 mock 分支）

### Task 2 — 真读用例收敛（Read-Live）
- 新增: test/e2e/read_live_test.go
- 覆盖 (只读): whoami/user info/session activate/task list/submitted/teacher/withdrawn/public/dimensions/circle-type(types:3694)/circle types/tasks/images/dict(23)/honor types/type-options/level-options/levels(1148,1291)/list/typical-case list(默认与 status=1)/self-eval status/grad-status — table-driven 在 TestE2E_ReadLive 内，无 secret 时 t.Skip，429/5xx 时 t.Skip 不 Fail
- 验证: go test ./test/e2e -run TestE2E_ReadLive -count=1 -v（有 secret 时全绿）

### Task 3 — 写模拟用例（Write-Mock）
- 新增: test/e2e/write_mock_test.go, test/e2e/fixtures/write_expectations.json
- 覆盖 (写): SubmitTask/EditCircle/DeleteCircle/AddCircleComment/SetCircleLike/AddHonor/UpdateHonor/DeleteHonor/AddTypicalCase/UpdateTypicalCase/DeleteTypicalCase/DeleteBatchTypicalCase/SubmitSelfEvaluation/SubmitSelfEvaluationStructured/SubmitSelfGradEvaluation/UpdateMyInfo/UpdateMyInfoStructured — 构造最小合法 types.*Input/Payload 调 mockClient，断言 err==nil 且 recordedWrites 匹配，负向用例可选
- 验证: go test ./test/e2e -run TestE2E_WriteMock -count=1 必过（离线）

### Task 4 — File 真上传与下载 + Makefile/文档收尾
- 新增: test/e2e/file_live_test.go
- 修改: Makefile 加 e2e/e2e-mixed/e2e-live targets; .gitignore 已在 Task1 处理; README.md 与 docs/cli/README.md + docs/sdk/README.md 加 E2E 章节（可选小段）
- 细则: file upload 默认真 (NAZHI_E2E_LIVE_UPLOAD=1), 用 16x16 JPEG 走 doc.nazhisoft.com 断言 attachmentId>0; download 用该 id 落盘断言大小>0; NAZHI_E2E_LIVE_UPLOAD=0 时走 mock 不发真请求；NAZHI_E2E_LIVE_WRITE 控制写是否切真（本任务不强制切真写，仅保证 file 真链路）
- 验证: go test ./test/e2e -run TestE2E_File -count=1; go test ./...; go vet ./... 全绿

## 全局约束
- 不破 pkg/client 公开 API，仅新增 test/e2e
- 复用 test/integration/har_fixtures + integration_test.go 的 loadCreds/sharedLogin 思路
- 无 sleeps/限速，FetchTasks 上限 8 保留
- 开源闸门: 无 secret 自动 Skip 真读，无 LIVE_WRITE=1 写永不发真请求
