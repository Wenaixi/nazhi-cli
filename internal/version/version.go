// Package version 提供 CLI 版本信息。
package version

// Version 是 nazhi CLI 的当前版本号。
// 遵循 semver：major.minor.patch
//
//	0.1.0 — 初始版本
//	0.2.0 — 跨平台 OCR + 进程级单例 + HAR 驱动测试 + cookie 同步修复
//	0.2.1 — 多图多试 OCR 优化（1×99 策略）+ CI 全平台修复 + 文档完善
//	0.2.2 — Shell 自动补全 + 版本子命令 + Session bug 修复 + 测试补充 + 代码质量修复
//	0.3.0 — 全仓库代码审查修复（panic 风险/ExpiresAt 零值/session token 感知/代码结构重构）
//	0.3.1 — 二轮 review-tdd：13 findings 修复 + client.New 改 error 返回 + 并发安全 + ctx 退出
//	0.3.2 — 三轮 review-tdd：9 findings 修复 + CI 集成测试编译 break + 5 worktree 并行 TDD
//	0.3.3 — 四轮 review-tdd：7 findings 修复 + HAR fixture PII 清理 + image_prep -69 行 dead code + syncCookieToken baseURL propagate + LoginResponse.UserInfo 删除 + drainAndClose helper + ErrBusinessRejected + Transport Clone 隔离
var Version = "0.3.4"
