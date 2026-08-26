package client

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestDeleteBatchTypicalCase_EmptyGuards 锁定 typical_case.go:213-215 的
// 空/nil 切片守卫：返回 ErrInvalidPayload 不发出字面 null 请求体。
// 该分支对齐前端 classiccanter.vue:497-503 空选择拦截 + CLI 层解析守卫；
// commit 1522446 的修复本体必须有客户端层回归保护，以防重构误删。
//
// 注意：仅覆盖守卫触发的两条输入（nil / 空切片）；含正数 id 的输入属于正常路径，
// 应发出 wire 请求并由端点服务器响应，不在本测试覆盖范围。
func TestDeleteBatchTypicalCase_EmptyGuards(t *testing.T) {
	cases := []struct {
		name string
		ids  []int64
	}{
		{"nil", nil},
		{"empty slice", []int64{}},
	}
	okResp := []byte("{\"code\":1,\"msg\":\"ok\"}")
	userResp := []byte("{\"code\":1,\"msg\":\"ok\",\"returnData\":{\"name\":\"test\"}}")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 区分预期预热请求与意外业务请求的服务器。
			// 预热路径 / / getMenu / getMyInfo 返回 200，业务路径必须零命中。
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/", "/api/studentInfo/getMenu":
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(okResp)
				case "/api/studentInfo/getMyInfo":
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(userResp)
				default:
					t.Errorf("unexpected business request to %s; empty input must not hit deleteBatchTypicalCase", r.URL.Path)
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
			defer srv.Close()
			c, _ := New(WithBaseURL(srv.URL), WithSSOBase(srv.URL), WithTimeout(5*time.Second))
			defer c.Close()

			err := c.DeleteBatchTypicalCase(t.Context(), "tok", tc.ids)
			if err == nil {
				t.Fatalf("want ErrInvalidPayload got nil")
			}
			if !errors.Is(err, ErrInvalidPayload) {
				t.Fatalf("want ErrInvalidPayload got %v", err)
			}
			if !strings.Contains(err.Error(), "批量删除需要至少一个 id") {
				t.Fatalf("want err contains 批量删除需要至少一个 id got %q", err.Error())
			}
		})
	}
}
