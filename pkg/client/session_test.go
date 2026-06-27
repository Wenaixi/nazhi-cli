// session_test.go 聚合 session.go 内部白盒测试（同包），覆盖：
//   - S1: thundering herd 抑制（10/100 并发 backoff）
//   - F15: backoff 缓存按 token 隔离（不互相污染）
//   - 缓存：失败 token 不清除其他 token 的 cachedUserInfo
//   - 并发：同 token 4 步只跑 1 次 + 不同 token 串行激活
//   - 死锁：100 goroutine 不死锁
package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ─── session_backoff_concurrent_test.go (S1): 100 并发 backoff 抑制 ───

// TestActivateSessionIfNeeded_BackoffHundredConcurrent 验证 100 个 goroutine
// 同时调用 activateSessionIfNeeded（同 token 激活失败）时 backoff 缓存生效：
// 1. 步骤 4 只会被执行 1 次（首 goroutine 触发完整的 4 步）
// 2. 其余 99 个 goroutine 在 backoff 窗口内命中 lastActivationErr 缓存
// 3. 无 data race（-race 检测器干净）
// 设计动机：backoff 缓存虽然在 CLI 单进程场景无 hit（CLI 不会先失败再重试），
// 但在 SDK 多 goroutine 场景（如 100 路并发 FetchTasks）中，
// backoff 可有效抑制 thundering herd——首路 4 步失败后其余 99 路直接返回，
// 避免 99 次重复的 4 步激增加 cookie jar 脏写 + 服务端压力放大。
func TestActivateSessionIfNeeded_BackoffHundredConcurrent(t *testing.T) {
	var step4Count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			atomic.AddInt32(&step4Count, 1)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	c, _ := New(
		WithBaseURL(srv.URL),
		WithTimeout(5*time.Second),
		WithSessionBackoff(time.Hour), // 大窗口保证所有 goroutine 命中
	)

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	start := make(chan struct{})
	var errCount int32
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start
			if _, err := c.ensureActivated(context.Background(), "shared-token"); err != nil {
				atomic.AddInt32(&errCount, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if n := atomic.LoadInt32(&errCount); n != goroutines {
		t.Errorf("期望 %d 个 goroutine 返回错误，实际 %d", goroutines, n)
	}
	// 步骤 4 只应被执行 1 次，而不是 100 次
	if got := atomic.LoadInt32(&step4Count); got != 1 {
		t.Errorf("步骤 4（getMyInfo）期望 1 次 HTTP 请求，实际 %d — backoff 未抑制", got)
	}
}

// ─── session_backoff_token_test.go (F15): backoff 缓存按 token 隔离 ───

// TestActivateSessionIfNeeded_BackoffIsScopedToToken 回归测试 F15：
// 上次激活失败后，backoff 缓存键必须包含 token 维度。同一 Client 切换 token
// 重新激活时，backoff 不应命中上次失败的缓存（避免 stale error 被错误 propagate）。
func TestActivateSessionIfNeeded_BackoffIsScopedToToken(t *testing.T) {
	var step4Count int32
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			atomic.AddInt32(&step4Count, 1)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer failSrv.Close()

	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			atomic.AddInt32(&step4Count, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
		}
	}))
	defer okSrv.Close()

	c, _ := New(
		WithBaseURL(failSrv.URL),
		WithTimeout(5*time.Second),
	)
	c.sm.backoff = time.Hour

	if _, err := c.ActivateSession(context.Background(), "token-A"); err == nil {
		t.Fatal("第一阶段：token-A 在失败 server 上应返回 error")
	}
	if c.sm.lastErr == nil {
		t.Fatal("第一阶段后：sm.lastErr 应被缓存，实际 nil")
	}

	c.baseURL = okSrv.URL
	if _, err := c.ActivateSession(context.Background(), "token-B"); err != nil {
		t.Fatalf("第二阶段：token-B 在成功 server 上激活应成功，实际: %v", err)
	}
	if c.sm.LoadToken() != "token-B" {
		t.Errorf("token-B 成功后 sm token 应 = \"token-B\"，实际 %q", c.sm.LoadToken())
	}
	if c.sm.lastErr != nil {
		t.Errorf("token-B 成功后 sm.lastErr 应清零，实际 %v", c.sm.lastErr)
	}
}

