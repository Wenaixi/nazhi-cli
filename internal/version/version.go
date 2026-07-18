// Package version 提供 CLI 版本信息。
package version

// Version 是 nazhi CLI 的当前版本号。
// 遵循 semver：major.minor.patch
//
//	0.1.0 — 初始版本
//	0.2.0 — 跨平台 OCR + 进程级单例 + HAR 驱动测试 + cookie 同步修复
//	0.2.1 — 多图多试 OCR 优化（1×99 策略）+ CI 全平台修复 + 文档完善
//	0.2.2 — Shell 自动补全 + 版本子命令 + Session bug 修复 + 测试补充 + 代码质量修复
//	0.3.0 — 全仓库代码审查修复（panic 风险 / ExpiresAt 零值 / session token 感知 / 代码结构重构）
//	0.3.1 — Login drain+close / expiresAt 告警 / unmarshal 错误传播 / syncCookieToken error 返回 /
//	        OCR ctx 退出 / session 并发安全 / 资源泄漏 / BREAKING: client.New 改 (*Client, error)
//	0.3.2 — 集成测试编译修复 / stderr 双重输出 / Pool.Close 并发安全 / FetchTasks 并发上限 /
//	        session Referer URL 编码
//	0.3.3 — HAR fixture PII 清理 / image_prep 死代码删除 / syncCookieToken baseURL 传播 /
//	        ErrBusinessRejected 哨兵 / LoginResponse.UserInfo 删除 / drainAndClose 辅助函数 / Transport Clone
//	0.3.4 — Token 过期时间解析 / 死字段删除 / 5+1 处 ErrBusinessRejected 统一包装 / 6 个 Option 守卫 /
//	        buildClient 统一 / trackInit sync.Map / printPrompt / whoami 空状态 / session backoff / FetchTasks partial 失败处理
//	0.3.5 — OCR 可选构建（build tag: ddddocr）/ multipart 终止边界 / GIF 黑底修复 / 压缩死循环 /
//	        os.Exit 资源泄漏 / PII 守卫扩展 / 自定义 Transport 16 conns/host / 4 个错误哨兵 /
//	        flagChanged token 守卫 / 顶层 panic recover / context cancel 检查 / 文档清理
//	0.4.0 — review-tdd 第 15/16 轮全面修复 + 架构深化（sessionManager 收口 / HTTP helper 私有化 /
//	        DecodeUnified 原语 / tokenparse 包 / DerefOr[T]）/ OCR Windows DLL 占用降级 + 启动
//	        时清扫临时目录残留 + panic stack 输出 + Cookie sync 兜底 + image_prep 缩放级联优化
//	0.4.1 — review-tdd 第 22 轮修复：parallel.go 泛型并发 helper + error_category.go 错误分类枚举 +
//	        recoverx 统一 panic recover 包 + image_prep 简化为单次缩放 + defaultOCR 惰性预热 +
//	        tokenparse 3 哨兵错误 + withURLGuard/withNilGuard Option 工厂
//	0.5.0 — honor 荣誉申报全功能（SDK 5 方法 + CLI 3 子命令）+ GetSubmittedCircles 已提交写实记录 +
//	        --payload - stdin 读取 + review-tdd 第 23 轮全面清理（40 commits）+
//	        全部文档全面同步
//	0.5.1 — 修复 commit 消息 @ 前缀违规 + honor.go 注释缩进 + CLAUDE.md 同步
//	1.0.0 — BREAKING CHANGE: 全面重构 — 字段裁减 122→67、统一 camelCase JSON 命名、统一响应信封
//	        （status/code/message/data）、状态字段 bool 化（submitted/approved/needPic）、
//	        时间字段 ISO 8601 +08:00、HTTP 风格业务码、三重退出码、工程化规范（lint/CI/Makefile/迁移指南）
//	1.1.0 — 任务提交链路升级：最小输入模型 TaskSubmitInput，SDK 内部自动完成 getCircleTypeByTaskId
//	        元数据预取 + UploadFile 图片上传 + 30 字段 addCircle 组装；CLI 同步支持 --address/--level
//	        独立 flag；playRole 默认空串不猜 "3"；level 默认 "5" 不靠任务类型分支；
//	        移除军训/班会/劳动等特殊类型逻辑；全链路真实环境成功验证（4月生产劳动）；
//	        全面更新 docs/sdk/README.md + docs/cli/README.md + real-responses-reference.md
//	1.1.1 — 修复 task list 输出 SDK 最终业务模型（FetchTasks）而非原始 JSON（FetchTasksJSON）；
//	        修复 FetchTasksJSON 聚合时 dimErrs 并发写入 race + id=0 维度收发次数不一致卡死
//	1.1.2 — task submitted 支持 --limit / --offset / --count 分页控制；
//	        新增 GetSubmittedCirclesLimitJSON SDK 方法
//	1.1.3 — 清理误跟踪 test_get_school_id.go；同步文档至最新（CLI/SDK README 对应表修正）
//	1.1.4 — 版本号同步 + CHANGELOG 补充 + 文档同步（task submitted --limit/--offset/--count）
//	1.1.5 — 文档 task submitted 示例更换为真实脱敏 JSON，注明含同班同学姓名/学号
//	1.2.1 — 新增典型案例提交功能（SDK + CLI + 单元测试）
//	1.2.3 — 新增教师写实/被撤回写实/公示查看三个接口（SDK + CLI），重构 submitted.go 提取通用 fetchCirclePage
//	1.2.4 — 新增 task edit 命令（修改已提交的写实记录）+ EditCircle SDK 方法
//	1.3.0 — 深度修复：根据前端源码全面补齐 SDK 字段和 API
//	        - 类型定义：扩展 CircleRecord（30+ 字段）、UserInfo（10+ 字段）
//	        - 新增类型：ExamResult、ViolationRecord、Notification、BonusInfo、DemocraticActivity 等
//	        - 新增 SDK 方法（8 个文件）：
//	          circle.go（删除写实、添加评论、点赞、获取图片/任务/类别/维度/字典）
//	          exam.go（成绩查询）、democratic.go（民主评价）、violation.go（违规记录）
//	          notification.go（通知消息）、bonus.go（积分商城）、file_bag.go（档案查看）、user_update.go（用户信息更新）
//	        - 新增 CLI 命令（6 个父命令）：circle、exam、violation、notification、bonus、user
var Version = "1.3.0"
