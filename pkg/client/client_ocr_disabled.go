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
func defaultOCR() captchaRecognizer {
	return nil
}

// WithOCRConcurrency 占位实现（!ddddocr 构建）。
//
// ddddocr 未启用时无内置识别器，本 Option 仅 warn 提示调用方改用
// WithCustomOCR 注入，避免编译错误（保持 Option 函数签名导出）。
func WithOCRConcurrency(n int) Option {
	return func(c *Client) {
		c.logger.Warn("WithOCRConcurrency: 当前构建未启用 ddddocr（-tags ddddocr 缺失），本选项无效。请改用 WithCustomOCR 注入自定义识别器",
			"n", n,
			"tip", "Nazhi-auto CGO_ENABLED=0 构建下必须用 WithCustomOCR")
	}
}