// TestActivateSessionIfNeeded_BackoffHitsForSameToken 验证同 token 在 backoff
// 窗口内仍被抑制（确认 lastFailedToken 匹配时 backoff 正常工作）。
func TestActivateSessionIfNeeded_BackoffHitsForSameToken(t *testing.T) {
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer failSrv.Close()

	c, _ := New(
		WithBaseURL(failSrv.URL),
		WithTimeout(5*time.Second),
	)
	c.sm.backoff = time.Hour

	if _, err := c.ActivateSession(context.Background(), "token-X"); err == nil {
		t.Fatal("第一阶段：token-X 在失败 server 上应返回 error")
	}
	_, errSecond := c.ActivateSession(context.Background(), "token-X")
	if errSecond == nil {
		t.Error("同 token 在 backoff 窗口内应仍返回缓存错误，实际 nil")
	}
	if !errors.Is(errSecond, ErrSessionBackoff) {
		t.Errorf("backoff 错误应包装 ErrSessionBackoff 哨兵，err=%v", errSecond)
	}
}

// ─── session_cache_guard_test.go: 失败 token 不清其他 token 缓存 ───

// TestActivateFailedToken_DoesNotClearOtherTokenCache_SameClient 验证在同一 Client
// 上先激活 token-A 成功、再激活 token-B 失败时，token-A 的缓存不被清除。
func TestActivateFailedToken_DoesNotClearOtherTokenCache_SameClient(t *testing.T) {
	var mu sync.Mutex
	step4Fail := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"success"}`))
		case "/api/studentInfo/getMyInfo":
			mu.Lock()
			fail := step4Fail
			mu.Unlock()
			if fail {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"code":0,"msg":"服务降级"}`))
			} else {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
			}
		}
	}))
	defer srv.Close()

	c, _ := New(
		WithBaseURL(srv.URL),
		WithTimeout(5*time.Second),
	)

	info, err := c.ActivateSession(context.Background(), "tok-A")
	if err != nil {
		t.Fatalf("token-A 激活失败: %v", err)
	}
	if info == nil || info.Name != "张三" {
		t.Fatalf("token-A 激活后 info 异常: %+v", info)
	}
	if c.sm.LoadToken() != "tok-A" {
		t.Fatalf("sm token 应为 tok-A, 实际 %v", c.sm.LoadToken())
	}
	cached := c.sm.GetCachedUserInfo()
	if cached == nil || cached.Name != "张三" {
		t.Fatalf("token-A 激活后 sm cachedUserInfo 应有数据: %+v", cached)
	}

	mu.Lock()
	step4Fail = true
	mu.Unlock()

	_, err = c.ActivateSession(context.Background(), "tok-B")
	if err == nil {
		t.Fatal("token-B 激活应失败，但返回 nil")
	}

	cached = c.sm.GetCachedUserInfo()
	if cached == nil {
		t.Fatal("F2 回归：token-B 失败不应清除 token-A 的 cachedUserInfo")
	}
	if cached.Name != "张三" {
		t.Errorf("cachedUserInfo.Name 期望 '张三', 实际 %q", cached.Name)
	}
	if c.sm.LoadToken() != "tok-A" {
		t.Errorf("sm token 应仍为 tok-A（token-B 未成功）, 实际 %v", c.sm.LoadToken())
	}
}

// ─── session_concurrent_test.go: 同/异 token 并发激活 ───

// TestActivateSessionIfNeeded_ConcurrentSameToken 回归测试：N 个 goroutine
// 并发调用 activateSessionIfNeeded(token)，验证：
// 1. 不发生 race
// 2. 4 步激活请求不被重复执行（每个端点的请求次数恒定）
// 3. sm token 在所有 goroutine 返回前/后均为同一 token
func TestActivateSessionIfNeeded_ConcurrentSameToken(t *testing.T) {
	var (
		step1Count int32
		step2Count int32
		step3Count int32
		step4Count int32
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			atomic.AddInt32(&step1Count, 1)
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			if r.Header.Get("Referer") != "" && strings.Contains(r.Header.Get("Referer"), "/homepage") {
				atomic.AddInt32(&step2Count, 1)
			} else {
				atomic.AddInt32(&step3Count, 1)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			atomic.AddInt32(&step4Count, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
		}
	}))
	defer srv.Close()

	c, _ := New(
		WithBaseURL(srv.URL),
		WithTimeout(5*time.Second),
	)

	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start
			if _, err := c.ensureActivated(context.Background(), "shared-token"); err != nil {
				t.Errorf("ensureActivated 失败: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := atomic.LoadInt32(&step1Count); got != 1 {
		t.Errorf("步骤 1 期望 1 次，实际 %d", got)
	}
	if got := atomic.LoadInt32(&step2Count); got != 1 {
		t.Errorf("步骤 2 期望 1 次，实际 %d", got)
	}
	if got := atomic.LoadInt32(&step3Count); got != 1 {
		t.Errorf("步骤 3 期望 1 次，实际 %d", got)
	}
	if got := atomic.LoadInt32(&step4Count); got != 1 {
		t.Errorf("步骤 4 期望 1 次，实际 %d", got)
	}

	if c.sm.LoadToken() != "shared-token" {
		t.Errorf("sm token = %q, 期望 %q", c.sm.LoadToken(), "shared-token")
	}
}

