# CHANGELOG

## [1.6.4] - 2026-09-05

### 修复

- 修复结构化和原始 JSON 写实列表在 `totalPage` 虚低或为 0 时的漏页与索引越界，限制分页同步按 `totalNum` 推导页数。
- 原始 JSON 写实列表拒绝非数组 `dataList`，统一返回 `ErrInvalidResponse`。
- 文件上传遇到无法安全 Clone 的自定义 `RoundTripper` 时回退到无状态默认传输器，避免认证拦截器状态进入上传通道。
- stdin 读取在取消或超时后关闭实际句柄，唤醒后台阻塞读取。

## [1.6.3] - 2026-09-05

### 修复

- 修复文档规则仍读取已删除的 `docs/cli/README.md` 与 `docs/sdk/README.md`，改为校验当前唯一的 `docs/README.md` 源码地图。
- 将文档规则和仓库元数据检查接入 Makefile 与 CI，避免治理测试脱离发布门禁。
- 修复 stderr 写入失败时 `printError` 未设置最终退出码的问题，确保异常输出路径仍返回非零退出状态。

## [1.6.2] - 2026-08-28

发布链接：[v1.6.2](https://github.com/Wenaixi/nazhi-cli/releases/tag/v1.6.2)

### 修复

- 三个时间敏感测试适配 CI 慢 runner，修复 v1.6.1 起 CI 红灯导致 Release 链断裂（commit `a7b27fe`）：TestFetchTasks_Parallel 的 1200ms 绝对耗时断言降级为观察日志（判别力由 ConcurrentLimitBounded 的 in-flight 峰值计数承担，CI 实测 5.11s vs 本机 1.2s）；TestFetchTasks_MixedBizAndCancel 的 ctx 1.5s→4s、handler 睡眠 2s→6s 保持 ctx 先超时语义；TestGetSubmittedCircles_CancelDuringPaging 两处 5s 保护超时放宽至 15s。

## [1.6.1] - 2026-08-28

发布链接：[v1.6.1](https://github.com/Wenaixi/nazhi-cli/releases/tag/v1.6.1)

### 修复

- 移除 go.mod 本地 replace（指向本机绝对路径导致异机/CI 构建失败），改为 go.mod 声明远程版本 + 本地开发用不入库 go.work 覆盖（本地优先/远程兜底）（commit `de969b5`）。
- captcha-sdk 依赖升级 v0.2.1（含 README import 路径修复与 CI 断言收紧）。

## [1.6.0] - 2026-08-27

### 修复

- ActivateSessionJSON 改调 GetMyInfo 使学校信息 SSO 降级补全真实生效（原实现直通 sm.Activate 后 Marshal，godoc 承诺的补全从未执行）；空数据仍返回 (nil,nil) 保持原契约（commit `a621901`）。
- UpdateCachedUserInfo 显式比对 token：签名加 forToken 参数，跨 token 的迟到写入（多 goroutine 场景）不再污染新 token 的缓存（commit `9f36dea`）。
- 日志脱敏先于截断：新增 logx.RedactBodyThenTruncate，修复敏感值跨 100 字节截断边界时正则失配泄漏前缀；request.go/file.go/auth.go 七处消费点统一改调，auth.go 四处 debug bodySnippet 同步改为脱敏版（commit `cdec8b9`）。
- httpDo 响应体读取封顶 1MB：异常/被劫持服务端塞超大 body 不再整体入内存，超限归 ErrInvalidResponse（commit `07ad8a5`）。
- UpdateMyInfoStructured 全零输入视为 no-op：不再发出仅含 studentUuid 空串的空 POST 并失效本地缓存（CLI --payload '{}' 可达）（commit `85ed8f9`）。
- 本地 IO 错误归参数档：上传附件不存在 / 图片解码打开失败 / 下载目标路径不可写由 SDK 包 ErrInvalidPayload 哨兵，CLI 退出码从 500/exit2 纠正为 400/exit3，脚本不再对永久性本地输入错误无限重试（commit `f42df62`）。

### 特性

- CLI 与 SDK 内置 nazhi-captcha-sdk 本地验证码识别器，Login 零配置可用，移除外部视觉模型 OCR 依赖（commit `25e02bb` 起；本提交同步清理集成测试与文档）。

### 文档

- GetDate wire 形态披露：前端 el-date-picker 无 value-format 实际提交 ISO 8601 时间戳，纯日期是否被服务端接受以平台裁决为准（types/honor.go + honor 命令 Long）。
- 典型案例 status 合法集合注释修正为 0/1/2/3（原漏 0=未审核）。
- GetSubmittedCirclesJSON godoc 修正为「恒为合法 JSON 数组」（原「可能为 null」失实）。

### 工程

- user update 测试 helper 单次 Body.Read 改 io.ReadAll 消除理论欠读（commit `b229b7d`）。
- gofmt 对齐 honor Long 与 client.go 空行（commit `b520bf2`）。

## [1.5.3] - 2026-08-26

### 修复

- 写实 remark 关键词强制传图分支无回归测试锁定：task.go:320-325（SDK 单方面发明的校验——前端 remark 仅作展示无此校验）的「备注含照片/图片/pdf + pictureList 为空 → ErrInvalidPayload」逻辑原本仅靠实现存在，重构误删不会有任何失败信号。新增 task_remark_image_required_test.go 表驱动测试 10 例覆盖（commit `897f9ac`）。
- 典型案例批删空切片守卫缺 pkg/client 层回归：typical_case.go:213-215 的 `len(ids)==0 → ErrInvalidPayload` 实现正确（commit 1522446 修复本体），但客户端层无任何测试断言该守卫。test/e2e:109 与 cmd/nazhi/missing_cli_capabilities_test.go:51-62 两处引用均只覆盖相邻路径。新增 typical_case_batch_empty_test.go 客户端层回归 2 例（nil + 空切片双态，httptest server 零业务请求计数）（commit `07fb3da`）。
- DownloadFile 中途传输失败缺 ErrNetwork 哨兵：file.go:435-438 copyErr 路径裸 `fmt.Errorf("写入文件失败: %w", copyErr)` 让 SDK 调用方按 `errors.Is(err, ErrInvalidResponse)` / `ErrNetwork` 判重试时不可识别（服务端 200+HTML 已被主管线拦截，但 mid-stream EOF/连接重置场景下 do() 拿不到响应头仅拿到 copyErr）；同函数 `:441` closeErr 路径同样裸包装。修复：copyErr 非 ctx 取消时包装 ErrNetwork 哨兵（用户主动 ctx 取消不归类为网络故障，避免自动重试误触发），closeErr 包 ErrNetwork。新增 file_download_midstream_test.go 回归 2 例（commit `83d1f41`）。
- 业务层四处 DecodeResponse 裸包装与主管线分叉：auth.go:49 GetSchoolID + auth.go:274 验证码预校验 + user.go:69 GetMyInfo + raw_json.go:591 fetchTasksDimensionJSON 自行调 `types.DecodeResponse` 后裸 `fmt.Errorf`，让 `errors.Is(err, ErrInvalidResponse)` 在服务端 200+HTML（WAF/维护页）场景下落空，CLI 漏斗走 default 500/exit2，与主管线 `doBizAndDecode` (request.go:234) 双 %w 哨兵口径分叉。修复：抽 `decodeOrInvalidResponse(opName, bodyBytes)` helper 接管 DecodeResponse + ErrInvalidResponse 包装；四处调用方各改一行（commit `b6b64b4`）。
- honor update 缺 payload 正数 id 校验：cmd/nazhi/honor.go:170-199 update 命令对 `payload["id"]` 零校验，会发出无 id 的业务请求，与 cmd/nazhi/typical_case.go:225-228 + 同文件 delete/levels 双重分叉。前端 performanceM.vue:489 编辑提交必然注入记录 id，此处对齐该契约。修复：平移 `typicalCasePayloadIDValid` 为共享 helper `PayloadPositiveIDValid` 到 cmd/nazhi/payload.go；honor update Run 在 json.Unmarshal 后调用该 helper，缺 id 或非正数 → envelope.Error(400) + exit 3 不发业务请求；typical-case update 改为调用共享 helper。新增 honor_update_id_test.go RED→GREEN 测试 1 例（commit `4f69402`）。
- postProcessSchoolFallback 锁外原地突变数据竞争窗口（防御纵深上沿）：ActivateSession 出口在 sm.mu 锁外对共享缓存指针（RecordSuccess 原指针入缓存）原地写 SchoolID/SchoolName，与 fast path 并发读取方形成真实数据竞争（Go 内存模型下 string 头撕裂风险）。本 API 的 godoc 明确承诺并发安全，但 `-race` detector 在 100 goroutine 并发激活测试中可复现。修复：新增 sm.fallbackDone atomic.Bool 标志区分首次激活与重入；首次激活走 `infoCopy := *info` 浅拷贝 + fallback 改副本 + `UpdateCachedUserInfo(&infoCopy)` 替换缓存指针 + fallbackDone 设 true；重入/fast path 命中直接返回缓存指针保持 DCL 同一缓存指针契约；RecordFailure 清 fallbackDone。pkg/client 全域 29 秒 -race 全绿（commit `b480538`）。
- honor list / typical-case list 分页参数缺非负校验：cmd/nazhi/honor.go:71-72 与 cmd/nazhi/typical_case.go:90-92 对 `--page`/`--page-size` 负值原样透传 SDK 查询串（`raw_json.go:749` 直拼 `strconv.Itoa(pageNo)`），对照组 cmd/nazhi/circle_metadata.go:83-90 对同形状参数有完整校验。修复：两处各加 4 行非负守卫 + envelope.Error(400) + exit 3（commit `926c897`）。
- WithHTTPClient 超时继承语义无专项回归测试：pkg/client/client.go:219-228 prevTimeout 继承是 #22 证伪后的行为加固产物，但仅由 godoc 承载；重构误删该逻辑 CI 全绿静默回归，重新引入 Option 声明顺序敏感性。新增 option_inherit_timeout_test.go 表驱动测试 2 例（双序）锁定（commit `926c897`）。

### 文档

- self_eval_submit_test.go:307 块注释「业务错误应触发 pendingExitCode=2（envelope.Error 5xx → exit code 2）」与紧随断言（:308）与 t.Errorf 文案（:309）均锁定 pendingExitCode=1（ErrBusinessRejected → 422 → exit 1）矛盾。CLAUDE.md #29 记载「同步更新 task_submit_test 与 self_eval_submit_test 两处锁定断言为 exit 1」时改了断言与 t.Errorf，漏改上一行的块注释。修正：块注释 2 改 1（commit `013a311`）。
- file.go:25 + :71 注释「前端限制 10MB」与 reference/nazhi 经典案例镜像实际提示文案「20MB」矛盾（commits ac2986a/1e34350 已同步镜像）。修正：两处注释「前端限制 10MB」改「前端镜像文案 20MB」（commit `013a311`）。
- task.go:407 EditCircle godoc 披露范围不足：原 godoc 点名回填例外仅 hours 与图片两项，首句「用户字段空串原样发送」在编辑语境下构成误导。前端 openEdit→getCircleTypeByTaskId 把列表记录 26 个活动字段（name/hostName/circleDate/rank/level/circleBeginDate/circleEndDate/checkResult/patentType/patentNum/address/termName/各类型专属字段/playRole/likeSpecialty1-3 等）整体回填 JSON.stringify 后整包提交；SDK 编辑路径若只填 `{id,taskId,content}`（CLI 官方示例正是如此引导），上述字段全部以空串上线。修正：godoc 扩写披露「前端编辑是 26 字段全量回填模式——任何留空的专属字段 SDK 均发空串，要保留原值请从 CircleRecord 对应字段回填」（commit `013a311`）。
- user.go:30 GetMyInfo godoc fast path 描述与实现相反：原注释「session 已激活（fast path）时返回 nil,nil」与实际不符——sm.Activate 持锁 fast path 命中返回 `(sm.cachedUserInfo, nil)` 非 nil，全链路不存在 (nil,nil) 返回。复用机制正因 fast path 返回非 nil info 被 :35-38 直接采纳。修正：注释「返回 nil,nil」改「返回缓存指针（非 nil）」（commit `013a311`）。
- cmd/nazhi/output.go:101 rejectLoneOffset godoc 缺调用次序披露：四写实列表命令允许在 buildBizClient 之后调用（task_teacher/public/submitted/withdrawn），与 honor delete / typical-case delete 等先校后建派两派并存。修正：godoc 加披露段说明「重构如欲收敛到先校后建，需同步四调用点的位置；当前两派并存是历史累积的有意保留」（commit `013a311`）。

### 工程

- gofmt 对齐 pkg/client/request.go + cmd/nazhi/payload.go 两个主树文件（commit `8b9620c`）。

## [1.5.2] - 2026-08-26

### 修复

- 文件下载 404/403 被误判为网络故障：DownloadFile 非 2xx 分支的 default 哨兵此前挂 ErrNetwork，确定性失败（附件已删/风控拦截）映射 502/exit2，脚本按「可重试」对永久失败无限重试；改归 ErrInvalidResponse（429→限流、5xx→服务端不可用分支不变），与 httpDo/doBizGet/doGetMenu/UploadFile 四个兄弟分支同口径，退出码纠正为 422/exit1（commit `563d1d1`）。
- 会话激活锁窗口可被放大到秒级：学校信息 SSO 回退补全（getMyInfo 缺 schoolId/schoolName 时）曾在 sm.mu 持锁路径内同步发起真实 SSO 域 POST，最坏把多 goroutine 并发激活的阻塞时间从数百毫秒放大到 HTTP 超时秒级，违背 ActivateSession 并发契约；回退已移至解锁后执行（幂等，字段已齐零开销），className 清理留锁内（commit `7a8d014`）。附带治理：两个单测夹具此前每次运行都向生产 SSO 域发真实请求，现已注入本地测试服务器。
- 荣誉 typeName 自动反查在大 id 下失效：反查比较用平台相关 int 宽度，typeId 超 2^31 时 32 位编译目标静默截断导致必不命中、退化为空 typeName 提交；统一 int64 比较（commit `730b3df`）。
- honor add 缺 --payload 的报错顺序与同族命令分叉：先建客户端后校验 payload，双参数缺失时报 token 配置错误而非参数错误；已收敛为先校验后建客户端的统一规范（commit `afc9806`）。
- task preview 同款校验顺序分叉：「先校验 payload 后建客户端」不变式的漏网第三处（submit/edit 已于上轮修复），位置对齐并补回归（commit `5467ce3`）。
- session --help 出现两行相同的 activate 条目：activate 子命令被注册两次而 cobra 不去重，删除重复注册点（commit `22b4093`）。

### 文档

- .env.example 补 NAZHI_LOG_LEVEL / NAZHI_LOG_FORMAT / NAZHI_LOG_FILE 三个环境变量示例，并披露 file download 与 upload 同为 30 秒超时档（commit `59b546d`）。
- typical-case submit 示例去除手填 typeName：示例引导用户手填展示名会使 SDK 自动补全链路失效，改由代码映射自动生成（与 honor add 示例同款修正）（commit `59b546d`）。
- 自我评价 *JSON 透传方法补充对账口径披露：前端唯一读取通道是 dataMap，双容器并存时透传内容可能与网页所见不一致（commit `59b546d`）。
- UserUpdateInput.Seat 注释明确字面 "0" 视为跳过不发送；如需强制清零走裸 map 路径（commit `59b546d`）。
- newCleanClient godoc 披露上传/下载通道超时下限 30s（小于该值静默上浮并告警）与无超时时 5 分钟兜底（commit `59b546d`）。

## [1.5.1] - 2026-08-25

### 修复

- 上传被服务端拒绝（文件类型不收/风控）误判为可重试的服务端故障：errors.go 16 个哨兵中 `ErrUploadRejected` 此前漏配 `mapSentinelToHTTPCode`，落 default 500/exit2；本轮补 case 归 422/exit1（与 ErrFileTooLarge 同族）。新增 TestMapSentinelToHTTPCode_UploadRejected 锁定（commit `1522446`）。
- GetSchoolID 断网/超时场景下学号泄漏：`request.go:327`（ErrTimeout）与 `:329`（ErrNetwork）两条 do() 网络层失败分支嵌入裸 `url`，与同文件其他六处已正确使用 `logx.RedactBody(url)` 脱敏不一致；修复后错误消息形如 `"请求 ...?userName=*** 失败:"`，参数名保留、值掩蔽。回归测试 `TestLogin_GetSchoolID_NetworkError_DoesNotLeakUsername` 锁定（commit `90ccd64`）。
  - **已知上限**：stdlib `*url.Error` 内嵌原始 URL 是 net/http intrinsic 行为，超出 SDK request.go 可控范围；更上层防御纵深挂在 cmd 层 printError 出口。

### 新增

- UploadFile 非图片直传白名单加入 .pdf（原样直传，与 doc/zip 同路径）；新增 file_upload_pdf_test.go 锁死「字节不改写、文件名保留、超限本地拒绝」三行为。用户需求：典型案例附件需支持 PDF（commit `f1a28d1`）。
- 非图片附件直传上限放宽至 20MB：服务端实测无 2MB 硬限（真实上限约 46.86MiB，2026-08-25 字节级二分探测），SDK 上限取用户决策的 20MB；前端仍限 10MB，CLI 直传不受前端约束（commit `2e69e21`）。

### 文档

- 参考镜像 `reference/nazhi/src/components/classic/classiccanter.vue` 两处上传提示（:147/:206）随 SDK 上限放宽同步插入 pdf、把「大小不超过2MB」改为 20MB（commits `ac2986a`、`1e34350`）；`reference/nazhi` 是镜像源、不是产品代码，但保持与上游线上文案一致便于用户对照。

## [1.5.0] - 2026-08-24

### 修复

- 上传图片按 EXIF Orientation 自动摆正：decodeImage 改用 imaging.AutoOrientation，竖拍照片经 CLI 上传不再横置（对齐前端 canvas drawImage 的现代浏览器默认行为）。
- FetchTasks 聚合结果按维度声明顺序稳定输出：ParallelDims 原按 goroutine 完成序追加导致同账号两次 task list 顺序抖动，现与 FetchTasksJSON 的保序策略对齐。
- 写实 content 超过 200 字显式拒绝：前端 el-input maxlength=200 为浏览器硬截断、线上恒发不超过 200 字；SDK 不再放行超长原文，返回 ErrInvalidPayload。
- 任务提交状态判定改子串匹配：「已结束 未提交」等自由文案变体不再误判为已提交。
- CircleRecord.LikeList JSON 键名修正为 likeList（真实 API 返回 camelCase），字段恢复可解码。
- 自我评价查询别名链收窄：移除无前端依据的投机键 content/teacherRemark，统一 snake 主读、camel 兼容。

### 破坏性变更

- AddHonorPayload.CertImgAttachmentID 类型 string → int64：出站对齐前端裸 number、无附件省略键；入站继续兼容 number/数字字符串/空串/null。直接以字符串字面量赋值该字段的 Go 调用方需改为数字。
- AddHonorPayload.Name 加 omitempty：前端 addHonor 表单不含 name 键，空 Name 不再出现在请求体。

### 加固

- 非图片附件上传先 os.Stat 预检大小再读入内存；CLI @file payload 与 stdin 对齐受 16 MiB 上限保护。
- tokenparse 补充 JWT exp 提取的实现注释；panic recover 退出码契约注释修正为实际值 2。

## [1.4.1] - 2026-08-24

### 变更

- `task submit` / `task edit` 新增图片数量上限校验：pictureList 合并后超过 2 张返回 ErrInvalidPayload，对齐前端 el-upload `:limit="2"` 约束。
- CLI 帮助文本与错误文案专业化修正：task preview 全文中文化；file download 的 jq 批量下载示例兼容全量/limit 双模式输出形状；login 补充 NAZHI_OCR_BASE_URL / NAZHI_OCR_MODEL 可选变量披露。

### 文档

- README/源码指引对照前端源码全面复核修正：envelope 双层 code 判成功语义澄清、哨兵错误数量对齐源码、荣誉功能证据文件指向 performanceM.vue。
- 工程注释治理：清除全部审计编号标记（注释与测试标识符）、历史修复叙事、失效引用；doc comment 与实现一致性修正。

## [1.4.0] - 2026-08-24

### 新增

- SDK `PreviewSubmitPayload` / `PreviewEditPayload` 与 CLI `nazhi task preview [--edit]`：与 SubmitTask/EditCircle 共用 buildTaskPayload 组装链路、不发请求，如实暴露任务预设（circleTaskId/circleTypeId/dimensionId/hours/pictureList），空 address/orgName/level 保持空串不发明默认值；预览为纯组装不上传 ImagePaths。
- `types` 新增写实等级常量 TaskLevelNational..Grade（1..6）+ TaskLevelName、审核情况 CheckResultExcellent..Poor + CheckResultName，对齐原生字典 cateCode=23。
- `AddTypicalCasePayload.UnmarshalJSON` 兼容前端表单回传的 type/role/level 数字与 "1.0" 浮点格式（flexStringFromNumber），attachmentId 兼容空字符串。
- FetchTasks 迁移至 ParallelDims 泛型并发 helper（行为等价由回归测试锁定）；CLI 组装层收敛为 assembly 深 Module（ProcessScope 统一进程级资源管理）。
- 集成测试真读链路注入 OCR（env/Nazhi-auto 配置 fallback），SubmitTask HAR 场景改用动态生成图片夹具。
- 日志系统增强（全流程可追踪）：新增 pkg/logx 薄封装（基于 stdlib slog，零新依赖）提供 Level/Format/File 解析、脱敏与 traceId 上下文；CLI 新增 --log-level debug/info/warn/error、--log-format text/json、--log-file 路径三旗标及对应 NAZHI_LOG_LEVEL/FORMAT/FILE 环境变量，兼容旧 --verbose（等价 debug，仅当未显式传 --log-level 时生效）；--quiet 仅静默 stderr，文件仍落盘便于 CI 留痕；SDK 在 request.go 统一 HTTP 生命周期打点并经 context 透传到 auth/session/file 全链，错误按 ClassifyError 定级；敏感字段统一脱敏，验证码原文永不落地。

### 破坏性变更

- SDK 移除本地验证码识别器、相关模型/原生运行库及构建选项；所有 `Login` 调用方必须通过 `WithCustomOCR` 注入视觉识别器。CLI 默认使用硅基流动 Qwen3-Omni，纯 Go 构建不再需要 CGO 或额外模型文件。

### 文档

- 同步 README、CLI/SDK 分册、CI、Makefile 与 `CLAUDE.md`，明确验证码识别依赖注入契约和纯 Go 构建矩阵。

### 本轮审计与删除

- 前端源码复核后深度删除违规功能：移除 `ViolationRecord`/`ViolationType`、SDK 客户端方法、CLI 命令及专属测试；前端历史调用点不再属于当前 SDK 契约。
- 完成一次脱敏云端登录冒烟：CLI 使用本机运行时注入的 SiliconFlow Qwen3-Omni 密钥成功返回 200 envelope 和 token；密钥、账号和 token 未写入输出或仓库。

- CLI `nazhi honor update`：保留 SDK `UpdateHonor` 能力，对象 payload 走 `parseJSONObjectPayload`，自动空 typeName 反查（`GetHonorTypeOptions`）；典型案例批量删除 `nazhi typical-case delete-batch --payload '[1,2,3]'`：保留 SDK `DeleteBatchTypicalCase` 能力，纯 ID 数组 payload 校验非空/正整数。
- SDK `types.UserUpdateInput` 新增 `Birthday` 字段（对应前端 `updateMyInfo.birthday` 键）；`UpdateMyInfoStructured` 写入 wire key `birthday`，`Birthday` 优先、`BirthdayStr`（兼容旧调用）仅在 `Birthday` 为空时生效。SDK 原样透传，**不**做日期或时区转换（前端实际发送 ISO 8601 UTC）。
- CLI `nazhi self-eval grad-status` / `grad-submit`：透传前端毕业评价查询与提交。查询走 `QuerySelfGradEvaluationJSON` 保留 `dataMap.student_comment` / `isGrad` 原始字段；提交走 `SubmitSelfGradEvaluation` 单层 `{studentComment}`。
- CLI `nazhi honor levels --type-id`：透传 SDK `GetHonorLevel`，对齐前端按荣誉类型联动加载级别。
- CLI `nazhi honor type-options` / `level-options`：分别透传 SDK `GetHonorTypeOptions` 的 `dataList` 类型选项与 `GetHonorTypeForSelect` 的 `returnData` 通用等级选项，避免两种下拉语义混用。
- CLI `nazhi task dimensions`、`task circle-type --task-id`：`nazhi task dimensions` 透传 SDK `GetDimensions`；`task circle-type` 透传 SDK `GetCircleTypeByTaskID`，自动拒绝非正整数 `--task-id`，不发请求。
- CLI `nazhi circle types --dimension-id [--pid]`、`circle tasks --type-id`、`circle images [--page] [--page-size]`、`circle dict --cate-code`：分别透传 SDK `GetCircleTypes`/`GetCircleTasks`/`GetCircleImages`/`GetDictList`，正整数 flag 在非法时立即走参数错误路径（退出码 3）。
- CLI 登录可接入 Nazhi-auto 同款硅基流动 Qwen3-Omni：设置 `NAZHI_SILICONFLOW_API_KEY`（兼容 `NAZHI_OCR_API_KEY` / `SILICONFLOW_API_KEY`）后通过 `WithCustomOCR` 注入；密钥不入库。

### 测试

- 集成守卫 `TestNoRealPII` 抽出 `piiSkipDir` 辅助函数并新增 `TestPiiSkipDirSkipsNestedGitRepo` 回归：自动跳过嵌套 git 仓库（worktree / 子模块等），避免旧 worktree 中残留的早期 PII 夹具持续误报 `go test ./...`；主仓库 `.git`、经典 `vendor` / `node_modules` 跳过规则保持不变。

### 修复

- SDK `GetMyInfo` 的 `className` 后处理与前端 `userBox`、`modifyBox`、`header` 对齐：只移除首个“级”字，不再按 `gradeName` 删除前缀。
- CLI stdin payload 超过 16 MiB 时返回参数错误，避免读取限制造成静默截断。
- CLI 写实 payload 将 `level`、`checkResult`、`playRole` 的合法整数 number 规范为标准十进制代码字符串，同时继续拒绝小数、非有限值和溢出值。
- CLI `self-eval submit --payload=` 显式空值立即返回参数错误，不再误走 stdin/纯文本模式。
- 修复写实列表分页测试夹具的并发计数竞态，`go test -race` 不再因测试自身的共享计数器误报。

### 文档

- **docs 重组**：删除设计类单页（architecture / login-flow / OCR / env-vars / har-testing / migration_v2 / api-coverage）；SDK 按功能域分册（`docs/sdk/*.md`）；CLI 精简为单文件并并入环境变量速查
- 根 README / 文档中心仅链 CLI + SDK 分册
- **自动补全总表** [`docs/sdk/autofill.md`](docs/sdk/autofill.md)：对照源码列出 Login 按学号查学校、GetMyInfo 用学号补 schoolId/schoolName、写实 hours/元数据/图片、荣誉 typeName/name/score、典型案例 *Name、用户中文映射与禁止发明默认等；各域分册补「用户输入 vs SDK 自动」；CLI 增加「SDK 自动补全对照」节
- 文件域 SDK 文档对齐 `UploadFileResult` 的 `attachmentID` / `attachmentName` 输出，并将 `DownloadFile` 文案改为“跟随 HTTP 重定向”
- CLI 文件上传示例统一使用实际 flag：`nazhi file upload -f ...`

### 破坏性变更 (BREAKING)

- SDK 写实列表 `*JSON` 方法签名新增 `key string`：`GetSubmitted/Teacher/Withdrawn/PublicCirclesJSON`、`*LimitJSON` 及 `getCirclesJSON`/`getCirclesLimitJSON` 内部贯通关键字筛选（此前硬编码 `key=""`）
- SDK `GetHonorList` / `GetHonorListJSON` 签名新增 `key string`（此前硬编码 `&key=` 空值）
- SDK `CircleRecord` 结构化字段 JSON tag 对齐真实 API 混用命名（见「修复」）：依赖错误 snake_case tag（如 `img_list`/`is_my_self`）的调用方需改用真实键或字段访问
- `CircleRecord.IsMySelf bool` 重命名为 `IfMySelf int`（前端 `ifMySelf==1`）
- `GetTypicalCaseList` / `GetTypicalCaseListJSON` 增加可选 `status ...int`（默认 3=全部）；三参数旧调用仍兼容
- `CircleRecord.PlayRole` 类型由 `string` 改为 `PlayRoleCode`（JSON 数字/字符串均可解码；比较请用 `string(rec.PlayRole)` 或 `.String()`）
- 写实提交：空 `Address`/`OrgName` **不再**回落学校名；空 `Level` **不再**默认 `"5"`。依赖旧便利默认的调用方须显式传值

### 新增

- CLI `nazhi task submitted|done|teacher|public|withdrawn` 支持 `--key` 关键字筛选（含 Peek 总数路径）
- CLI `nazhi honor list` 支持 `--key` 关键字筛选
- CLI `nazhi typical-case list --status`（0 未审 / 1 通过 / 2 驳回 / 3 全部，默认 3）
- `TaskInput` / `TaskSubmitInput` / `TaskEditInput` 新增独立 `TermName` 字段（不再误用 `CircleDate` 填 `termName`）
- CLI 新增 `printParamError`：参数错误固定 `envelope.Error(400)` → 退出码 3
- `HonorRecord.Status int`：荣誉列表审核状态码（前端 `scope.row.status`）
- 常量 `TypicalCaseStatusPending/Approved/Rejected/All`
- `HonorType.Score` + snake tag；`AddHonorPayload.Score`（默认 0 随请求发出）
- `AddTypicalCase` / `UpdateTypicalCase`：空 `typeName`/`roleName`/`levelName` 时按 code 自动补全（对齐 classiccanter 下拉）

### 修复

- CLI 各现有对象型 `--payload` 入口拒绝顶层 `null`、数组等非对象 JSON，统一走参数错误路径（退出码 3），避免零值请求或错误请求发出
- `AddTypicalCasePayload.UnmarshalJSON` 遵循标准部分解码语义：缺失字段保留实例原值，仅对 JSON 明确提供的空 `attachmentId` 归一为 0；避免 SDK 调用方复用 payload 时已有字段被意外清零
- `AddTypicalCasePayload` 兼容前端初始 `attachmentId:""`：空字符串/null 归一为零值并省略，无附件的前端原始表单可直接提交
- `nazhi typical-case list` 默认 `--page-size` 从 20 调整为前端一致的 10
- 修正 `honor.go` 顶部 `deleteHonorById` 端点注释为真实 GET
- `httpDo` 对非 2xx 走 `classifyHTTPStatus`，主业务路径可识别 429/5xx/4xx 哨兵错误
- `hasHostSuffix` 要求 exact 或以 `.`+suffix 结尾，防止 `evilnazhisoft.com` 受信绕过
- `assembleCirclesJSON` 空首页不再产生 leading comma 非法 JSON
- `getCirclesLimitJSON` 只请求 offset/limit 覆盖页，避免全量翻页再截断
- `FetchTasksJSON` cancel 路径对齐 `ErrRetryable`（与 `FetchTasks` 对称）
- `parseHours` 非法输入返回 `ErrInvalidPayload`，不再静默回退 meta.Hours
- `TaskAddCirclePayload.ID` 加 `omitempty`，新增写实不再发 `"id":null`
- `UpdateMyInfo` 成功后失效 `sm.cachedUserInfo`，避免同进程 `GetMyInfo` 返回更新前快照；新增 `InvalidateCachedUserInfo`
- `nazhi user update` 解析 `UserUpdateInput` 并调用 `UpdateMyInfoStructured`，友好键（genderName 等）正确 remap
- `QuerySelfEvaluation` 未提交评价时返回 `(nil, nil)`，不再把空成功误判为「所有解码器均失败」
- `UpdateHonor` 对称补全 `typeName`（与 `AddHonor` 共用 ensureHonorTypeName）
- `AddHonor` 空 `Name` 时回落 `TypeName`（对齐前端新增表单不传 name）
- `AddHonorPayload.UnmarshalJSON` 部分解码时保留未提供字段，证书附件 ID 继续兼容 number/string，并区分缺失与显式 null
- `GetCircleTypes` 对 `pid` 做 `url.QueryEscape`
- 历史识别并发 Option 不再覆盖 `WithCustomOCR` 注入的识别器
- 参数错误改用 `printParamError(400)`→exit 3（缺 token / payload 解析失败等）
- 写实列表 `Get*CirclesJSON` 部分页失败时输出 `envelope.Partial(207)` 保留已合并数据
- **CircleRecord 混用命名解析**：`imgList`/`imgPreViewList`/`commentList`/`likeStatus`/`ifMySelf`/`auditRemark`/`creationTimeStr`/`showName`/`imgPath`/`studentId` 对齐平台真实 JSON（此前 snake_case tag 导致结构化 API 静默丢字段；CLI `*JSON` 透传不受影响）
- **GetTypicalCaseList 注释与能力**：status=3 为前端「全部」而非「已提交」；支持按审核状态筛选
- **HonorType JSON tag**：`dimension_name`/`level_name`（前端德育说明表；此前 camel 导致空字段）
- **UpdateMyInfoStructured**：忽略 `NationalStudentNumber`（前端只读，防止误写学籍）
- **输入暴露原则**：用户手填字段进 Input；前端自动填字段由 SDK 补全（典型案例 *Name、荣誉 typeName/name/score）
- **CircleRecord.PlayRole**：类型改为 `PlayRoleCode`，兼容列表 API 的 number 与表单 string（前端 `switch(map.play_role) case 1/2/3`）；序列化统一为字符串码
- **SelfEvalStatus**：`UnmarshalJSON` 主解码 `student_comment`/`teacher_comment`（前端 mainLeft/selfgaintloss），兼容 camelCase；`QuerySelfEvaluation` 的 normalize 兜底仍保留
- **荣誉/自评协议文档**：补充荣誉各方法的真实 HTTP method/path、删除荣誉的 GET+`id` 查询参数，以及自评/毕业评价的 endpoint、请求体层级、`dataMap` 字段和当前 CLI 能力边界
- **荣誉/自评回归测试**：覆盖 `DeleteHonor` 的 GET+`id` query、结构化自评的双层 `studentComment` JSON 请求体，以及荣誉证书附件 ID 的 number/string 输入兼容
- **parseHours / TaskSubmitInput.Hours**：对齐前端 `hoursStatus`——任务元数据 hours>0 时用户可空（SDK 用预设）；hours≤0 且用户空 → `ErrInvalidPayload`（不再静默提交 0）；显式 Hours 始终优先
- **写实 Address/OrgName/Level**：去掉 SDK 发明的默认（空 Address/OrgName→学校名、空 Level→`"5"`）；与前端一致，空串原样提交；调用方须按活动类型自行填写
- **典型案例 *Name 映射**：对齐 classiccanter el-option——type `"2"`→「社会调查报告」、level `"1"`→「国际」（此前误为「社会实践报告」/「国家」）
- **UpdateTypicalCase 数字 code**：`fillTypicalCaseDisplayNamesMap` 支持 type/role/level 为 number 或 string（列表回填常为 number；此前仅 string 能自动补 *Name）
- **写实 CLI payload 类型兼容**：CLI 私有 JSON 解码 helper 兼容前端编辑回填的 `hours` number/string（保留小数），并仅接受 `level`、`checkResult`、`playRole` 的有限整数 number；可接受数字在进入 client 前统一为提交字段使用的字符串，字符串按原值保留。同时兼容 `circleTaskId` → `taskId`、`pictureList` → `imageIDs`，规范字段优先；`TaskSubmitInput` / `TaskEditInput` 公开 Go 字段和标准 JSON 解码语义保持不变
- **写实提交默认行为说明**：历史版本中的 Address/OrgName 学校名回落与 Level 默认 `"5"` 仅代表旧行为；当前版本保持前端语义，空值不自动替换

## [1.3.0] - 2026-07-18

### 新增

#### 类型定义扩充

- `CircleRecord` 扩充 30+ 字段：补齐前端 getStudentCircle 所有原始字段（hostName、rank、level、checkResult、patentType、activityName、sportsName、teamName、orgName、resultsName、obtainTime、specialtyTechnology、playRole、likeSpecialty1-3、operatorName、creationTimeStr、circleTaskName、showName、isMySelf、auditRemark、likeStatus、commentList 等）
- `CircleComment` — 新增写实评论类型
- `UserInfo` 扩充 8 字段：telephone、genderName、birthdayStr、youthLeagueFlag、nation、familyAddress、hobbies、idCard、idType
- `ExamResult`、`TermInfo`、`ExamInitInfo`、`ExamType`、`Course` — 新增成绩管理类型
- `ViolationRecord`、`ViolationType` — 历史版本曾新增的违规类型，当前版本已删除
- `Notification`、`NotificationListResult` — 新增通知消息类型
- `BonusInfo`、`BonusRank`、`BonusDetail` — 新增积分商城类型
- `DemocraticActivity`、`SelfEvaluationItem`、`MutualEvaluation`、`DemocraticResult`、`MutualPersonInfo`、`ClassStudent` — 新增民主评价类型

#### 新增 SDK 方法（8 个新文件）

- `circle.go` — 写实管理扩展：DeleteCircle、AddCircleComment、SetCircleLike、GetCircleImages、GetCircleTasks、GetCircleTypes、GetDimensionsBySchool、GetDictList
- `exam.go` — 成绩管理：GetExamInitInfo、QueryStudentExam（**v1.3.0 已删除，不再维护**）
- `democratic.go` — 民主评价：GetDemocraticActivities、GetDemocraticActivityByID、GetSelfEvaluationData、GetMutualPersonInfo、GetDemocraticResult、GetMutualEvaluationDetail、AddOrUpdateSelfEvaluation、AddOrUpdateMutualEvaluation（**v1.3.0 已删除，不再维护**）
- `violation.go` — 历史版本的违规记录实现，当前版本已删除文件、方法和测试
- `notification.go` — 通知管理：GetUnreadNotifications、GetNotificationByID、ReadNotification、GetAllNotifications（**v1.3.0 已删除，不再维护**）
- `bonus.go` — 积分商城：GetMonthBonus、GetHistoryBonus、GetBonusRank、GetBonusDetail（**v1.3.0 已删除，不再维护**）
- `file_bag.go` — 档案查看：GetTermList、GetStudentInfoForTerm（**v1.3.0 已删除，不再维护**）
- `user_update.go` — 个人信息更新：UpdateMyInfo

#### 新增 CLI 命令（6 个父命令）

- `nazhi circle` — 写实管理（delete、comment、like）
- `nazhi exam` — 成绩管理（query）（**v1.3.0 已删除，不再维护**）
- `nazhi violation` — 历史版本的违规查询命令，当前版本已删除命令树和专属测试
- `nazhi notification` — 通知管理（unread、read）（**v1.3.0 已删除，不再维护**）
- `nazhi bonus` — 积分管理（month、rank）（**v1.3.0 已删除，不再维护**）
- `nazhi user` — 用户管理（update）

### 构建

- 版本号：`1.3.0`

## [1.2.4] - 2026-07-18

### 新增

- SDK `EditCircle` — 修改已提交的写实记录
- CLI `nazhi task edit` — 修改已提交的写实记录
- SDK `TaskEditInput` — 修改写实记录的最小输入（与 TaskSubmitInput 对齐，新增 id 字段）

### 构建

- 版本号：`1.2.4`

## [1.2.3] - 2026-07-18

### 新增

- SDK `GetTeacherCircles` / `GetTeacherCirclesJSON` / `GetTeacherCirclesLimitJSON` — 获取教师代写的写实记录（type=2）
- SDK `GetWithdrawnCircles` / `GetWithdrawnCirclesJSON` / `GetWithdrawnCirclesLimitJSON` — 获取被撤回的写实记录（type=3）
- SDK `GetPublicCircles` / `GetPublicCirclesJSON` / `GetPublicCirclesLimitJSON` — 获取公示的写实记录（type=4，全班）
- SDK `PeekTeacherTotal` / `PeekWithdrawnTotal` / `PeekPublicTotal` — 轻量获取对应类型记录总数
- CLI `nazhi task teacher` — 获取教师代写的写实记录
- CLI `nazhi task withdrawn` — 获取被撤回的写实记录
- CLI `nazhi task public` — 获取公示的写实记录（全班）
- SDK `EditCircle` — 修改已提交的写实记录
- CLI `nazhi task edit` — 修改已提交的写实记录

### 重构

- `pkg/client/submitted.go` 提取通用 `fetchCirclePage` / `fetchCirclePageJSON` 辅助函数，支持按 `type` 参数查询不同类别的写实记录
- 原有 `GetSubmittedCircles` / `PeekSubmittedTotal` / `GetSubmittedCirclesJSON` / `GetSubmittedCirclesLimitJSON` 保持向后兼容

### 文档

- `docs/cli/README.md` 命令树 + 新增命令详细文档
- `docs/sdk/README.md` 方法签名表 + CLI 输出对照表更新

### 构建

- 版本号：`1.2.3`

## [1.2.2] - 2026-07-17

### 新增

- SDK `PeekSubmittedTotal` — 轻量获取已提交写实记录总数（内部 `pageNo=1&pageSize=1`，只拉 1 条）
- CLI `task submitted --count` / `task done --count` 改用 `PeekSubmittedTotal`，不再经过 `GetSubmittedCirclesLimitJSON`

### 文档

- `docs/sdk/README.md` 方法表 + 完整文档小节 + 代码示例

### 测试

- 4 个单元测试覆盖正常/零数据/业务错误/请求参数验证

### 构建

- 版本号：`1.2.2`

## [1.2.1] - 2026-07-17

### 新增

- `typical-case submit` CLI 命令 — 提交一条典型案例，payload 支持 `@file.json` 和 `-`（stdin）两种来源
- `typical-case list` CLI 命令 — 获取当前用户已提交的典型案例记录（分页），CLI 透传 SDK 原始 JSON 1:1 对齐
- SDK `AddTypicalCase` — 调用 `/api/studentCircleNew/addTypicalCase` 提交典型案例
- SDK `GetTypicalCaseList` / `GetTypicalCaseListJSON` — 调用 `/api/studentCircleNew/getTypicalCase` 获取已提交列表

### 类型

- `AddTypicalCasePayload` — 13 字段提交请求体（HAR 确认 type/role/level 为 JSON 字符串）
- `TypicalCaseRecord` — 16 字段列表记录（与提交 payload 不同的 Go 类型，列表响应中 type/role/level 为整数）
- `TypicalCaseListResult` — 列表统一返回对象（Records + Page）
- `TypicalCaseRoleHost` / `TypicalCaseRoleParticipant` 角色常量

### 构建

- 版本号：`1.2.1`

## [1.2.0] - 2026-07-15

### 特性

- `task list` 输出扩展：Task 结构体新增 18 个服务端原始字段（全部 omitempty，完美兼容已有输出）
- `TaskSubmitInput` 新增 14 个可选字段，暴露前端 addCircle 请求体全部参数（零值空串时保持原有 fallback 行为）

### SDK

- `Task` 新增字段：`schoolId`, `circleTypeId`, `creator`, `modifier`, `modifyTime`, `roleId`, `auditorSubjectId`, `stateType`, `areaId`, `areaTaskId`, `upPic`, `evaluatedNumber`, `unEvaluatedNumber`, `unsubmittedNumber`, `submitNumber`, `pictureList`, `classId`, `gradeId`
- `TaskSubmitInput` 新增字段：`Name`, `HostName`, `CircleDate`, `Rank`, `ActivityName`, `SportsName`, `TeamName`, `OrgName`, `ResultsName`, `ObtainTime`, `SpecialtyTechnology`, `LikeSpecialty1~3`
- `buildTaskSubmitPayload` 将新输入字段映射到 `TaskAddCirclePayload`，`OrgName` 空串时 fallback 学校名

### 文档

- docs/cli/README.md：task list 字段计数更新为 40 字段，新增 v1.2.0 变更说明段落
- docs/sdk/README.md：TaskSubmitInput 代码示例补充新字段展示

### 重构（BREAKING）

- **OCR 移除 primary/fallback 级联退避**：`c.ocr` 作为唯一识别通道，不再分 primary 降级两道循环。`ocrRecognizeWithRetry` 从 3 返回值改为 2（移除 `fallbackUsed`）。`LoginResponse` 移除 `FallbackUsed` 字段。`buildLoginResponse` 移除 `fallbackUsed` 参数。

### 移除

- 历史后备识别 Option 不再有两阶段 cascade
- `Client.fallbackOCR` / `Client.fallbackConc` 字段 — `Client` 结构体减负
- `defaultFallbackOCR` / `safeFallbackRecognize` — 死代码删除
- `ocr_fallback_test.go` — 5 个 cascade 测试全部删除
- `LoginResponse.FallbackUsed` 字段 — `LoginResponse` 精简到 2 字段

### 文档

- 全量文档同步 v1.2.0：architecture.md 架构决策 #5 移除 fallback 级联描述；Option 表删除 fallback 两行；Login 流程示例更新（移除 fallbackUsed）；login-flow.md LoginResponse 结构体同步；CLI/SDK 参考的登录响应示例更新；版本号全量同步至 v1.2.0。

### 构建

- 版本号：`1.2.0`

## [1.1.5] - 2026-07-12

### 文档

- task submitted / GetSubmittedCircles 示例更换为真实脱敏 JSON，注明含同班同学姓名/学号
- CLI 描述改为"含同班同学的姓名和学号"
- SDK 描述改为"含姓名、学号、正文、图片、审核状态"
- docs/README.md 版本表同步

## [1.1.4] - 2026-07-12

### 文档

- 全量版本号同步至 v1.1.4（README.md / docs/README.md / docs/cli/README.md 版本表、徽章、命令概述）
- docs/sdk/README.md CLI↔SDK 对应表修正：task list 改回 `FetchTasks`（非 `FetchTasksJSON`）
- 方法速查表新增 `GetSubmittedCirclesLimitJSON`

### 清理

- 移除误跟踪的 `test_get_school_id.go`（`go:build ignore` 调试文件）
- 补充 `.gitignore` 防止再次误跟踪

## [1.1.3] - 2026-07-12

版本号占位。未单独发布变更。

## [1.1.2] - 2026-07-12

### 特性

- `task submitted` 新增 `--limit` / `--offset` / `--count` 分页控制
- SDK 新增 `GetSubmittedCirclesLimitJSON(ctx, token, offset, limit)` 方法
- `--limit` + `--offset` 超出实际数据量时返回空数组，不报错
- `--count` 只拉第一页获取 TotalNum，不拉列表数据

### SDK

- 新增 `GetSubmittedCirclesLimitJSON` — 用深度扫描逐条分割 JSON 数组，不反序列化为 Go struct
- 翻了够 offset+limit 就停，不等 totalPage 全部翻完

## [1.1.1] - 2026-07-11

### 修复

- task list 恢复输出 SDK 最终业务模型（`FetchTasks`）而非原始 JSON（`FetchTasksJSON`），
  字段重新包含 `submitted` / `needPic` / `dimensionName` 等业务语义字段
- 修复 `FetchTasksJSON` 聚合时 `dimErrs` 的并发写入 data race
- 修复 `FetchTasksJSON` 跳过 `id=0` 维度后收发次数不一致导致卡死
- 修复 `FetchTasksJSON` raw JSON 拼接时多余逗号导致序列化失败

## [1.0.0] - 2026-07-10

重大破坏性更新：types 全面精简 + JSON tag 统一 camelCase + CLI 输出 envelope 化。
升级前请阅读 [MIGRATION.md](MIGRATION.md)。

### Breaking Changes

- types: UserInfo 51→10 字段 (删除 initials/pinyin/seat/gender/birthday/telephone/creationTime 等 41 字段)
- types: Task 18→11 字段 + 新增 ScopeClass/ScopeGrade/ScopeStage 常量 (删除 upPic/pushNum/score/creatorName/roleName/termID)
- types: CircleRecord 15→9 字段, CircleImage 5→1 字段 (Approved bool + 仅 AttachmentID)
- types: HonorType 8→5 字段, HonorRecord 17→9 字段 (approved bool 替代 status int)
- types: SelfEvalStatus 10→3 字段 (id + studentComment + teacherComment)
- types: LoginResponse 删 RawData 字段 (3 字段)
- 命名: 全 SDK 统一 camelCase JSON tag
- 时间: 时间字段改 time.Time (ISO 8601 + 时区序列化)
- CLI: 删 `nazhi school` 命令 (从 UserInfo 获取)
- CLI: 新增 `nazhi task done` 别名 (替代 `task submitted`)
- CLI: 退出码三分 (0/1/2/3)

### Added

- pkg/envelope/envelope.go (统一 envelope 包装)
- pkg/client/internal/convert.go (helper: MapTaskStatus/MapSchoolID/ParseServerDate/MapCircleApproved)
- ScopeClass/ScopeGrade/ScopeStage 常量
- SDK `DownloadFile(ctx, attachmentID, dst)` — 按附件 ID 下载图片到本地。
  入口 `ssoBaseURL/common/attachment/getImg?id=X`，跟随 302 到 FastDFS 真实存储；
  CheckRedirect 同域白名单（nazhisoft.com）+ 5 次上限；不发任何鉴权头。
- CLI `nazhi file download` — 按附件 ID 下载图片。
  ```
  nazhi file download --id 5006375 --output ./photo.jpg
  ```
  不接受 `--token`（公开服务）；urlType=`sso` 走 SSO 域名。

### Removed

- types/score 字段 (Task/HonorType/HonorRecord)
- types/initials/pinyin 字段 (UserInfo)
- 全部 omitempty 派生字段 (UserInfo seatSort/telephone/email 等)
- 重复字段 (UserInfo.studentName/SelfEvalStatus.schoolId 等与 UserInfo 重复)
- nazhi school CLI 命令

### Changed

- UserInfo/Task/CircleRecord/HonorRecord/SelfEvalStatus 字段重命名 (status→submitted/approved)
- circleTaskStatus 字符串 → submitted bool (简化版)
- circleDate/getDate 字符串 → time.Time (自动序列化 ISO 8601)
- `GetMyInfo` 学校信息 SSO 降级条件放宽 — 原来仅 `schoolId==0` 时触发，现放宽到 `schoolId==0 || schoolName==""` 任一缺失时通过 `GetSchoolID` 公开 API 补全。学校名缺省但 ID 存在时也能自动补上学校名称。

### 文档

- 全量脱敏示例刷新 — `docs/cli/README.md` 与 `docs/sdk/README.md` 全部 CLI 命令和 SDK 方法的输出示例替换为真实测试验证后的脱敏响应（含 login / session activate / whoami / task list / task submit / task submitted / self-eval / honor / file 全系列）。

### 修复

- Windows flaky timing 测试：3 个 `TestFetchTasks_*` 在 Windows 慢机器偶发失败
  - `TestFetchTasks_MixedBizAndCancel_FailedCountAccurate`: ctx 500ms → 1.5s
  - `TestFetchTasks_ContextCancel_ReturnsErrBusinessRejected`: ctx 1s → 1.5s
  - `TestFetchTasks_Parallel`: bounds 重写为 `warmupOverhead+perDimDelay+slack`
    （warmupOverhead=800ms，反映 Windows session warmup 真实耗时）
  - handler sleep 300ms → 2s（确保 dim 必被 ctx cancel 而非正常完成）
  30 次连跑零失败。

### 清理

- `.golangci.yml` 精简 — 移除 godot/godox linter（减少 lint 噪音），清理注释冗余。
- `errors.As` 替换裸类型断言 — `switch e := err.(type)` 改为 `errors.As`，兼容 wrapped error。
- switch exhaustive 补全 — `image_prep` 中 `format` switch 补 `default` 分支防漏。

## [0.6.0] - 2026-07-04

### 特性

- OCR primary+fallback 双策略降级 — primary OCR 全部失败后自动降级到内置 ddddocr 重新识别新图，提高验证码识别成功率（v0.5.0 引入，本篇正式记录）。

### 修复

- ocr_fallback_test.go 格式修复 — `gofmt` 对齐不一致导致 lint 失败。

### 构建

- 版本号: `0.6.0`

## [0.5.2] - 2026-07-04

### 修复

- validateCaptcha 重试修复 — 之前 OCR 识别成功后只调一次 `validateCaptcha`，校验失败直接退出 Login，浪费剩下 8 次重试预算。修复后将 `validateCaptcha` 移入 `ocrRecognizeWithRetry` 循环内部，校验失败时 `continue` 换图重试，用完 9 张图预算为止。错误链保持 `errors.Is(err, ErrLoginRejected)` 兼容性。`Login()` 不再单独调 `validateCaptcha`，外层流程精简。
- warnIfExpiresAtFallback nil 守卫 — 两处 `c.logger.Warn` 未检查 `c.logger` 是否为 nil，可能导致 nil 指针 panic。
- GetSubmittedCircles ctx 取消返回 error — 翻页过程中 context 取消时，之前返回 `(all, nil)` 掩盖错误，改为 `(all, err)` 让调用方感知截断。
- self-eval ReadString 错误传播 — `ReadString(0)` 的 error 被 `_` 丢弃，真实 I/O 错误被掩盖。
- honor 死代码删除 — `AddHonor` 中 `_ = resp` 无意义，改为 `_, err :=`。
- GetSubmittedCircles merge 误改修复 — 正常路径应返回 nil error，merge 冲突错误地改成了 `return all, err`。

### 重构

- cmd payload 抽取公共 `parsePayloadFromArg` — task_submit/honor 重复的 `@file.json`/stdin 读取逻辑归一到同一 helper。
- opt_builder switch case 替换为 map 查找 — 3 处相同的 switch 结构简化为一次 map 构建 + O(1) 查找。

### 清理

- OCR/types 注释与参数清理 — NewPool preload 废弃标记、BusinessError 注释代码示例清理、LoginResponse 死字段历史注释清理。

### 测试

- TestGetSubmittedCircles_CancelDuringPaging 竞态修复 — 同步点从 page 1 handler 响应后移到 page 2 handler 被调用时，消除 goroutine 调度不确定性。

### 构建

- 版本号: `0.5.2`
- .gitignore: 补充 `.git-rewrite/`、``、`angle5-findings.json` 忽略规则。

## [0.5.1] - 2026-07-03

### 修复

- commit 消息 @ 前缀违规 — `feat(task):` 的 commit 消息以 `@` 开头，违反 Conventional Commits 约束，changelog 生成器解析异常。rebase 修复。
- honor.go 注释缩进 — `deleteHonorById` 行多一个制表符缩进，已对齐。
- CLAUDE.md 版本/OCR 参数同步 — 仍引用 v0.4.1 版本号和 `maxOCRImagesTotal=99`，更新为 v0.5.0 和 9。

## [0.5.0] - 2026-07-03

40 个 commit，自 v0.4.1 以来的完整变更。

### 新增

- 荣誉申报 SDK（honor.go） — 5 个方法：GetHonorTypes / GetHonorTypeForSelect / GetHonorLevel / GetHonorList / AddHonor。通过 TDD 驱动开发，11 个单元测试全部通过
- nazhi honor CLI 命令 — `nazhi honor types` / `nazhi honor list` / `nazhi honor add` 三个子命令，支持 `@file.json` 和 `-`（stdin）两种 payload 来源
- nazhi task submitted CLI 命令 — 调用 `GetSubmittedCircles` 获取已提交写实记录，自动翻页合并输出
- `--payload -`（stdin 读取） — task submit 和 honor add 都支持从 stdin 读取请求体 JSON

### 改进

- docs SDK 参考 — 新增 submitted.go 和 honor.go 完整章节正文（含代码示例、错误说明、分页策略）
- docs CLI 参考 — 新增 `nazhi task submitted` 和 `nazhi honor {types,list,add}` 完整文档章节
- 文档同步 — README.md / docs/README.md / docs/architecture.md / docs/env-vars.md 同步 honor + submitted 相关内容
- `parsePayload` 抽取 — task_submit.go 将从文件/从 stdin 读取 payload 的逻辑抽取为独立 helper，honor add 复用相同模式

## [0.4.1] - 2026-07-02


### 新增

- `parallel.go` — CLAUDE.md 候选 #6：`ParallelDims[T any]` 泛型并发维度查询 helper，含 `ParallelDimsResult` + 错误聚合。81 行（vendor 化），`FetchTasks` 后续可迁移
- `error_category.go` — CLAUDE.md 候选 #7：`ClassifyError(err) ErrorCategory` 枚举（`ContextCancel` / `ContextTimeout` / `NetworkTimeout` / `BusinessError` / `Unknown`）。80 行，`task.go` `isContextError` 已使用
- `internal/recoverx` 包 — 统一 panic recover 策略，`RecoverPanic(recovered, sentinel, name)` 输出 `debug.Stack()` 到 stderr。auth.go / session.go / main.go 3 处调用点全部收敛
- `tokenparse` 3 个哨兵错误 — `ErrTokenReturnDataEmpty` / `ErrTokenTypeMismatch` / `ErrTokenFieldMissing`。`ExtractFromReturnData` 调用方可精确区分 3 种解析失败
- 5 个 HTTP 状态码哨兵错误：`ErrRateLimited`（429）/ `ErrServiceUnavailable`（5xx）/ `ErrTimeout`（超时）/ `ErrInvalidResponse`（4xx-其他）/ `ErrRetryable`（ctx cancel 可重试）。SDK 用户通过 `errors.Is` 精确识别 HTTP 层 / 业务层错误
- `doBizGet` 按 StatusCode 自动包装 sentinel：429 → `ErrRateLimited`，5xx → `ErrServiceUnavailable`，4xx → `ErrInvalidResponse`，不再笼统 "ErrNetwork"
- `isTimeoutError` helper：`c.do` 内部识别 `context.DeadlineExceeded` / `*url.Error.Timeout()` / `net.OpError.Timeout()`，用 `ErrTimeout` 包装

### 修复

- 顶层 panic recover exit code 1 回归 — 之前 `pendingExitCode=0` 走 exit 0，与正常 error 不一致。修复 `printError` 递归 fallback 设 `pendingExitCode=1`
- OCR Pool 加 `sweepStaleTempDirs` 启动时清扫 — `nazhi login` 顺手 best-effort 扫 `%TEMP%` 历史残留，能删的删
- `fetchTasksForDimension` panic recover 错误链保留 — `defer recover` 用 `%w` 包装原始 error
- `SetLimit(0)` 死代码修复 — `errgroup.SetLimit(0)` 在 `len(dimensions)==0` 时死路径，调最小为 1
- PII 守卫 AST 自检盲区修复 — 字符串拼接绕过的扫描覆盖
- `task list` cancelledCount 虚高修复 — 占位 error 不计入 `failedCount`
- `cookie_sync` partial decode 防御 — `dec.More()` 检查 reader 残留，partial 时 RawData 置 nil
- `ErrFileTooLarge` 错误链修复 — `errors.Join(ErrFileTooLarge, ErrImageTooLarge)`，errors.Is 单一识别所有"文件过大"路径
- `Login` body 摘要 — 非预期状态码错误消息附 `logSafeBody(bodyBytes)` 100 字节截断

- 注释中文化 — magic bytes sniff、`multipartBufPool` Grow 等
- `getQualitySteps` 内联为 `qualityAfterOptimization` 常量
- 结构化日志 — `warnIfExpiresAtFallback` 改为结构化 slog 字段
- `logSafeBody` 提取变量消除重复
- `isContextError` helper 消除 3 处重复
- self_eval 空值兜底 guard 删除（无消费者）

- `ErrTimeout` 包装 — `isTimeoutError` 出口处用 `ErrTimeout` 包装而非裸 fmt.Errorf
- `doBizGet` 包装 body 摘要 + ErrInvalidResponse fallback
- `atomic.Pointer[url.URL]` race 修复 — `c.baseURLParsed` 全部访问原子化

- `image_prep` 缩放级联简化 — 7 轮 resize 改为单次缩放（0.7^7 ≈ 0.082 常量计算），`getScaleFactors` 删除
- `decodeImage` 改用 `image.Decode` — 删除手写 magic bytes switch，stdlib 自动识别格式
- `prepareImageForUpload` 加 `ctx` 参数 — 支持超时取消
- `defaultOCR` 惰性预热 — 从同步 `NewPool(min(NumCPU,4))` 改为 `sync.Once` + `atomic.Pointer` 懒加载
- `Close()` 清理 sessionManager backoff 状态 — 避免复用 Client 时误触发冷却
- `New()` `url.Parse` 静默吞错改为 warn 日志
- `withURLGuard`/`withNilGuard` Option 工厂 — 消除 6 处 Option 重复守卫逻辑
- `--timeout 0` warn 回退 — 之前静默覆盖为正数超时，现在 warn 并保留默认值
- `valueToString` float64 精度保留 — 改用 `FormatFloat` 替代 `FormatInt` 截断
- `writeModelFile` 失败走 `cleanupTempDir` — 复用 DLL 占用降级逻辑
- `sweepStaleTempDirs` case-insensitive FS 下 `EqualFold` — 避免 Windows 大小写误判
- `initOnce` panic `%v` → `%w` — 保留 error chain
- `tryDecodeFallback` 删除 — 被 `doBizGetDecode` 吸收，不再需泛型 fallback helper
- `maxOCRAttemptsPerImage` 常量删除 — 设计意图代之以代码注释（架构深化后单图 OCR 1 次策略已稳定）
- `getScaleFactors` 删除
- `countTasksByType` `int` → `float64` — 泛型 `sumValues[T int | float64]` 改为 `T int64` + `json.Number` 兼容
- `Close()` 末尾清 `sm.clearBackoff()` 
- `maps.Clone` 删除 — `doGetMenu` 直接修改 map 而非 Clone，减少一次分配
- DCL fast path `cachedUserInfo` nil guard

### 改进

- `c.logger.Warn` 资源警告统一走用户注入 slog — 不依赖 cmd 通道，SDK 纯净
- `go.mod` 模块单一 — 仓库只有一个 `module github.com/Wenaixi/nazhi-cli`
- 文档全面升级到 v0.4.1 — CLI / SDK / 架构 / 登录流程 / OCR / HAR / 环境变量全部同步

### 构建

- 版本号：`0.4.1`
- make build 仍缺 `-tags=ddddocr`（已知坑不变）
- 新增 `internal/recoverx` 包 — 零依赖，无测试文件（由 3 个客户端隐式覆盖）

## [0.4.0] - 2026-06-30

v0.3.5 → v0.4.0 之间合入 305 个 commit，172 个文件改动。

### 新增

- session 激活 4 入口收口为 1 个公开方法 + 1 个内部 fast-path；`sessionManager` 封装 `SetBackoff`（d≤0 守卫）与 `tryActivate`
- HTTP helper 私有化（`doRequest` → `httpDo`、`doRequestWithResp` → `rawDoWithResp`），公开 API 保持不变
- `pkg/types/response.go` 新增 `DecodeUnified()` 原语（组合 `DecodeResponse` + `CheckCode`）
- 新建 `pkg/tokenparse/` 包封装 SSO token 解析（Location 头 + returnData）；泛型 `DerefOr[T]` 升到 `pkg/types/deref.go`；auth.go 瘦身约 40%
- `extractModels` 建好本进程目录后扫一遍 `%TEMP%` 下其他 `nazhi-cli-ocr-*` 残留，能删的删（已退出进程）、删不动的跳过（其他运行实例），绝不误删其他程序目录。Windows 登录后 `%TEMP%` 不再无限堆积

### 修复

OCR Windows 三轮 TDD 修复：

- `5ff0ea8` Windows DLL 占用降级：`Close` 时删 `onnxruntime.dll` 因 `LoadLibrary` 句柄未释放被拒（`Access is denied`），抽 `cleanupTempDir` 对 Windows 两类 errno（`ERROR_ACCESS_DENIED` / `ERROR_SHARING_VIOLATION`）降级返 nil。stderr 不再被权限错误污染
- `a81c9f3` GOOS 守卫：上一轮注释承诺「非 Windows 永远 false」但代码不保证（Linux errno 5=EIO、32=EPIPE 也会命中），加 `goosFn` 注入点 + `runtime.GOOS == "windows"` 守卫，降级只在 Windows 生效
- `7d5dd65` 启动时清扫：见新增段最后一条

`SetBackoff` race 修复；`main` panic recover 走 `closeAllClients` LIFO；`--output` 死代码删除；`Login` 并发（CallStep 改 mutex）；顶层 panic recover 输出 `debug.Stack()` 到 stderr；`buildLoginResponse` 非法 JSON 保底 `RawData` 为空 map（Finding #8）+ RawData 置 nil 消除二次解析；`image_prep` 缩放级联优化（resize N 次后统一 encode）；Transport 加 `TLSHandshakeTimeout=10s` 防网络挂起；`tryActivate` 先检查 `ctx.Err()` 再检查 backoff（避免被掩盖）；`bizURL` helper 集中处理裸 baseURL 拼接；`noRedirect` 共享变量；OCR Pool `o.mu` panic defer Unlock 防死锁；引用已删接口的文档清理；中文注释规范化。

### 兼容性

- `client.New(...)` 仍返 `(*Client, error)`（v0.3.1 起的契约不变）
- `Login` / `ActivateSession` / `FetchTasks` / `SubmitTask` / `SubmitSelfEvaluation` / `QuerySelfEvaluation` / `GetMyInfo` / `UploadFile` / `GetSchoolID` 业务方法签名与 v0.3.5 完全一致
- 环境变量清单（`NAZHI_USERNAME` / `NAZHI_PASSWORD` / `NAZHI_TOKEN` / `NAZHI_SSO_BASE` / `NAZHI_BASE_URL` / `NAZHI_UPLOAD_URL` / `NAZHI_TIMEOUT`）与 v0.3.5 一致
- `file upload` 仍不接受 `--token`（独立公共服务，不需要业务域 token）

### 构建

- 版本号：`0.4.0`
- 所有路径显式 `-tags=ddddocr`；Makefile `build` target 仍缺 tag（已知坑，需手动 `go build -tags=ddddocr`；CI 已正确）
- 测试：45+ OCR 测试 race + ddddocr 双 tag 全绿；新增 `ocr_win_cleanup_test.go` / `ocr_sweep_test.go`
- 跨平台：5 平台（linux/darwin/windows × amd64/arm64）vet 通过；macOS x86_64 不支持（Microsoft 已停发）

## [0.3.5] - 2026-06-26

### Features

- OCR 可选构建 — 新增 `-tags ddddocr` 编译标签。不加标签时编译为纯 Go 二进制（无 CGO），`login` 命令会返回明确提示指导使用 `WithCustomOCR` 或下载预编译 release。CGO-free 嵌入式场景不再被 onnxruntime 强制依赖阻塞。
- 新增 3 个错误哨兵 — `ErrOCRNotConfigured`、`ErrEmptyUserInfo`、`ErrSessionBackoff`。SDK 用户可用 `errors.Is` 精确区分 OCR 缺失、空用户信息、session 背压三种场景。

### Fixed

- 文件上传 multipart 缺少终止边界 — 修复 upload 请求体尾部缺 `--boundary--\r\n` 导致服务端解析失败。
- GIF 上传背景变黑 — 修复透明 GIF 合成白底时走特殊路径导致的回归。
- 图片压缩失败死循环 — 修复 JPEG 编码失败时无限重试导致 CPU 100%。
- CLI 退出泄漏 ONNX 资源 — 修复 `os.Exit(1)` 跳过 defer 导致临时目录永久残留。
- 上传命令误读 NAZHI_TOKEN — 修复 `file upload` 将 `NAZHI_TOKEN` 环境变量误写入 sso 域 Cookie 的问题。
- OCR 并发关闭泄漏 — 修复池关闭后新创建的 OCR 实例不被清理、临时目录泄漏。
- 不同 token 共享 session 背压状态 — 修复 A 登录失败导致 B 也被误判为激活失败。
- session 激活并发安全 — 修复 `ActivateSession` 无 mutex 保护导致并发请求数据污染。
- 空用户信息被当错误处理 — 修复 `GetMyInfo` 返回空数据时误报错误。
- 任务列表部分维度失败时空白 — 修复部分评价维度请求失败时整个列表不输出成功数据。
- 空消息导致日志 panic — 修复 `resp.Msg` 为 nil 时无保护解引用。
- 维度抓取 panic 崩溃进程 — 修复某维度请求异常时整个 CLI 进程退出。
- 11 处 PII 残留 — 替换测试文件和文档中残留的真实姓名和学号。
- HTTP 连接池限制 — 默认 `MaxIdleConnsPerHost=2` 不够用，改为 16 避免高并发反复 TLS 握手。
- Debug 日志无谓分配内存 — 非 Debug 级别不再为日志参数做 `fmt.Sprintf` 分配。
- Base URL 拼接不统一 — 3 处直接拼接改为 `bizURL()` helper 集中处理。
- token flag 空字符串覆盖环境变量 — 用 `flagChanged()` 区分"没传"和"传了空值"。
- 顶层 panic 无保护 — 加 recover 统一 exit code 1，不打 stack trace。
- Session 背压无提示 — 捕获 `ErrSessionBackoff` 时输出冷却提示等待时长。
- context cancel 被任务抓取吞掉 — `FetchTasks` 的 goroutine 闭包检查 `gctx.Err()`。
- 文档残留已删接口引用 — 同步清理 `login-flow.md` 中已删 `GetCaptcha` 的说明。

### Changed

- OCR 可选构建 — `pkg/client` 不再强制导入 `internal/ocr`。无 `-tags ddddocr` 时编译为纯 Go，Login 返回 `ErrOCRNotConfigured`。
- 错误哨兵体系 — 新增 4 个哨兵，覆盖 Location 解析、OCR 缺失、session 背压、空数据场景。

### Build

- 版本号：`0.3.5`
- 新增构建变体：`go build -tags ddddocr`（含 OCR）/ `无 -tags`（纯 Go 无 CGO）
- CI 增加双构建变体验证

## [0.3.4] - 2026-06-26

### Fixed

- Token 过期时间不准 — 之前 200 路径始终用 `now+24h` 兜底，现在会解析服务端返回的 `exp`/`expires_in` 字段。
- GetSchoolID 死分支 — 删除了一个永远不会触发的 else-if 分支（服务端只返回 NAME 字段）。
- `derefOr` helper 简化 — nil-safe 字符串解引用，5 行变 3 行。
- `LoginResponse.RefreshAfter` 字段删除 — 从未被服务端填充过，删掉免得误导调用方。
- `UnifiedResponse` 6 个孤儿字段删除 — DataString、PageBean、Note、InsertID、UpdateCount、IsAttendance 全仓库 0 引用。
- drain+close 全部统一 — 所有 HTTP 请求的 body 关闭前都会先 drain 再 close，保持 keep-alive 连接可重用。
- 5+1 处业务错误用统一哨兵包装 — `SubmitSelfEvaluation`、`QuerySelfEvaluation`、`QuerySelfGradEvaluation`、`GetMyInfo`、`fetchDimensions` 的 CheckCode 改用 `ErrBusinessRejected` 而不是之前的各种散装错误。
- 维度抓取不静默吞错误 — 之前 `fetchTasksForDimension` 遇到业务错误只 logDebug 就返回 nil，现在会返回 error 让调用方知情。
- 上传客户端 50 次握手回归 — 修复新创建的 clean client 没复用 Transport 导致批量上传反复 TLS 握手。
- 6 个 Option 加校验守卫 — `WithSSOBase`/`WithBaseURL`/`WithUploadURL`/`WithHTTPClient`/`WithOCRConcurrency`/`WithToken` 遇到空值或负值时 warn + 保留原值。
- CLI 自动获得 Client 清理 — school 和 file_upload 改用统一 `buildClient` helper 后，自动获得 `trackClient(c)` 注册，退出时不再泄漏 ONNX 临时目录。
- `whoami` 空数据不报错 — 当 `GetMyInfo` 返回 `(nil,nil)` 时输出 `{"status":"empty"}` 而不是裸 `null`，区分"空响应"和"激活失败"。
- Session 激活失败背压 — 失败后缓存 + 5 秒冷却窗口，防止 N 个并发请求同时触发激活。
- 任务列表部分维度失败不吞成功数据 — 全失败返回全部错误；部分失败返回成功维度 + 错误信息；全成功正常返回。

### Changed

- `LoginResponse.RefreshAfter` 和 `UnifiedResponse` 6 个字段删除 — BREAKING API，全仓库确认 0 引用。旧 API 响应 JSON 反序列化兼容（Go 忽略未知字段）。
- OCR 进程级单例删除 — 不再有 `GetDefault`/`defaultOCR`/`defaultOnce`，由 Pool 替代。
- trackInit 改用 sync.Map — 99 次串行锁写 map 改为 `LoadOrStore`，key 已存在时 lock-free 跳过。
- 新增 `printPrompt` 函数 — 终端交互提示（如 self-eval 的"请输入评价"）走独立通道，不受 verbose 守卫，受 quiet + TTY 检测守卫。

### Build

- 版本号：`0.3.4`

## [0.3.3] - 2026-06-25

### Fixed

- HAR 测试数据含真实姓名和学号 — `self_eval.json` 的 `student_number`/`studentName` 仍有真实信息，替换为占位值，新增自动化扫描防止再出现。
- 图片处理 69 行死代码 — `prepareImageWithStats`、`prepResult`、`PrepStats` 结构体（14 字段）、`CompressionRatio` 方法全部未用，删除后 inline 到 `prepareImageForUpload`。
- syncCookieToken URL 解析失败静默 — 之前只有 Jar 类型断言失败会报错，URL 解析失败只打一条日志就返回 nil，现在统一返回 error。
- SubmitTask 业务错误用了错误的错误哨兵 — 业务 code≠1 时包装成 `ErrLoginRejected`，误导 SDK 用户走重新登录流程。新增 `ErrBusinessRejected` 哨兵专门用于业务拒绝场景。
- 上传客户端污染业务连接池 — `newCleanClient` 复用业务 Client 的 Transport，调用 `CloseIdleConnections` 时会误关业务请求的 keep-alive 连接。改用 `Transport.Clone()` 创建独立 idle 连接池。

### Changed

- `LoginResponse.UserInfo` 字段删除 — BREAKING API。登录响应从未填充过这个字段，用户信息请通过 `GetMyInfo()` 获取。

### Build

- 版本号：`0.3.3`

## [0.3.2] - 2026-06-25

### Fixed

- 集成测试编译 break — `client.New()` 签名改 `(*Client, error)` 后集成测试没适配，CI 编译失败。
- CLI 错误信息重复输出 — cobra 和 main 同时输出错误，终端看到两遍错误信息。统一由 `printError` 输出 JSON 格式。
- 200 登录路径缺少 token 过期告警 — 302 fallback 路径有兜底 warn，200 路径没有，不对称。
- Referer 头里的 token 没做 URL 编码 — 虽然 JWT 是 URL-safe 的，但防御性编程应使用 `url.Values.Encode()`。
- OCR 池并发关闭不安全 — 第二个 goroutine 关闭时第一个还在释放实例，可能重复释放同一 ONNX session。
- 任务抓取并发数不限 — 之前只留了 TODO 注释，现在加 `errgroup.SetLimit(8)` 限制并发。

### Build

- 版本号：`0.3.2`
- 新增依赖 `golang.org/x/sync v0.21.0`

## [0.3.1] - 2026-06-25

### Fixed

- 登录请求后没 drain HTTP body — 多个 early-return 路径直接 close 连接，导致 TCP 连接无法归还 keep-alive 池，高频调用下反复建连。
- Token 过期告警被静默 — expiresAt 兜底应打 Warn 级别，但误用了 Debug 级别，默认配置下完全看不到。
- 200 登录路径 unmarshal 失败被吞 — 错误信息只说"未找到 token"，丢了 body 内容这个关键诊断信息。
- syncCookieToken 静默失败 — 类型断言失败只打一条 warn 就返回 nil，build client 阶段完全感知不到，后续业务接口全空时才暴露问题。改返回 error，`client.New()` 签名调整为 `(*Client, error)`。
- OCR 重试不响应 context cancel — 99 次循环顶部没检查 ctx，用户取消后还会跑完所有重试。
- Session 激活并发安全 — 检查 state 后立刻放锁，4 步激活在无锁状态下执行，并发 goroutine 浪费请求且污染 cookie。
- Session 激活第 4 步失败被掩盖 — `getMyInfoRaw` 失败只打 debug 日志，调用方收到空 UserInfo 以为激活成功。
- WithTimeout 负数/零值没阻拦 — 0 值覆盖已有正数超时，导致请求可能永久挂起。
- `whoami` 输出 null 被当错误处理 — `GetMyInfo` 返回 `(nil,nil)` 时走 `printError` + 退出码 1，误导用户。
- `printError` 直接 os.Exit 绕过资源清理 — 跳过 `defer closeAllClients()`，ONNX session + 临时目录 + keep-alive 连接全部泄漏。改为标记退出码，统一在 main 末尾退出。

### Changed

- `client.New(opts ...Option) *Client` → `(*Client, error)` — BREAKING API。`syncCookieToken` 现在返回 error，`WithHTTPClient` 传了非 CookieJar 的 Jar 时会报错。12 个 cmd 调用点已用 `c, _ := client.New(...)` 适配。

### Build

- 版本号：`0.3.1`

## [0.3.0] - 2026-06-24

### Fixed

- io.ReadAll 错误静默丢弃 — 网络闪断时读 body 失败，错误没说清楚，只给一句误导性的"未找到 token"。
- 验证码图片读取失败时没 drain — 出错了也先 drain body 再 close，保证 TCP 连接可复用。
- ExpiresAt 零值 — 200 路径的登录过期时间返回公元 0001 年，改为 `now+24h` 兜底。
- syncCookieToken 兼容性 — 类型断言失败时输出实际类型和修复提示，方便排查。
- Session 激活不感知 token — 不同 token 共享同一个 session 缓存，切换 token 后可能返回旧用户数据。
- FetchTasks 没用 session 激活 — 与其他业务方法不一致，少了 `activateSessionIfNeeded` 调用。
- getMyInfoRaw 错误传播中断 — CheckCode 错误被截断，调用方收不到准确错误。
- sync.Pool 裸类型断言 — 没有 `ok` 检查，GC 回收后可能 panic。
- 上传客户端零超时传播 — 父 client 没设超时时上传请求无限等待，兜底 30s。

### Changed

- 重构 request.go — 提取 `buildRequest()` 消除 `doRequest`/`doRequestWithResp` ~40 行重复代码。
- CLI 提取 `buildBizClient()` — 消除 6 个命令文件各 ~15 行 env fallback + Client 构造样板，统一到 `cmd/nazhi/client_builder.go`。
- 请求日志加 debug guard — 非 Debug 级别不再每次请求都遍历 header。
- version 命令输出 JSON — `nazhi version` 输出 `{"version":"0.3.0"}` 统一输出格式。

### Build

- 版本号：`0.3.0`

## [0.2.2] - 2026-06-24

### Added

- Shell 自动补全 `nazhi completion [bash|zsh|fish|powershell]`
- 版本号子命令 `nazhi version`

### Fixed

- Session 兜底 body 读取 bug — `session.go:77` 中步骤 4 失败后 body 已被 defer Close 消耗的问题。

### Changed

- 文档 emoji 清理 — 全部文档和注释移除 emoji。
- Makefile — echo 消息纯文本化。

### Tests

- `TestActivateSession_*` 系列 — 5 个 session fallback 测试覆盖。

### Build

- 版本号：`0.2.2`

## [0.2.1] - 2026-06-24

### Changed

- OCR 重试策略：`3×33` → `1×99`。同一张图 OCR 结果是确定性的，重试无意义，换图才有效。
- Makefile echo：移除所有 emoji，输出保持纯文本。
- CHANGELOG / README / 文档：全部移除 emoji，统一风格。

### Fixed

- 测试性能：`TestPrepareImage_CompressesLargeImage` 从 3000×3000 降为 1500×1500 + Pix 直接填充（29s → 3s）。
- CI 全平台修复：10+ 轮修复后，5 平台（Linux amd64/arm64, macOS arm64, Windows amd64/arm64）全部构建通过。
  - Linux arm64：`gcc-aarch64-linux-gnu` 在 amd64 runner 交叉编译
  - Windows arm64：`zig cc` 在 amd64 runner 交叉编译
  - golangci-lint：`go install` 兼容 Go 1.26.1
  - softprops release：`continue-on-error: true` 处理新 release 404
- CLAUDE.md：OCR 并发策略、CI 修复历程、发布资产全部更新。

### Build

- 版本号：`0.2.1`

## [0.2.0] - 2026-06-22

### Features

#### 跨平台 OCR（5 平台）
- 5 平台 build tag 隔离的 `onnx_*.go` 嵌入文件（win/lin/mac × amd64/arm64）
- `ocr.GetDefault()` 进程级单例 + `sync.Mutex` 并发保护
- 99 次重试机制（同一图片）提高识别准确率
- 解压到磁盘目录供 `onnxruntime_go` 加载

#### 全自动验证码流程
- 简化 `Login()` 内部流程：InitSession → GetSchoolID → OCR → validate → 302/200 提取 token
- 优先处理 200 JSON 响应（HAR 验证），fallback 到 302 Location
- 移除所有手动/交互式验证码模式
- 自动 `syncCookieToken` 同步到 SSO + 业务域 Cookie

#### HAR 对齐的 4 步 Session 激活
- 步骤 1：GET / 初始化后端 Session
- 步骤 2：GET /api/studentInfo/getMenu（Referer: /homepage?token=xxx）
- 步骤 3：GET /api/studentInfo/getMenu（Referer: /home）
- 步骤 4：GET /api/studentInfo/getMyInfo（返回完整 51 字段 UserInfo）

#### UserInfo 51 字段
- 完整暴露 `getMyInfo` 返回数据
- `birthdayStr` 字符串化（Java LocalDate JSON 数组兼容）
- 移除自定义 `Birthday` 类型

#### 图片自动压缩预处理
- 任意格式 → JPG（PNG/BMP/WEBP/GIF 支持）
- 透明合成（flattenOnWhite）
- 质量级联 → 缩放级联
- 上限 5MB
- 全部在内存中完成，不写盘

#### CLI 环境变量支持
- `NAZHI_USERNAME` / `NAZHI_PASSWORD` / `NAZHI_TOKEN`
- `NAZHI_SSO_BASE` / `NAZHI_BASE_URL` / `NAZHI_UPLOAD_URL`
- `NAZHI_TIMEOUT`
- 命令行标志优先于环境变量（用 `flagChanged` 检测）
- `.env.example` 模板 + `.gitignore` 排除真实 `.env`

#### HAR 驱动集成测试
- 5 个 fixture 文件（task_flow、self_eval、military、class_meeting、labor）
- 6 个 HAR 驱动测试覆盖 FetchTasks、SubmitTask（4 种类型）、SubmitSelfEvaluation
- 真实环境 10 步端到端 `TestReal_FullChain`
- 4 个回归测试

#### 完整文档体系
- `docs/README.md` — 文档中心索引
- `docs/cli/README.md` — CLI 命令参考
- `docs/sdk/README.md` — Go SDK API 参考
- `docs/architecture.md` — 架构总览
- `docs/login-flow.md` — 登录流程详解
- `docs/cross-platform-ocr.md` — 跨平台 OCR 设计
- `docs/env-vars.md` — 环境变量参考
- `docs/har-testing.md` — HAR 驱动测试架构

### Fixes

#### Security
- 历史凭据泄露已修复（v0.1.0 之前）：通过 `git-filter-repo` 重写所有分支和 tag 历史
- CLI `--token` Cookie 同步：新增 `WithToken()` Option，CLI 传 token 时同时写 Header + Cookie
- UploadFile 禁用重定向：cleanClient.CheckRedirect 防止 302 跳转到攻击者主机

#### Bugs
- Task.StartDate 字段错配：从 `startDate`（数组）改为 `startDateStr`（字符串）
- extractTokenFromLocation URL 解析：从 `strings.Index` 改为 `net/url.Parse`，支持 fragment
- session.go 步骤 1/2 Body 泄漏：defer + io.Copy 模式
- QuerySelfGradEvaluation 错误被吞：所有路径失败时返回明确 error
- FetchTasks 静默失败：用 `c.logDebug` 记录（不破坏 API）
- output.go stderr 编码失败：加 `fmt.Fprintln` 兜底
- ImagePrep 兜底大小检查：避免返回超大文件
- stdin 无 TTY 阻塞：`isTerminalStdin()` 检测

#### Dead Code 清理
- 删除未使用的 4 个哨兵错误（ErrTokenExpired、ErrSessionExpired、ErrIncompleteResponse、ErrUnexpectedStatus）
- 删除未使用的类型（SchoolInfo、SessionInfo）
- 删除未使用的函数（EnforceCode、自定义 min）
- 删除 debug 工具目录（cmd/debuglogin/、cmd/reallogin/、cmd/getcaptcha/、cmd/ocrtest/）

### CI/CD

- 5 平台 native runner 矩阵（ubuntu-latest、ubuntu-22.04-arm64、macos-latest、windows-latest、windows-11-arm）
- 新增 `integration` Job：tag 发布时跑真实环境集成测试（需 secrets）
- 新增 `gofmt` 检查
- 新增 `go mod tidy` 验证
- 新增 SHA256 校验和
- 二进制 `--version` 验证步骤

### Build

- Go 1.26.1
- 单二进制分发（内嵌 OCR 模型 + onnxruntime）
- Makefile：`build` / `test` / `test-verbose` / `test-integration` / `lint` / `vet` / `fmt` / `release` / `clean`

## [0.1.0] - 2026-06-21

初始发布 — nazhi-cli：纳智综合评价自动化 CLI + Go SDK。

### Features

- SSO 全自动登录 — InitSession → GetSchoolID → 验证码处理 → Login 全流程
- 内置 OCR 验证码识别 — ddddocr 引擎 + 模型已内嵌至二进制，无需运行时下载
- 学校 ID 查询 — 根据学号获取学校信息
- 业务 Session 激活 — 登录后激活目标平台 API Session
- 用户信息查询 — 获取当前用户 profile
- 任务管理 — 列出任务 + 提交任务（支持 `@file.json` 读取）
- 自我评价 — 提交评价 & 查询评价状态
- 文件上传 — 本地图片上传至目标平台
- 跨平台构建 — Linux / macOS / Windows 三平台二进制支持

### Tech

- Go 1.26 + cobra CLI 框架
- ddddocr（ONNX Runtime）嵌入式验证码识别
- 单二进制分发，零外部依赖
