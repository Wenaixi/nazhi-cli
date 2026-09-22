package client_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// I3-05：典型案例 remark/content 无长度校验。
// 官方前端 classiccanter.vue:124/130 maxlength=198(remark)/1500(content) 为
// 浏览器硬截断，线上恒发 ≤198/≤1500 字；SDK 原样透传超长 wire（对齐 task
// content 的 maxTaskContentRunes=200 拒绝纪律），让服务端裁决。
// 修复：SDK 层按 rune 长度显式拒绝（ErrInvalidPayload），remark≤198、content≤1500，
// 与前端 wire 行为对齐为显式拒绝而非静默截断/放行。

// TestAddTypicalCase_TooLongRemarkContentRejected 锁定 I3-05：
// AddTypicalCase 对超长 remark/content 返回 ErrInvalidPayload 且不发业务请求。
// 对齐 task content 的 errors.Is(err, ErrInvalidPayload) 契约（CLI 漏斗归 400/exit3）。
func TestAddTypicalCase_TooLongRemarkContentRejected(t *testing.T) {
	hit := false
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/addTypicalCase" {
			hit = true
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.NotFound(w, r)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)

	// 先构造合法请求确认 mock 路径可达；都超长时贴到有语义的失败上。
	t.Run("content 超长 1500 拒绝", func(t *testing.T) {
		err := c.AddTypicalCase(context.Background(), "test-token", types.AddTypicalCasePayload{
			Title:   "标题",
			Content: strings.Repeat("长", 1501),
		})
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

	t.Run("remark 超长 198 拒绝", func(t *testing.T) {
		err := c.AddTypicalCase(context.Background(), "test-token", types.AddTypicalCasePayload{
			Title:  "标题",
			Remark: strings.Repeat("备注", 99) + "注",
		})
		if err == nil {
			t.Fatal("超长 remark 应被拒绝")
		}
		if !errors.Is(err, client.ErrInvalidPayload) {
			t.Fatalf("超长 remark 应包 ErrInvalidPayload，实际: %v", err)
		}
	})

	t.Run("边界长度放行（不许发生业务拒绝）", func(t *testing.T) {
		// criticism：边界内容必须真正到达 addTypicalCase（mock 返回 404 网络层拒绝
		// 是预期，只要不是 ErrInvalidPayload 即说明长度校验放行）。放行后与预热
		// 漏斗同构：content=1500 与 remark=198 恰好合法。
		err := c.AddTypicalCase(context.Background(), "test-token", types.AddTypicalCasePayload{
			Title:   "标题",
			Content: strings.Repeat("正", 1500),
			Remark:  strings.Repeat("备", 198),
		})
		if err == nil || errors.Is(err, client.ErrInvalidPayload) {
			t.Fatalf("边界长度应放行到业务层（非 ErrInvalidPayload），实际: %v", err)
		}
	})
}
