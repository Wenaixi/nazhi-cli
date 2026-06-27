//go:build !ddddocr
// +build !ddddocr

package client

// defaultOCR 在未指定 -tags ddddocr 时返回 nil。
//
// 设计动机：Nazhi-auto 等 CGO-free 消费者无法引入 ddddocr（CGO），
// 本函数允许默认客户端不导入 internal/ocr 包，编译为纯 Go 二进制。
//
// 调用方需通过 WithCustomOCR 注入自定义识别器，否则 Login() 会立即
// 返回 ErrOCRNotConfigured（行为已通过 client_ocr_optional_test.go 验证）。
func defaultOCR() CaptchaRecognizer {
	return nil
}

// WithOCRConcurrency 占位实现（!ddddocr 构建）。
//
// ddddocr 未启用时无内置识别器，本 Option 对 n 值做区分处理：
//   - n == 0：静默 no-op（合法降级，保持 c.ocr 不变，不输出 warn）
//   - n < 0：warn + 保持当前 c.ocr（防止负数静默覆盖已有 WithCustomOCR 注入）
//   - n > 0：warn + 引导改用 WithCustomOCR（提示调用方注入自定义识别器）
//
// H4 修复：原实现对所有 n 值统一 warn，n=0 场景（SDK 用户期望
// reset to single instance）收到误导性 warn。改为精准区分。
func WithOCRConcurrency(n int) Option {
	return func(c *Client) {
		switch {
		case n == 0:
			// 静默 no-op：n=0 是合法降级请求，不输出 warn。
			// 与 WithTimeout(0) 的行为不同——WithTimeout(0) warn 是因为
			// 它会清零正数超时导致请求永久挂起；n=0 只是保留单实例，
			// 无危害。
		case n < 0:
			c.logger.Warn("WithOCRConcurrency: 负数参数被拒绝，保持当前识别器。!ddddocr 构建下请改用 WithCustomOCR 注入自定义识别器",
				"n", n,
				"tip", "传入 0 可静默降级到单实例，传入 >0 需要 WithCustomOCR")
		default: // n > 0
			c.logger.Warn("WithOCRConcurrency: 当前构建未启用 ddddocr（-tags ddddocr 缺失），本选项无效。请改用 WithCustomOCR 注入自定义识别器",
				"n", n,
				"tip", "Nazhi-auto CGO_ENABLED=0 构建下必须用 WithCustomOCR")
		}
	}
}
