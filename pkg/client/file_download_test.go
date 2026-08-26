// file_download_test.go 验证 DownloadFile 的重定向跟随 + 跨域守卫 + 上限守卫。
//
// 设计要点：
//   - 用 httptest.Server + 302 跳转模拟真实下载流（nazhisoft.com → doc.nazhisoft.com）
//   - 通过包级 var trustedHostSuffixes 在测试中临时替换为 "127.0.0.1" /
//     "evil.com"，验证同域/跨域守卫分支
//   - 验证 0 字节拒绝 / 写入失败时半成品删除
package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// withTestTrustedHosts 在测试期间临时替换 trustedHostSuffixes，结束后还原。
// 必须 defer 还原——其他并发测试若共享包级状态会被污染。
func withTestTrustedHosts(t *testing.T, suffixes []string) {
	t.Helper()
	orig := trustedHostSuffixes
	trustedHostSuffixes = suffixes
	t.Cleanup(func() { trustedHostSuffixes = orig })
}

// newDownloadTestClient 构造一个 ssoBaseURL 指向指定 srvURL 的 Client。
func newDownloadTestClient(srvURL string) *Client {
	c, _ := New(WithSSOBase(srvURL), WithTimeout(5*time.Second))
	return c
}

// TestDownloadFile_FollowsRedirect 验证基本 302 跟随：服务端 302 → 200 图片 bytes。
// 用单个 mux 同时处理 entry 和 storage 路径，让 transport 把 302 → 同 host 不同 path 跟随后
// 落到 storage handler。
func TestDownloadFile_FollowsRedirect(t *testing.T) {
	withTestTrustedHosts(t, []string{"127.0.0.1"})

	mux := http.NewServeMux()
	mux.HandleFunc("/common/attachment/getImg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/storage/test-image-bytes.bin")
		w.WriteHeader(http.StatusFound)
	})
	mux.HandleFunc("/storage/test-image-bytes.bin", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake-jpeg-bytes-for-test"))
	})
	entry := httptest.NewServer(mux)
	defer entry.Close()

	dst := t.TempDir() + "/out.bin"
	c := newDownloadTestClient(entry.URL)

	if err := c.DownloadFile(context.Background(), 12345, dst); err != nil {
		t.Fatalf("DownloadFile 失败: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("读 dst 失败: %v", err)
	}
	if string(got) != "fake-jpeg-bytes-for-test" {
		t.Errorf("dst 内容不符: 期望 %q 实际 %q", "fake-jpeg-bytes-for-test", got)
	}
}

// TestDownloadFile_RejectsCrossDomainRedirect 验证跨域重定向被拒绝。
// 服务端 302 跳转到 evil.com，必须 error 而非跟随。
func TestDownloadFile_RejectsCrossDomainRedirect(t *testing.T) {
	withTestTrustedHosts(t, []string{".trusted.test", "trusted.test"})

	entry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 跳转到 evil.com——绝对 URL，会被 isSameTrustedHost 拒绝
		w.Header().Set("Location", "http://evil.test/storage/malware.exe")
		w.WriteHeader(http.StatusFound)
	}))
	defer entry.Close()

	dst := t.TempDir() + "/out.bin"
	c := newDownloadTestClient(entry.URL)

	err := c.DownloadFile(context.Background(), 999, dst)
	if err == nil {
		t.Fatal("DownloadFile 应拒绝跨域重定向，实际 nil")
	}
	if !strings.Contains(err.Error(), "拒绝跨域重定向") {
		t.Errorf("错误信息应含 '拒绝跨域重定向'，实际: %v", err)
	}

	// 关键防御：dst 文件不应被创建（无任何写入发生）
	if _, statErr := os.Stat(dst); statErr == nil {
		t.Error("跨域拒绝时不应创建目标文件")
		_ = os.Remove(dst)
	}
}