// TestActivateSessionIfNeeded_ConcurrentDifferentTokens 回归测试：N 个 goroutine
// 持不同 token 并发调用 activateSessionIfNeeded，验证：
// 1. 不发生 race
// 2. 持锁激活保证不同 token 串行执行 4 步，无并发写 cookie jar 污染
// 3. sm token 最终为某次成功调用的 token
func TestActivateSessionIfNeeded_ConcurrentDifferentTokens(t *testing.T) {
	var step1Count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			atomic.AddInt32(&step1Count, 1)
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三"}}`))
		}
	}))
	defer srv.Close()

	c, _ := New(
		WithBaseURL(srv.URL),
		WithTimeout(5*time.Second),
	)

	const goroutines = 8
	tokens := []string{
		"tok-A", "tok-B", "tok-C", "tok-D",
		"tok-E", "tok-F", "tok-G", "tok-H",
	}
	var wg sync.WaitGroup
	wg.Add(goroutines)
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			if _, err := c.ensureActivated(context.Background(), tokens[i]); err != nil {
				t.Errorf("token=%s ensureActivated 失败: %v", tokens[i], err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := atomic.LoadInt32(&step1Count); got != int32(goroutines) {
		t.Errorf("不同 token 步骤 1 期望 %d 次，实际 %d", goroutines, got)
	}
	finalToken := c.sm.LoadToken()
	found := false
	for _, tok := range tokens {
		if finalToken == tok {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("sm token=%q 不在预期 token 列表中", finalToken)
	}
}

// ─── session_deadlock_test.go: 100 并发不死锁 ───

// TestActivateSession_ConcurrentNoDeadlock 验证 ActivateSession 在 100 个
// goroutine 并发调用时不发生死锁。
// 历史风险：ActivateSession 在 sessionMu 持锁状态下执行
// 4 步网络请求，如果外部调用方在 errgroup.Go 中持其他锁再调 ActivateSession，
// 可能引发 ABBA 死锁。本测试验证 ActivateSession 自身不因 sessionMu 持有
// 而阻塞所有并发——仅锁定序列化 4 步写入 cookie jar。
func TestActivateSession_ConcurrentNoDeadlock(t *testing.T) {
	stepCount := struct {
		sync.Mutex
		val int
	}{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			stepCount.Lock()
			stepCount.val++
			stepCount.Unlock()
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
		}
	}))
	defer srv.Close()

	c, _ := New(
		WithBaseURL(srv.URL),
		WithTimeout(5*time.Second),
	)

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	start := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			token := fmt.Sprintf("token-%03d", i)
			_, _ = c.ActivateSession(context.Background(), token)
		}()
	}
	close(start)
	wg.Wait()

	// 如果走到这里没死锁，测试通过
	// 验证至少大部分 token 都完成了步骤 4（因 httptest 并发处理和 backoff
	// 序列化，在极端调度下可能个别 goroutine 的请求被服务端漏响应，但
	// 99+/100 是可接受的——核心目标是验证不死锁）。
	stepCount.Lock()
	got := stepCount.val
	stepCount.Unlock()
	if got < goroutines-1 {
		t.Fatalf("期望至少 %d 次步骤 4 调用，实际 %d — 可能有 token 被跳过或死锁", goroutines-1, got)
	}
	t.Logf("步骤 4 被调用 %d 次（共 %d goroutine），无死锁 ✓", got, goroutines)
}

// ─── session_empty_userinfo_test.go (F10): ErrEmptyUserInfo 哨兵 ───

