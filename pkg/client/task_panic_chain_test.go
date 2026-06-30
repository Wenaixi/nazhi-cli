// Package client 白盒测试：F10.1 fetchTasksForDimensionSafe panic recover 错误链保留。
//
// 修复动机：fetchTasksForDimensionSafe 的 defer recover 用 `%v` 格式化 r（any 类型），
// 当 r 是 error 时会丢失 chain，导致 SDK 用户无法 errors.Is 识别 panic 根因。
//
// 修复策略：把 recover→error 包装抽成独立 helper wrapPanicAsErr，方便 100% 覆盖两条分支：
//   - r 是 error → fmt.Errorf("...: %w", r) 保留 chain
//   - r 不是 error → fmt.Errorf("...: %v", r) 退化路径
//
// ponytail: helper 提取让生产逻辑 + 测试都极简（一行调用），避免给 fetchTasksForDimension
// 注入测试钩子（污染生产代码）。
package client

import (
	"errors"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestWrapPanicAsErr_ErrorType_PreservesChain 验证 r 是 error 时走 %w 保留 chain。
//
// RED 动机：当前 fetchTasksForDimensionSafe 用 `%v` 格式化 r，无论 r 是 string 还是
// error 都退化成字符串，errors.Is 永远命中不到 panic 里的根因 error。
func TestWrapPanicAsErr_ErrorType_PreservesChain(t *testing.T) {
	root := errors.New("底层 panic 根因: context deadline exceeded")
	got := wrapPanicAsErr(types.Dimension{ID: 42, Name: "教学维度"}, root)

	if got == nil {
		t.Fatal("wrapPanicAsErr 应返回非 nil error")
	}
	if !errors.Is(got, root) {
		t.Errorf("r 是 error 时 wrapPanicAsErr 应保留 chain，errors.Is 命中 root；实际 err=%v", got)
	}
}

// TestWrapPanicAsErr_ErrorType_ContainsDimInfo 验证包装消息含 dim 信息便于排查。
func TestWrapPanicAsErr_ErrorType_ContainsDimInfo(t *testing.T) {
	root := errors.New("panic root")
	dim := types.Dimension{ID: 42, Name: "教学维度"}
	got := wrapPanicAsErr(dim, root)

	msg := got.Error()
	if !contains(msg, "42") {
		t.Errorf("包装消息应含 dim.ID=42，实际: %s", msg)
	}
	if !contains(msg, "教学维度") {
		t.Errorf("包装消息应含 dim.Name=教学维度，实际: %s", msg)
	}
}

// TestWrapPanicAsErr_NonErrorType_FallsBackToV 验证 r 不是 error（典型：string / struct）
// 时退化为 %v 格式化，不 panic、不丢信息。
func TestWrapPanicAsErr_NonErrorType_FallsBackToV(t *testing.T) {
	cases := []struct {
		name string
		r    any
		want string
	}{
		{"string", "boom", "boom"},
		{"int", 42, "42"},
		{"struct", struct{ Msg string }{"kaboom"}, "kaboom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := wrapPanicAsErr(types.Dimension{ID: 1, Name: "d"}, tc.r)
			if got == nil {
				t.Fatal("wrapPanicAsErr 应返回非 nil error")
			}
			if !contains(got.Error(), tc.want) {
				t.Errorf("非 error r 应退化为 %v 格式化，期望含 %q，实际: %s", tc.want, tc.want, got.Error())
			}
		})
	}
}

// TestWrapPanicAsErr_NilRecovery 验证 r == nil（recover 边界）时不 panic、返回明确 error。
func TestWrapPanicAsErr_NilRecovery(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("wrapPanicAsErr(nil) 不应 panic: %v", r)
		}
	}()
	got := wrapPanicAsErr(types.Dimension{ID: 1, Name: "d"}, nil)
	if got == nil {
		t.Fatal("r == nil 时也应返回明确 error（不让裸 nil 走调用链）")
	}
}

// contains 是 strings.Contains 的极简本地版，避免 import 膨胀。
func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}