# 究极大测试：读真写模拟 统一套件

## 1. 上下文与目标
合并三套验证（pkg/client 单元 httptest / HAR 回放 / TestReal_FullChain 真域 + CLI 手动）为单套件 test/e2e：
- 默认 读真写模拟（读走真域、写走本地 httptest mock，不污染线上数据）
- 写永远 mock，不支持全真（NAZHI_E2E_LIVE_WRITE=1 已禁用，设置仅打印 WARN）
- file upload/download 默认可走真文件域 http://doc.nazhisoft.com（无 token，安全）；NAZHI_E2E_LIVE_UPLOAD=0 可强制走 mock
- 不限速，无 sleep/backoff，FetchTasks 并发上限 8 保留
- 复用 pkg/client、pkg/types、har_fixtures/*.json 与 helpers
- 无 NAZHI_USERNAME 时 t.Skip 真读，CI 仅跑离线 mock 永远绿

## 2. 架构
test/e2e/harness_test.go (TestMain 双后端 + 15 条写 mock + Omni login + TokenCache)
test/e2e/token_cache.go (.e2e_token 0600, 10m 提前过期)
test/e2e/read_live_test.go (TestE2E_ReadLive table-driven, 22 只读)
test/e2e/write_mock_test.go (TestE2E_WriteMock 15 写永远 mock)
test/e2e/file_live_test.go (mock-encode 离线 + live 真域在 liveAvailable 时)
test/e2e/fixtures/write_expectations.json

## 3. 开关
- NAZHI_USERNAME/NAZHI_PASSWORD 有则启用真读，否则 Skip
- NAZHI_E2E_LIVE_WRITE 保留读取但强制忽略，仅 WARN（写永远 mock）
- NAZHI_E2E_LIVE_UPLOAD 默认 1（真域），设 0 走 mock
- NAZHI_SILICONFLOW_API_KEY 优先 env，其次回读 Nazhi-auto settings.yaml fallback

## 4. 文件变更
- Task1: harness+token_cache+.gitignore
- Task2: read_live
- Task3: write_mock+fixtures+session stubs
- Task4: file_live+Makefile e2e/e2e-mixed（e2e-live 已移除）

## 5. 验证
- go test -count=1 -race ./... 全绿
- go test ./test/e2e 离线全绿（读 Skip，写 15/15 PASS）
- NAZHI_USERNAME/PASSWORD 有 credential 时 go test ./test/e2e 读真 22/22 PASS，写仍 mock，file 真域 upload+download PASS