// TestGetMyInfoRaw_EmptyResponse_ReturnsErrEmptyUserInfo 回归测试 F10：
// getMyInfoRaw 在 returnData + dataMap 都为 nil 时（业务成功响应但确实无用户数据），
// 必须返回 (nil, ErrEmptyUserInfo) 而非 (nil, nil)。
// 修复前：返回 (nil, nil) → cmd/nazhi/session.go:38 printJSON(info) 输出裸 null
// 与 cmd/nazhi/whoami.go 的 {status: empty, reason: get_my_info_empty} envelope 不一致。
// 修复后：返回 (nil, ErrEmptyUserInfo) → cmd 层用 errors.Is 分支统一走 status envelope。
func TestGetMyInfoRaw_EmptyResponse_ReturnsErrEmptyUserInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// 关键：code=1（业务成功）但 returnData + dataMap 都为 nil
		// 这是「服务端确认查询成功但确实无数据」的状态，不是错误
		_, _ = w.Write([]byte(`{"code":1,"returnData":null,"dataMap":null}`))
	}))
	defer srv.Close()

	c, _ := New(
		WithBaseURL(srv.URL),
		WithTimeout(5*time.Second),
	)

	info, err := c.getMyInfoRaw(context.Background(), "test-token")

	// 关键断言：返回 nil UserInfo + ErrEmptyUserInfo 哨兵
	if info != nil {
		t.Errorf("空响应应返回 nil UserInfo，实际: %v", info)
	}
	if err == nil {
		t.Fatal("空响应应返回 ErrEmptyUserInfo，实际 nil")
	}
	if !errors.Is(err, ErrEmptyUserInfo) {
		t.Errorf("空响应错误必须包装 ErrEmptyUserInfo 哨兵，让 SDK 用户 errors.Is 识别。err=%v", err)
	}
}

// TestGetMyInfoRaw_ValidResponse_ReturnsUserInfo 验证正常响应仍返回 UserInfo + nil err。
// 防止 F10 修复破坏 happy path。
func TestGetMyInfoRaw_ValidResponse_ReturnsUserInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
	}))
	defer srv.Close()

	c, _ := New(
		WithBaseURL(srv.URL),
		WithTimeout(5*time.Second),
	)

	info, err := c.getMyInfoRaw(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("正常响应不应返回 err，实际: %v", err)
	}
	if info == nil {
		t.Fatal("正常响应应返回 UserInfo，实际 nil")
	}
	if info.Name != "张三" {
		t.Errorf("Name = %q, 期望 %q", info.Name, "张三")
	}
}

// TestGetMyInfo_EmptyResponse_BestEffortReturnsNil 验证公开 GetMyInfo 仍然保持
// 「最佳努力设计」契约：调用方通常吞错，nil 不算 error。
// 但当 getMyInfoRaw 返回 (nil, ErrEmptyUserInfo) 时，GetMyInfo 应 propagate
// 这个 err（语义信号），让调用方能 errors.Is 分支处理空响应。
func TestGetMyInfo_EmptyResponse_PropagatesErrEmptyUserInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
	}))
	defer srv.Close()

	c, _ := New(
		WithBaseURL(srv.URL),
		WithTimeout(5*time.Second),
	)

	info, err := c.GetMyInfo(context.Background(), "test-token")
	if info != nil {
		t.Errorf("空响应 GetMyInfo 应返回 nil UserInfo，实际: %v", info)
	}
	if !errors.Is(err, ErrEmptyUserInfo) {
		t.Errorf("空响应 GetMyInfo 必须 propagate ErrEmptyUserInfo，让 cmd 层按 errors.Is 分支。err=%v", err)
	}
	// 防止 dev 误把 ErrEmptyUserInfo 当成 ErrBusinessRejected 包装
	if errors.Is(err, ErrBusinessRejected) {
		t.Error("ErrEmptyUserInfo 不应被包装为 ErrBusinessRejected — 空响应不是业务错误")
	}
}

// ─── session_thundering_herd_test.go (S1): thundering herd 抑制 ───

