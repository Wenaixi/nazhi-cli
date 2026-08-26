package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestDoBizAndDecode_EmptyBody_InvalidResponse 锁定哨兵口径一致性：
// 200+HTML 与 200+空 body 都必须归 ErrInvalidResponse（传输层截断/网关异常不是业务裁决）。
// 空输入时 encoding/json 的 json.Unmarshal 返回 "unexpected end of JSON input"，
// 因此 doBizAndDecode :231-234 的解析失败分支天然覆盖空 body——本测试防止未来
// 有人把该分支改成「静默零值放行」或绕过 decodeOrInvalidResponse 时悄悄回归。
func TestDoBizAndDecode_EmptyBody_InvalidResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// 零字节 body
	}))
	defer srv.Close()

	c, err := New(WithBaseURL(srv.URL), WithToken("test-token"))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	_, err = c.doBizAndDecode(context.Background(), "test-token", "OpTest", "/api/test", http.MethodGet, nil)
	if err == nil {
		t.Fatal("200+空 body 应返回错误，实际 nil")
	}
	if !errors.Is(err, ErrInvalidResponse) {
		t.Errorf("200+空 body 应归 ErrInvalidResponse（与 200+非 JSON 口径一致），实际: %v", err)
	}
}
