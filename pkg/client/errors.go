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
	// F7 修复：SubmitTask 等业务方法应使用本哨兵而非 ErrLoginRejected，
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
	ErrOCRNotConfigured = errors.New("OCR not configured: use WithCustomOCR to inject a captchaRecognizer, or build with -tags ddddocr to enable the built-in ddddocr engine")

	// ErrSessionBackoff session 激活在 backoff 窗口内被抑制（thundering herd 防护）。
	//
	// 与 ErrNetwork / ErrBusinessRejected 的语义边界：
	//   - ErrSessionBackoff：上次激活失败后短时间内再次调用，
	//     SDK 用户应等待 backoff 窗口结束或换 token 后重试
	//   - ErrNetwork / ErrBusinessRejected：实际尝试过后的真实错误
	//
	// F15 修复（round-7）：backoff 命中时返回本哨兵（包装上一个错误），
	// 而非直接返回 lastActivationErr。这样 SDK 用户能通过 errors.Is 识别
	// 「这是被抑制的 stale 错误」并做出有意义的重试决策。
	ErrSessionBackoff = errors.New("session activation backoff: in cooldown window")

	// ErrEmptyUserInfo 业务无用户数据（getMyInfo 成功但 returnData + dataMap 都为 nil）。
	//
	// 与 ErrBusinessRejected 的语义边界：
	//   - ErrEmptyUserInfo：服务端成功响应（HTTP 200 + code=1）但确实没有用户数据
	//     （不是错误，只是空集）。SDK 用户的最佳实践是按 status envelope 渲染。
	//   - ErrBusinessRejected：服务端主动拒绝（HTTP 200 + code=0，或业务校验失败）
	//
	// F10 修复（round-7）：getMyInfoRaw 在全 nil fallback 时返回本哨兵，
	// 而非返回 (nil, nil) 让 cmd 层「裸 null」输出。cmd 层用 errors.Is 分支
	// 输出对称的 {status: empty, reason: ...} envelope，与 whoami 契约一致。
	ErrEmptyUserInfo = errors.New("getMyInfo returned no user data")

	// ErrLocationParseFailed Login 302 Location 头解析失败（畸形 URL）。
	//
	// F2-EXTRACT-TOKEN-ASYM 修复（round-8）：对称化 extractTokenFromLocation
	// 与 extractTokenFromReturnData 的错误处理契约——前者原本 url.Parse 失败时
	// 静默返回 ("", now+24h)，让畸形 Location 悄无声息地走到"未找到 token"
	// 错误，吞掉根因。改为 propagate 后 SDK 用户能看到具体 URL 解析错误。
	//
	// 调用方（auth.go Login 302 路径）只把 location 字符串写到 logDebug，
	// 避免泄漏到 stderr 错误消息（location 可能含 token fragment）。
	ErrLocationParseFailed = errors.New("login location header parse failed")
)
