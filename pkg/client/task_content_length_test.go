package client

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestBuildTaskPayload_ContentTooLong 锁定 content ≤200 字校验：
// 前端 el-input maxlength="200"（managementRightBottom.vue:389）是浏览器硬截断，
// 线上恒发 ≤200 字；SDK 不截断也不放行超长原文，与前端 wire 行为对齐为显式拒绝。
func TestBuildTaskPayload_ContentTooLong(t *testing.T) {
	var cap types.TaskAddCirclePayload
	srv := mockBizWithMetaAndCapture(t, metaForTask(2, 1), &cap)
	defer srv.Close()
	c, _ := New(WithBaseURL(srv.URL), WithSSOBase(srv.URL), WithTimeout(5*1000*1000*1000))
	defer c.Close()

	input := types.TaskSubmitInput{
		TaskID:  1001,
		Content: strings.Repeat("好", 201), // 前端会被 maxlength 硬截断到 200，SDK 必须显式拒绝
	}
	if _, err := c.buildTaskPayload(context.Background(), "tok", input, "TestSubmit", nil); !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("期望 ErrInvalidPayload，实际 err=%v", err)
	}

	// 恰好 200 字应通过（走到元数据获取之后的组装链）
	input.Content = strings.Repeat("好", 200)
	payload, err := c.buildTaskPayload(context.Background(), "tok", input, "TestSubmit", nil)
	if err != nil {
		t.Fatalf("200 字应合法，实际被拒: %v", err)
	}
	if payload.Content != strings.Repeat("好", 200) {
		t.Fatalf("200 字 content 未按原样透传: %q", payload.Content)
	}
}
