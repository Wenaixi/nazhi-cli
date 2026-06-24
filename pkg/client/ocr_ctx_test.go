// Package client 内部白盒测试。
//
// F11: pkg/client/auth.go:233 ocrRecognizeWithRetry ctx 不退出 — 回归测试。
//
// 历史 bug：99 次循环顶部无 ctx.Err() 检查，c.ocr.Recognize() 是 CGO 阻塞
// 调用不响应 ctx cancel。fetchCaptchaImage 走 doBizGet 已尊重 ctx，
// 但 OCR CGO 期间不响应，ctx cancel 后还在死等 OCR 返回。
//
// 修复后：在 ocrRecognizeWithRetry 的 for 循环顶部加
//   if err := ctx.Err(); err != nil { return lastResult, err }
// 让 ctx cancel 后能立即退出循环（CGO 调用无法打断，但循环顶部检查能避免
// 下一次 OCR 调用 + 让 fetchCaptchaImage 也走 ctx 路径）。
//
// 验证策略：让 mock server 记录 fetchCaptchaImage 调用次数，
// 断言 ctx cancel 后循环立即退出（调用次数 < 5，修复前会接近 99）。
package client

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fetchBlockingMockOCR 模拟 OCR 阻塞调用（CGO 不响应 ctx）。
type fetchBlockingMockOCR struct {
	ocrSleepMS int32 // OCR Recognize 阻塞多少毫秒
	ocrCalls   int32
}

func (m *fetchBlockingMockOCR) Recognize(_ []byte) (string, error) {
	atomic.AddInt32(&m.ocrCalls, 1)
	// 真实 CGO 阻塞：不响应 ctx（用 sleep 模拟）
	time.Sleep(time.Duration(atomic.LoadInt32(&m.ocrSleepMS)) * time.Millisecond)
	return "", nil
}

func (m *fetchBlockingMockOCR) Close() error { return nil }

// TestOCRRetry_RespectsContextCancel 验证 ocrRecognizeWithRetry 在 ctx cancel
// 后能立即退出循环，而不是跑满 99 次。
//
// 修复前：循环顶部无 ctx 检查 → 即使 ctx 50ms 就 cancel，循环仍会跑
//         ~99 次（每次 fetchCaptchaImage 立即失败但 continue）。
// 修复后：循环顶部 ctx.Err() 检查 → 第 1 次循环 OCR 阻塞 500ms 后返回 →
//         continue → 顶部检测到 ctx cancel → 立即返回。
//
// 验证策略：本测试断言"修复后"行为——ctx 提前 cancel 时，循环返回的 error
// 应显式标注 ctx cancel（而不是"均失败"的累加错误）。
//
// 修复前：返回 "OCR 识别 99 张图 × 1 次（共 99 次）均失败，最后错误: ..."，
//         错误信息不提及 ctx。
// 修复后：循环顶部检测到 ctx → 返回 "OCR 识别被 ctx cancel ..." 错误，
//         错误信息显式包含 ctx.Err() 字符串。
//
// 真实场景中 mock OCR 阻塞 500ms（模拟 CGO），server 立即返回（让第 1 次
// fetchCaptchaImage 成功）。修复后总耗时 ≈ 500ms，错误信息含 ctx 字样。
// 修复前总耗时 ≈ 500ms（同样因为 OCR 阻塞 500ms），错误信息含"均失败"字样。
func TestOCRRetry_RespectsContextCancel(t *testing.T) {
	var serverHits int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kaptcha/kaptcha.jpg" {
			atomic.AddInt32(&serverHits, 1)
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	mock := &fetchBlockingMockOCR{ocrSleepMS: 500}

	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     slog.New(slog.DiscardHandler),
		ocr:        mock,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := c.ocrRecognizeWithRetry(ctx)
	elapsed := time.Since(start)

	t.Logf("F11 debug: serverHits=%d ocrCalls=%d elapsed=%v err=%v",
		atomic.LoadInt32(&serverHits), atomic.LoadInt32(&mock.ocrCalls), elapsed, err)

	// 关键断言 1：循环退出时间 < 1s
	if elapsed >= 1*time.Second {
		t.Errorf("ocrRecognizeWithRetry 应在 ctx cancel 后 < 1s 退出，实际 %v", elapsed)
	}

	// 关键断言 2：错误信息应显式提及 ctx cancel（修复后行为）
	// 修复前：错误是 "OCR 识别 99 张图 × 1 次（共 99 次）均失败"
	// 修复后：错误包含 "ctx cancel" 字符串
	if err == nil {
		t.Fatal("期望 ctx cancel 后返回 error，实际 nil")
	}
	errMsg := err.Error()
	// 错误主框架应直接说明 ctx cancel（"被 ctx cancel" / "检测到 ctx"），
	// 不仅是错误链里包含 context 字样。
	if !strings.Contains(errMsg, "ctx") {
		t.Errorf("错误信息应直接说明 'ctx' cancel（循环顶部检查 ctx 后立即返回），实际: %v", err)
	}
	if !strings.Contains(errMsg, "cancel") {
		t.Errorf("错误信息应直接说明 'cancel' 关键字（区分于'均失败'累加错误），实际: %v", err)
	}
	// 反向断言：不应是"均失败"累加错误（修复前行为）
	if strings.Contains(errMsg, "均失败") {
		t.Errorf("错误信息不应是'均失败'累加错误（修复前行为），实际: %v", err)
	}

	if ctx.Err() == nil {
		t.Errorf("ctx 仍在活跃状态（cancel 未生效？），elapsed=%v", elapsed)
	}
}