// TestDownloadFile_RejectsTooManyRedirects 验证重定向次数超过 maxDownloadRedirects 被拒绝。
func TestDownloadFile_RejectsTooManyRedirects(t *testing.T) {
	withTestTrustedHosts(t, []string{"127.0.0.1"})

	var hop int
	entry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hop++
		// 每次都返回 302 → 同一 server 不同 path（构造循环）
		w.Header().Set("Location", "/hop?n="+itoa(hop))
		w.WriteHeader(http.StatusFound)
	}))
	defer entry.Close()

	dst := t.TempDir() + "/out.bin"
	c := newDownloadTestClient(entry.URL)

	err := c.DownloadFile(context.Background(), 1, dst)
	if err == nil {
		t.Fatal("DownloadFile 应因重定向次数过多失败，实际 nil")
	}
	if !strings.Contains(err.Error(), "重定向次数") {
		t.Errorf("错误信息应含 '重定向次数'，实际: %v", err)
	}
}

// TestDownloadFile_RejectsZeroBytes 验证服务端返回 0 字节时拒绝写入并删除半成品。
func TestDownloadFile_RejectsZeroBytes(t *testing.T) {
	withTestTrustedHosts(t, []string{"127.0.0.1"})

	entry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 直接返回 200 + 0 字节
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		// 不写任何 body
	}))
	defer entry.Close()

	dst := t.TempDir() + "/out.bin"
	c := newDownloadTestClient(entry.URL)

	err := c.DownloadFile(context.Background(), 1, dst)
	if err == nil {
		t.Fatal("DownloadFile 应拒绝 0 字节响应，实际 nil")
	}
	if !strings.Contains(err.Error(), "0 字节") {
		t.Errorf("错误信息应含 '0 字节'，实际: %v", err)
	}
	// dst 必须不存在（半成品已删除）
	if _, statErr := os.Stat(dst); statErr == nil {
		t.Errorf("0 字节响应不应留下 dst 文件: %v", dst)
		_ = os.Remove(dst)
	}
}

// TestDownloadFile_RejectsNon2xxStatus 验证 4xx/5xx 状态码被拒绝。
func TestDownloadFile_RejectsNon2xxStatus(t *testing.T) {
	withTestTrustedHosts(t, []string{"127.0.0.1"})

	entry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("attachment not found"))
	}))
	defer entry.Close()

	dst := t.TempDir() + "/out.bin"
	c := newDownloadTestClient(entry.URL)

	err := c.DownloadFile(context.Background(), 1, dst)
	if err == nil {
		t.Fatal("DownloadFile 应因 404 失败，实际 nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("错误信息应含 status code 404，实际: %v", err)
	}
}

// TestDownloadFile_Non2xxSentinelMapping 表驱动锁定非 2xx 状态码的哨兵归类：
// 403/404 等 4xx 是服务端明确拒绝（ErrInvalidResponse→422/exit1，不可重试），
// 不是网络故障（ErrNetwork→502/exit2 会诱导脚本对永久失败无限重试）；
// 429/500 验证 default 之上的 RateLimited/ServiceUnavailable 分支未被波及。
func TestDownloadFile_Non2xxSentinelMapping(t *testing.T) {
	withTestTrustedHosts(t, []string{"127.0.0.1"})

	cases := []struct {
		name       string
		status     int
		wantTarget error // 期望 errors.Is 命中的哨兵；nil 表示不关心具体哨兵但不得命中 ErrNetwork
	}{
		{"403 归 ErrInvalidResponse", http.StatusForbidden, ErrInvalidResponse},
		{"404 归 ErrInvalidResponse", http.StatusNotFound, ErrInvalidResponse},
		{"429 归 ErrRateLimited", http.StatusTooManyRequests, ErrRateLimited},
		{"500 归 ErrServiceUnavailable", http.StatusInternalServerError, ErrServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer entry.Close()

			dst := t.TempDir() + "/out.bin"
			c := newDownloadTestClient(entry.URL)

			err := c.DownloadFile(context.Background(), 1, dst)
			if err == nil {
				t.Fatalf("DownloadFile 应因 %d 失败，实际 nil", tc.status)
			}
			if !errors.Is(err, tc.wantTarget) {
				t.Errorf("status=%d 错误应 errors.Is(%v)，实际: %v", tc.status, tc.wantTarget, err)
			}
			if errors.Is(err, ErrNetwork) {
				t.Errorf("status=%d 非 2xx 不应归 ErrNetwork（网络故障语义），实际: %v", tc.status, err)
			}
		})
	}
}

// itoa 是 strconv.Itoa 的本地封装，避免在测试文件加 import。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
