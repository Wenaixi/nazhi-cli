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
var Version = "0.4.1"
