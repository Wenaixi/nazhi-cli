// Package client 实现纳智综合评价目标平台的完整 Go SDK。
//
// 每个 Client 实例拥有独立的 HTTP cookie jar，天然并发安全。
// 所有方法都需要 context.Context，支持超时与取消。
package client

import "errors"

// ─── 哨兵错误 ───

var (
	// ErrLoginRejected 登录被拒绝（凭证无效或验证码错误）。
	ErrLoginRejected = errors.New("login rejected: invalid credentials or captcha")

	// ErrNetwork 网络错误（连接超时、DNS 解析失败等）。
	ErrNetwork = errors.New("network error")

	// ErrUploadRejected 文件上传被服务器拒绝。
	ErrUploadRejected = errors.New("upload rejected by server")

	// ErrFileTooLarge 文件超出允许大小。
	ErrFileTooLarge = errors.New("file exceeds max size")

	// ErrInvalidPayload 任务请求体格式错误。
	ErrInvalidPayload = errors.New("invalid task payload")

	// ErrBusinessRejected 业务请求被服务端拒绝（非登录场景）。
	//
	// 与 ErrLoginRejected 的语义边界：
	//   - ErrLoginRejected：登录请求被拒绝（凭证无效/验证码错误），
	//     SDK 用户应触发重新登录流程
	//   - ErrBusinessRejected：已通过鉴权的业务请求被服务端拒绝
	//     （如任务已提交、参数错误、服务端 5xx），与登录状态无关，
	//     SDK 用户应只展示服务端 msg 或重试，不必重新登录
	//
	// SubmitTask 等业务方法应使用本哨兵而非 ErrLoginRejected，
	// 否则 SDK 用户按 errors.Is(err, ErrLoginRejected) 判定后会错误地走重新登录。
	ErrBusinessRejected = errors.New("business request rejected by server")

	// ErrOCRNotConfigured 表示 Client 未配置验证码识别器。
	//
	// 触发场景：构建时未加 -tags ddddocr（OCR 包未导入，c.ocr 默认 nil）
	// 且调用方未用 WithCustomOCR 注入自定义识别器，此时调用 Login() 必失败。
	//
	// 修复动机：CGO-free 消费者（如 Nazhi-auto CGO_ENABLED=0 构建）无法使用
	// ddddocr 内置识别器，必须通过 WithCustomOCR 注入 AI/外部识别器。
	// 该哨兵让 SDK 用户能 errors.Is 精确识别「没配 OCR」 vs 「OCR 识别失败」。
	//
	// H3 修复：错误消息改为中文 actionable，i18n key 为
	// 「errors.ocr_not_configured」。中英双语并列——英文部分是 SDK 用户
	// 编程接口可读的稳定契约（errors.Is(err, ErrOCRNotConfigured).Error() 输出），
	// 中文部分是给中文 CLI 用户的 actionable 指引（cmd/nazhi/login.go 用
	// errors.Is 分支渲染 envelope 时只取中文部分）。
	//
	// SDK 用户建议用 errors.Is(err, ErrOCRNotConfigured) 而非字符串匹配。
	ErrOCRNotConfigured = errors.New(
		"errors.ocr_not_configured: OCR 识别器未配置：当前构建未启用 -tags ddddocr。" +
			"请使用预编译 release 二进制（nazhi-cli releases 页面），或通过 SDK 调用 " +
			"client.WithCustomOCR(myRecognizer) 注入自定义识别器。" +
			" (OCR recognizer not configured: current build lacks -tags ddddocr. " +
			"Use the prebuilt release binary from GitHub releases, or inject a custom " +
			"recognizer via client.WithCustomOCR(myRecognizer).)",
	)

	// ErrSessionBackoff session 激活在 backoff 窗口内被抑制（thundering herd 防护）。
	//
	// 与 ErrNetwork / ErrBusinessRejected 的语义边界：
	//   - ErrSessionBackoff：上次激活失败后短时间内再次调用，
	//     SDK 用户应等待 backoff 窗口结束或换 token 后重试
	//   - ErrNetwork / ErrBusinessRejected：实际尝试过后的真实错误
	//
	// backoff 命中时返回本哨兵（包装上一个错误），
	// 而非直接返回 sm.lastErr。这样 SDK 用户能通过 errors.Is 识别
	// 「这是被抑制的 stale 错误」并做出有意义的重试决策。
	ErrSessionBackoff = errors.New("session activation backoff: in cooldown window")

	// ErrEmptyUserInfo 业务无用户数据（getMyInfo 成功但 returnData + dataMap 都为 nil）。
	//
	// 与 ErrBusinessRejected 的语义边界：
	//   - ErrEmptyUserInfo：服务端成功响应（HTTP 200 + code=1）但确实没有用户数据
	//     （不是错误，只是空集）。SDK 用户的最佳实践是按 status envelope 渲染。
	//   - ErrBusinessRejected：服务端主动拒绝（HTTP 200 + code=0，或业务校验失败）
	//
	// getMyInfoRaw 在全 nil fallback 时返回本哨兵，
	// 而非返回 (nil, nil) 让 cmd 层「裸 null」输出。cmd 层用 errors.Is 分支
	// 输出对称的 {status: empty, reason: ...} envelope，与 whoami 契约一致。
	ErrEmptyUserInfo = errors.New("getMyInfo returned no user data")

	// ErrOCRPanic OCR 识别器 Recognize panic（被 safeOCRRecognize recover）。
	//
	// Recognize 实现（mock / CGO ddddocr）
	// 可能在不可预见的边界条件下 panic（如 nil deref / CGO 崩溃）。
	// safeOCRRecognize 用 defer recover 捕获 panic 并包装为本哨兵，
	// 避免 panic 扩散到 Login 流程、crash 整个进程。
	ErrOCRPanic = errors.New("OCR recognizer panic: recovered")

	// ErrRateLimited 服务端限流响应（HTTP 429）。
	//
	// 触发场景：目标平台返回 429 Too Many Requests。
	// 与 ErrNetwork / ErrBusinessRejected 的语义边界：
	//   - ErrRateLimited：服务端主动节流，SDK 用户应退避后重试
	//     （可用 Retry-After 头判断等待时间）
	//   - ErrBusinessRejected：业务校验失败（HTTP 200 + code=0 或 业务 4xx）
	//   - ErrNetwork：网络层失败（连接拒绝/超时等），与服务端策略无关
	//
	// request.go 的 doBizGet 在收到 429 时包装本哨兵，
	// SDK 用户通过 errors.Is(err, ErrRateLimited) 精确识别限流场景，
	// 触发退避策略而非立即报错或重连。
	ErrRateLimited = errors.New("rate limited: HTTP 429 too many requests")

	// ErrServiceUnavailable 服务端不可用（HTTP 5xx）。
	//
	// 触发场景：目标平台返回 5xx（502/503/504 等）。
	// 与 ErrNetwork / ErrRateLimited 的语义边界：
	//   - ErrServiceUnavailable：服务端主动拒绝或临时过载，
	//     SDK 用户应等待重试（指数退避）
	//   - ErrNetwork：客户端网络层失败（连不上服务端）
	//   - ErrRateLimited：限流（429），属于可预期的服务端策略
	//
	// request.go 的 doBizGet 在收到 5xx 时包装本哨兵。
	ErrServiceUnavailable = errors.New("service unavailable: HTTP 5xx")

	// ErrTimeout 请求超时。
	//
	// 触发场景：上下文 deadline 触发 / net/http 内部超时。
	// 与 ErrNetwork 的语义边界：
	//   - ErrTimeout：超时（带 deadline 信息），SDK 用户可按 deadline 调整
	//   - ErrNetwork：网络层失败（连接拒绝 / DNS 失败 / TLS 失败）
	//
	// 主要用途：让 SDK 用户能 errors.Is 区分「超时」与「连不上」，
	// 决定是否调大 timeout 或换网络。
	//
	// ponytail: reserved for future timeout wrapping — not dead code
	ErrTimeout = errors.New("timeout: request exceeded deadline")

	// ErrInvalidResponse 服务端返回非 200 状态码（4xx 排除 429）。
	//
	// 触发场景：目标平台返回 4xx 但非 429（404/403/400 等）。
	// 与 ErrBusinessRejected / ErrRateLimited 的语义边界：
	//   - ErrInvalidResponse：HTTP 协议层错误（4xx），通常是请求语法错误或权限缺失
	//   - ErrBusinessRejected：业务逻辑拒绝（HTTP 200 + code=0）
	//   - ErrRateLimited：HTTP 429 限流（独立处理）
	//
	// request.go 的 doBizGet 在收到 4xx-other 时包装本哨兵，
	// 让 SDK 用户能精确识别「HTTP 层错误」与「业务层错误」，
	// 避免错误地把 404 等当成业务拒绝走重登录流程。
	ErrInvalidResponse = errors.New("invalid response: HTTP non-200 non-429")

	// ErrRetryable 表示「context 取消导致的失败，可重试」。
	//
	// 触发场景：FetchTasks 中部分维度因 ctx cancel 而失败（cancelledCount > 0），
	// task.go 用 cancelPlaceholder = fmt.Errorf("%w: ...", ErrRetryable, ...) 包装，
	// SDK 用户可通过 errors.Is(err, ErrRetryable) 区分「ctx cancel 应重试」
	// 与「业务错误不应重试」。
	//
	// 与 ErrBusinessRejected 的语义边界：
	//   - ErrRetryable：ctx cancel 引发的「可重试」语义标记
	//   - ErrBusinessRejected：服务端业务拒绝（code=0），不应盲目重试
	//
	// F2.1 修复：原 cancelPlaceholder 用裸 fmt.Errorf，错误消息含「可重试」但
	// 缺少 sentinel 标识，SDK 用户只能字符串匹配。改为 fmt.Errorf("%w: ...")
	// 让 errors.Is 精确识别。
	ErrRetryable = errors.New("retryable: context cancelled")
)
