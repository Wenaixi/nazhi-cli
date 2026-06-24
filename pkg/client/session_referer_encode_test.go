// Package client_test — session.go 步骤 2 Referer token URL 编码回归测试。
package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestActivateSession_Step2RefererEncodesToken 回归测试（F1）：
// 步骤 2 的 Referer 中 token 字段必须经过 URL 编码。
//
// 历史 bug：session.go:36 步骤 2 用 c.baseURL+"/homepage?token="+token
// 直接拼接，token 若包含 &、=、空格等字符会破坏 Referer URL 结构。
// JWT/cookie 等含 base64 字符的 token 虽不直接含 &，但 Referer 头被
// 浏览器/代理/服务端日志记录是普遍现象，未编码会引发：
//  1. 中间代理把 Referer 当 URL 解析失败
//  2. 服务端日志把 Referer 拆成多个 key=value
//  3. 防御性编程契约：URL 查询参数必须编码
//
// 修复后：使用 url.Values{"token": {token}}.Encode() 编码，特殊字符
// 会被 % 转义。
func TestActivateSession_Step2RefererEncodesToken(t *testing.T) {
	// 构造一个含 & 和 = 的 token，验证它们必须被编码为 %26 / %3D
	const rawToken = "abc&def=ghi"
	// 期望编码后: abc%26def%3Dghi（保留 = -> %3D，& -> %26）
	const wantEncoded = "abc%26def%3Dghi"

	var gotReferer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 只捕获第一个 getMenu 请求（步骤 2），通过路径精确匹配
		if strings.HasSuffix(r.URL.Path, "/getMenu") && strings.Contains(r.Header.Get("Referer"), "homepage") {
			gotReferer = r.Header.Get("Referer")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch r.URL.Path {
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
		default:
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		}
	}))
	defer srv.Close()

	c, _ := client.New(
		client.WithBaseURL(srv.URL),
		client.WithTimeout(5*time.Second),
	)
	_, _ = c.ActivateSession(context.Background(), rawToken)

	if gotReferer == "" {
		t.Fatal("步骤 2 getMenu 请求未发出，Referer 为空")
	}

	// 1. 原始特殊字符（& 和 =）不应直接出现在 Referer 中
	if strings.Contains(gotReferer, "abc&def") {
		t.Errorf("步骤 2 Referer 含未编码的 & 字符，会破坏 URL 结构: %s", gotReferer)
	}
	if strings.Contains(gotReferer, "=ghi") {
		// 注意：要排除 token= 自身的 =，所以精确匹配 "=ghi" 形式
		t.Errorf("步骤 2 Referer 含未编码的 =ghi 形式: %s", gotReferer)
	}

	// 2. 编码后的 token 必须出现在 Referer 中
	if !strings.Contains(gotReferer, wantEncoded) {
		t.Errorf("步骤 2 Referer 应含编码后的 token %q，实际: %s", wantEncoded, gotReferer)
	}

	// 3. Referer 必须保持 homepage?token= 前缀结构
	if !strings.Contains(gotReferer, "homepage?token=") {
		t.Errorf("步骤 2 Referer 应含 'homepage?token=' 前缀，实际: %s", gotReferer)
	}
}
