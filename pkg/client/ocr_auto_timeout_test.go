// Package client 内部白盒测试。
//
// C6: OCR 总 timeout 守卫 — 回归测试。
//
// 历史问题：maxOCRImagesTotal=99 张图 × ~600ms/张 ≈ 60s+ 总耗时，
// 远超合理的登录等待时间。若调用方未在 context 中设置 deadline，
// ocrRecognizeWithRetry 可能跑满 99 张图耗时约 60s。
//
// 修复后：当 ctx 无 deadline 时自动派生 ocrTimeout (30s) 超时上下文，
// 超时后循环顶部的 ctx.Err() 检查能立即返回。
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

// slowMockOCR 模拟慢速 OCR 识别——每次调用阻塞指定毫秒数后返回空字符串。
// 用于验证 auto-timeout 正确工作：慢速 OCR 在 auto-timeout 后应停止重试。
type slowMockOCR struct {
	sleepMS int32 // 每次 OCR 调用阻塞毫秒数
	calls   int32 // 总调用次数（原子）
}

func (m *slowMockOCR) Recognize(_ []byte) (string, error) {
	atomic.AddInt32(&m.calls, 1)
	time.Sleep(time.Duration(atomic.LoadInt32(&m.sleepMS)) * time.Millisecond)
	return "", nil
}

func (m *slowMockOCR) Close() error { return nil }

// TestOCRRetry_AutoTimeoutWithoutCtxDeadline 验证：ctx 无 deadline 时
// ocrRecognizeWithRetry 自动派生 30s 超时，超时后立即返回而非跑满 99 张图。
//
// 修复前：ocrRecognizeWithRetry(context.Background()) 跑满 99 张图，
// 每张图 OCR 如果是 500ms 则总耗时约 50s，无任何超时机制。
//
// 修复后：函数入口检测 ctx.Deadline() 为 nil 时，派生 ocrTimeout (30s) 超时，
// 超时后循环顶部的 ctx.Err() 检查返回带 timeout/cancel 字样的错误。
//
// 预期行为：
//   - 总耗时 ≈ 30s + 一次 OCR 阻塞时间（约 30.5s），远 < 50s
//   - 错误信息包含 "cancel" / "timeout" / "ctx" 字样
//   - OCR 调用次数 < 99（被 timeout 中断）
func TestOCRRetry_AutoTimeoutWithoutCtxDeadline(t *testing.T) {
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

	mock := &slowMockOCR{sleepMS: 500}

	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     slog.New(slog.DiscardHandler),
		ocr:        mock,
	}

	// context.Background() 无 deadline → 触发自动派生 timeout
	start := time.Now()
	_, err := c.ocrRecognizeWithRetry(context.Background())
	elapsed := time.Since(start)

	ocrCalls := atomic.LoadInt32(&mock.calls)
	hitCount := atomic.LoadInt32(&serverHits)

	t.Logf("C6 debug: elapsed=%v err=%v ocrCalls=%d serverHits=%d",
		elapsed, err, ocrCalls, hitCount)

	// 关键断言 1：总耗时应 < 50s（修复前 ≈ 50s，修复后 ≈ 31s）
	if elapsed >= 50*time.Second {
		t.Errorf("auto-timeout 应在 < 50s 内触发，实际 %v（可能 timeout 未生效）", elapsed)
	}

	// 关键断言 2：OCR 调用次数应 < 99（被 timeout 中断而非跑满）
	if ocrCalls >= int32(maxOCRImagesTotal) {
		t.Errorf("auto-timeout 应中断 OCR 循环（实际跑满 %d 次）", maxOCRImagesTotal)
	}

	// 关键断言 3：错误应包含 cancel/timeout 字样
	if err == nil {
		t.Fatal("期望 auto-timeout 后返回 error，实际 nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "cancel") && !strings.Contains(errMsg, "timeout") && !strings.Contains(errMsg, "ctx") {
		t.Errorf("错误信息应说明 cancel/timeout（auto-timeout 中断），实际: %v", err)
	}

	// 反向断言：不应是"均失败"累加错误（修复前行为）
	if strings.Contains(errMsg, "均失败") {
		t.Errorf("错误信息不应是'均失败'累加错误（修复前行为），实际: %v", err)
	}
}
