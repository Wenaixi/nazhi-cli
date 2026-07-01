//go:build ddddocr
// +build ddddocr

// Package client 在 -tags ddddocr 构建下导入 internal/ocr。
// 此文件跨越了 public -> private 边界——外部 pkg/client 的使用方
// 在自己的模块中无法使用此 build tag（Go 拒绝导入 internal/ 包）。
// 外部 SDK 使用者请使用 WithCustomOCR() 注入识别器。
package client

import (
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/Wenaixi/nazhi-cli/internal/ocr"
)

// defaultOCRInstance 是懒加载的进程级 OCR Pool 单例。
var (
	defaultOCROnce     sync.Once
	defaultOCRInstance atomic.Pointer[ocr.Pool]
)

// defaultOCR 在指定 -tags ddddocr 时返回 ddddocr Pool 单例。
// 懒加载：首次调用时创建，后续复用同一实例。
//
// F8.2 优化：默认预加载 min(4, runtime.NumCPU()) 个实例实现 OCR 并发，
// 避免 99 张验证码串行识别 ≈60s。每实例约 50MB，N=4 约 200MB。
// 与 client_ocr_disabled.go 中的 nil 默认行为对称：
//   - 启用 ddddocr → 客户端开箱即用，无需注入自定义 OCR
//   - 禁用 ddddocr → 必须用 WithCustomOCR 注入 AI/外部识别器
func defaultOCR() CaptchaRecognizer {
	defaultOCROnce.Do(func() {
		n := runtime.NumCPU()
		if n > 4 {
			n = 4
		}
		defaultOCRInstance.Store(ocr.NewPool(n))
	})
	return defaultOCRInstance.Load()
}

// WithOCRConcurrency 设置 OCR 实例池预分配数量（ddddocr 构建）。
// 行为约定：
//   - 0 或 1 = 默认懒加载单实例（与原单例行为一致，1 路串行识别）
//   - N > 1 = 预分配 N 个 OCR 结构体，ONNX session 惰性初始化，
//     首次调用 Recognize 时触发完整模型加载
//   - n < 0：拒绝设置并 warn，保持当前 c.ocr（防止负数被静默截 0
//     后用默认值覆盖调用方已注入的自定义识别器，如 WithCustomOCR mock）
//
// 内存代价：每个 ONNX session 约 50MB（模型 + 原生库），N=4 约 200MB。
// 业务场景：批量调用 Login() 时才需要调高；单次 Login 用 1 实例足够。
func WithOCRConcurrency(n int) Option {
	return func(c *Client) {
		if n < 0 {
			c.logger.Warn("WithOCRConcurrency: 负数被拒绝，保持当前 OCR 实例",
				"n", n,
				"tip", "用 0/1 = 单实例，N>1 = 并发池")
			return
		}
		c.ocr = ocr.NewPool(n)
	}
}