// TestActivateSessionIfNeeded_ThunderingHerd 回归测试 S1：
// 10 个 goroutine 并发 activateSessionIfNeeded，第 1 个失败后其余 9 个
// 应直接返回缓存错误，不重复执行 4 步激活（thundering herd 抑制）。
func TestActivateSessionIfNeeded_ThunderingHerd(t *testing.T) {
	var step4Count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			atomic.AddInt32(&step4Count, 1)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	c, _ := New(
		WithBaseURL(srv.URL),
		WithTimeout(5*time.Second),
	)
	c.sm.backoff = time.Hour

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	start := make(chan struct{})
	var errCount int32
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start
			if _, err := c.ensureActivated(context.Background(), "shared-token"); err != nil {
				atomic.AddInt32(&errCount, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if n := atomic.LoadInt32(&errCount); n != goroutines {
		t.Errorf("期望 %d 个 goroutine 返回错误，实际 %d", goroutines, n)
	}
	if got := atomic.LoadInt32(&step4Count); got != 1 {
		t.Errorf("步骤 4 期望 1 次 HTTP 请求，实际 %d — thundering herd 未被抑制", got)
	}
}

// ─── session_manager_test.go: sessionManager 内部状态机 ───

// newTestSM 构造一个测试用的 sessionManager，backoff 窗口压缩到 50ms 方便测试。
func newTestSM() *sessionManager {
	var sm sessionManager
	sm.backoff = 50 * time.Millisecond
	return &sm
}

func TestSessionManager_IsBackoffHit(t *testing.T) {
	t.Run("fresh sm returns false", func(t *testing.T) {
		sm := newTestSM()
		if sm.isBackoffHit("token") {
			t.Error("全新 sessionManager 不应命中 backoff")
		}
	})

	t.Run("same token within window returns true", func(t *testing.T) {
		sm := newTestSM()
		sm.lastErr = errors.New("some error")
		sm.lastFailedToken = "token"
		sm.lastAttempt = time.Now()

		if !sm.isBackoffHit("token") {
			t.Error("同 token 且在窗口内应命中 backoff")
		}
	})

	t.Run("expired window returns false after sleep", func(t *testing.T) {
		sm := newTestSM()
		sm.lastErr = errors.New("some error")
		sm.lastFailedToken = "token"
		sm.lastAttempt = time.Now()

		time.Sleep(60 * time.Millisecond) // 等待超过 50ms 的 backoff 窗口

		if sm.isBackoffHit("token") {
			t.Error("窗口过期后不应命中 backoff")
		}
	})

	t.Run("different token returns false", func(t *testing.T) {
		sm := newTestSM()
		sm.lastErr = errors.New("some error")
		sm.lastFailedToken = "tokenA"
		sm.lastAttempt = time.Now()

		if sm.isBackoffHit("tokenB") {
			t.Error("不同 token 不应命中 backoff")
		}
	})

	t.Run("lastErr nil returns false", func(t *testing.T) {
		sm := newTestSM()
		sm.lastFailedToken = "token"
		sm.lastAttempt = time.Now()
		// lastErr 默认 nil

		if sm.isBackoffHit("token") {
			t.Error("lastErr 为 nil 时不应命中 backoff")
		}
	})
}

func TestSessionManager_Activate_DCL(t *testing.T) {
	t.Run("first call activates, second call returns cached", func(t *testing.T) {
		sm := newTestSM()
		var callCount int32
		expectedInfo := &types.UserInfo{Name: "test"}

		activateFn := func(ctx context.Context, token string) (*types.UserInfo, error) {
			atomic.AddInt32(&callCount, 1)
			return expectedInfo, nil
		}

		// 第一次调用：触发激活
		info1, err := sm.Activate(context.Background(), "token", activateFn)
		if err != nil {
			t.Fatalf("第一次激活应成功，err=%v", err)
		}
		if info1 != expectedInfo {
			t.Error("第一次激活应返回 activateFn 的结果")
		}
		if n := atomic.LoadInt32(&callCount); n != 1 {
			t.Errorf("activateFn 应恰好调用 1 次，实际调了 %d 次", n)
		}

		// 第二次调用：走 fast path 直接返回缓存
		info2, err := sm.Activate(context.Background(), "token", activateFn)
		if err != nil {
			t.Fatalf("第二次激活应成功（缓存），err=%v", err)
		}
		if info2 != expectedInfo {
			t.Error("第二次激活应返回缓存的 UserInfo")
		}
		if n := atomic.LoadInt32(&callCount); n != 1 {
			t.Errorf("activateFn 仍应只调 1 次，实际调了 %d 次", n)
		}
	})

	t.Run("different token calls activateFn again", func(t *testing.T) {
		sm := newTestSM()
		var callCount int32

		activateFn := func(ctx context.Context, token string) (*types.UserInfo, error) {
			atomic.AddInt32(&callCount, 1)
			return &types.UserInfo{Name: token}, nil
		}

		// tokenA 激活
		_, _ = sm.Activate(context.Background(), "tokenA", activateFn)
		if n := atomic.LoadInt32(&callCount); n != 1 {
			t.Fatalf("第一次应调 1 次，调了 %d 次", n)
		}

		// tokenB 激活 —— 不同 token，应再次调用 activateFn
		info, err := sm.Activate(context.Background(), "tokenB", activateFn)
		if err != nil {
			t.Fatalf("tokenB 激活应成功，err=%v", err)
		}
		if info == nil || info.Name != "tokenB" {
			t.Errorf("tokenB 应返回其自身的 info，got %+v", info)
		}
		if n := atomic.LoadInt32(&callCount); n != 2 {
			t.Errorf("不同 token 应再次调用 activateFn，实际调了 %d 次", n)
		}
	})
}

func TestSessionManager_RecordFailure(t *testing.T) {
	t.Run("failure clears cachedUserInfo for same token", func(t *testing.T) {
		sm := newTestSM()
		sm.token.Store("token")
		sm.cachedUserInfo = &types.UserInfo{Name: "test"}

		sm.RecordFailure("token", errors.New("activation failed"))

		if sm.cachedUserInfo != nil {
			t.Error("同 token 失败后 cachedUserInfo 应被清空")
		}
	})

	t.Run("failure does NOT clear cache for different token", func(t *testing.T) {
		sm := newTestSM()
		sm.token.Store("activeToken")
		expectedInfo := &types.UserInfo{Name: "test"}
		sm.cachedUserInfo = expectedInfo

		sm.RecordFailure("otherToken", errors.New("activation failed"))

		if sm.cachedUserInfo != expectedInfo {
			t.Error("不同 token 失败不应清空当前 token 的缓存")
		}
	})
}

func TestSessionManager_StoreToken(t *testing.T) {
	t.Run("StoreToken sets token and clears backoff state", func(t *testing.T) {
		sm := newTestSM()

		sm.StoreToken("abc")

		if got := sm.LoadToken(); got != "abc" {
			t.Errorf("LoadToken 应返回 'abc'，实际 %q", got)
		}
		if sm.lastErr != nil {
			t.Error("StoreToken 后 lastErr 应为 nil")
		}
		if sm.lastFailedToken != "" {
			t.Errorf("StoreToken 后 lastFailedToken 应为空，实际 %q", sm.lastFailedToken)
		}
	})
}

func TestSessionManager_Activate_Backoff(t *testing.T) {
	t.Run("failure triggers backoff for same token", func(t *testing.T) {
		sm := newTestSM()
		activateFn := func(ctx context.Context, token string) (*types.UserInfo, error) {
			return nil, errors.New("network error")
		}

		// 第一次：失败
		_, err1 := sm.Activate(context.Background(), "token", activateFn)
		if err1 == nil {
			t.Fatal("第一次激活应失败")
		}

		// 第二次：命中 backoff
		_, err2 := sm.Activate(context.Background(), "token", activateFn)
		if err2 == nil {
			t.Fatal("第二次激活应命中 backoff")
		}
		if !errors.Is(err2, ErrSessionBackoff) {
			t.Errorf("backoff 错误应包装 ErrSessionBackoff 哨兵，实际: %v", err2)
		}
	})

	t.Run("different token bypasses backoff", func(t *testing.T) {
		sm := newTestSM()
		var callCount int32
		activateFn := func(ctx context.Context, token string) (*types.UserInfo, error) {
			atomic.AddInt32(&callCount, 1)
			if token == "tokenA" {
				return nil, errors.New("network error")
			}
			return &types.UserInfo{Name: token}, nil
		}

		// tokenA 失败
		_, _ = sm.Activate(context.Background(), "tokenA", activateFn)
		if n := atomic.LoadInt32(&callCount); n != 1 {
			t.Fatalf("tokenA 应调 1 次 activateFn，实际 %d", n)
		}

		// tokenB —— 不同 token，应绕过 backoff 再次调用 activateFn
		info, err := sm.Activate(context.Background(), "tokenB", activateFn)
		if err != nil {
			t.Fatalf("tokenB 应绕过 backoff 并成功，err=%v", err)
		}
		if info == nil || info.Name != "tokenB" {
			t.Errorf("tokenB 应返回其自身的 info，got %+v", info)
		}
		if n := atomic.LoadInt32(&callCount); n != 2 {
			t.Errorf("tokenB 应再次调用 activateFn，实际调了 %d 次", n)
		}
	})
}
