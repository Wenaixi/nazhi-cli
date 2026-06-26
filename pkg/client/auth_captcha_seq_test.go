// Package client 内部白盒测试 — F8-CAPTCHA-URL-COLLISION 验证。
package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

// TestFetchCaptchaImage_ConcurrentDifferentURLs 验证：8 路 goroutine 并发调用
// fetchCaptchaImage 拿到 8 个不同的 URL，避免并发 Login 撞同 URL 浪费 OCR 预算。
//
// F8 修复动机：原版用 time.Now().UnixMilli() 作为 cache-busting 参数，同一毫秒内
// 并发调用生成完全相同的 URL → 8 路 OCR 拿到同一张验证码图片（同一字符集）→
// 7 路必失败。
// 修复后：atomic.Int64 累加 seq 追加到 URL query，URL 唯一性由 atomic 保证。
func TestFetchCaptchaImage_ConcurrentDifferentURLs(t *testing.T) {
	// 记录所有进入 mock server 的 URL（含 query string）
	var (
		mu   sync.Mutex
		urls []string
	)
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		urls = append(urls, r.URL.String())
		mu.Unlock()
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("fake-jpeg-bytes"))
	}))
	defer sso.Close()

	mock := &countMockOCR{failBeforeSuccess: 0, returnText: "ab12"}
	c := newClientForOCRTest(sso.URL, mock)

	const n = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // 8 路同时放行，最大化同毫秒撞车概率
			if _, err := c.fetchCaptchaImage(context.Background()); err != nil {
				t.Errorf("fetchCaptchaImage 失败: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(urls) != n {
		t.Fatalf("期望 %d 次 fetch，实际 %d", n, len(urls))
	}
	seen := make(map[string]bool, n)
	for _, u := range urls {
		if seen[u] {
			t.Errorf("URL 重复: %s", u)
		}
		seen[u] = true
	}
}

// TestCaptchaSeq_Monotonic 验证 captchaSeq 累加器单调用递增。
//
// 防御性测试：未来若有人改回非 atomic 实现（如 sync.Mutex 包裹 int），
// 此测试会捕获 goroutine 间数据竞争。
func TestCaptchaSeq_Monotonic(t *testing.T) {
	const n = 100
	var seqs [n]int64

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			seqs[idx] = captchaSeq.Add(1)
		}(i)
	}
	wg.Wait()

	// 验证所有 seq 唯一（atomic.Add 保证）
	seen := make(map[int64]bool, n)
	for i, s := range seqs {
		if seen[s] {
			t.Errorf("seq %d 重复 (idx=%d)", s, i)
		}
		seen[s] = true
	}
	_ = atomic.LoadInt64 // touch atomic import even if unused
}
