package client_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestUpdateTypicalCase_TooLongTextRejected 锁定：UpdateTypicalCase
// 的 map 路径此前无 remark/content rune 长度校验（AddTypicalCase 有 198/1500）。
// 长度纪律不对称：Add 严、Update 松，超长原文原样上 wire。
//
// 断言：remark 199 rune 拒绝（ErrInvalidPayload 且不发业务请求）；content 1501
// rune 拒绝；边界 198/1500 放行（走到业务层，mock 404 是预期非 ErrInvalidPayload）。
func TestUpdateTypicalCase_TooLongTextRejected(t *testing.T) {
	hit := false
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/updateTypicalCase" {
			hit = true
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.NotFound(w, r)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	base := map[string]any{"id": int64(1), "title": "t", "remark": "r", "content": "c"}

	t.Run("remark 199 rune 拒绝", func(t *testing.T) {
		hit = false
		p := cloneMap(base)
		p["remark"] = strings.Repeat("注", 199)
		err := c.UpdateTypicalCase(context.Background(), "test-token", p)
		if err == nil {
			t.Fatal("超长 remark 应被拒绝")
		}
		if !errors.Is(err, client.ErrInvalidPayload) {
			t.Fatalf("超长 remark 应包 ErrInvalidPayload，实际: %v", err)
		}
		if hit {
			t.Fatal("超长 remark 不应发出业务请求")
		}
	})

	t.Run("content 1501 rune 拒绝", func(t *testing.T) {
		hit = false
		p := cloneMap(base)
		p["content"] = strings.Repeat("正", 1501)
		err := c.UpdateTypicalCase(context.Background(), "test-token", p)
		if err == nil {
			t.Fatal("超长 content 应被拒绝")
		}
		if !errors.Is(err, client.ErrInvalidPayload) {
			t.Fatalf("超长 content 应包 ErrInvalidPayload，实际: %v", err)
		}
		if hit {
			t.Fatal("超长 content 不应发出业务请求")
		}
	})

	t.Run("边界 198/1500 放行（非 ErrInvalidPayload）", func(t *testing.T) {
		p := cloneMap(base)
		p["remark"] = strings.Repeat("注", 198)
		p["content"] = strings.Repeat("正", 1500)
		err := c.UpdateTypicalCase(context.Background(), "test-token", p)
		if err == nil || errors.Is(err, client.ErrInvalidPayload) {
			t.Fatalf("边界应放行到业务层（非 ErrInvalidPayload），实际: %v", err)
		}
	})
}

func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
