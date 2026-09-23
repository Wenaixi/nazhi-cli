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

// CLI-110-1：self-eval submit / grad-submit 的 --comment 此前无 rune 长度校验，
// 超长原文原样上 wire。官方前端 mainLeft.vue:26/39 两处 textarea 均为
// maxlength="700"（浏览器硬截断保证线上恒发 ≤700 字）。SDK 侧补
// validateSelfEvalComment 700 rune 上限显式拒绝（ErrInvalidPayload → 400/exit3），
// 与典型-case 198/1500、task content 200 的纪律同族。

// TestSubmitSelfEvaluation_TooLongCommentRejected 锁定：SubmitSelfEvaluation
// 超长 700 rune 返回 ErrInvalidPayload 且不发业务请求；恰好 700 边界放行（走到
// 业务层，mock 404 是预期不是 ErrInvalidPayload）。结构化 --payload 路径不受限。
func TestSubmitSelfEvaluation_TooLongCommentRejected(t *testing.T) {
	hit := false
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentMoralEduNew/addSelfEvaluation" {
			hit = true
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.NotFound(w, r)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)

	t.Run("超长 701 rune 拒绝", func(t *testing.T) {
		err := c.SubmitSelfEvaluation(context.Background(), "test-token", strings.Repeat("评", 701))
		if err == nil {
			t.Fatal("超长评价应被拒绝")
		}
		if !errors.Is(err, client.ErrInvalidPayload) {
			t.Fatalf("超长评价应包 ErrInvalidPayload，实际: %v", err)
		}
		if hit {
			t.Fatal("超长评价不应发出业务请求")
		}
	})

	t.Run("边界 700 rune 放行（非 ErrInvalidPayload）", func(t *testing.T) {
		err := c.SubmitSelfEvaluation(context.Background(), "test-token", strings.Repeat("评", 700))
		if err == nil || errors.Is(err, client.ErrInvalidPayload) {
			t.Fatalf("边界 700 应放行到业务层（非 ErrInvalidPayload），实际: %v", err)
		}
	})
}

// TestSubmitSelfGradEvaluation_TooLongCommentRejected 锁定毕业评价同款上限。
func TestSubmitSelfGradEvaluation_TooLongCommentRejected(t *testing.T) {
	hit := false
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentMoralEduNew/addSelfGradEvaluation" {
			hit = true
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.NotFound(w, r)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)

	err := c.SubmitSelfGradEvaluation(context.Background(), "test-token", strings.Repeat("毕", 701))
	if err == nil {
		t.Fatal("超长毕业评价应被拒绝")
	}
	if !errors.Is(err, client.ErrInvalidPayload) {
		t.Fatalf("超长毕业评价应包 ErrInvalidPayload，实际: %v", err)
	}
	if hit {
		t.Fatal("超长毕业评价不应发出业务请求")
	}
}
