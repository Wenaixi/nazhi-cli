package recoverx

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStderr 把 os.Stderr 重定向到临时文件并返回其路径，测试结束恢复。
func captureStderr(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "stderr.txt"))
	if err != nil {
		t.Fatalf("create stderr capture: %v", err)
	}
	orig := os.Stderr
	os.Stderr = f
	t.Cleanup(func() {
		os.Stderr = orig
		f.Close()
	})
	return f.Name()
}

func readCaptured(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stderr capture: %v", err)
	}
	return string(b)
}

// TestRecoverPanic_QuietNoStack 锚定 SD-2 契约：
// --quiet 模式（SetQuiet(true)）下 RecoverPanic 不得把 debug.Stack() 写到
// stderr——此前 pkg/client 的 fetchTasksForDimensionSafe 及 task 拉取 recover 路径
// 无法感知 CLI quiet flag，--quiet 时维度 panic 仍把完整 goroutine stack 打到
// stderr，击穿「--quiet 关闭所有 stderr 输出」契约。
func TestRecoverPanic_QuietNoStack(t *testing.T) {
	path := captureStderr(t)
	SetQuiet(true)
	defer SetQuiet(false)

	r := errors.New("boom")
	got := RecoverPanic(r, nil, "quiet-test")
	if got == nil {
		t.Fatal("RecoverPanic 返回 nil，期望错误")
	}
	out := readCaptured(t, path)
	if strings.Contains(out, "goroutine ") && strings.Contains(out, "recoverx.go") {
		t.Fatalf("quiet 模式不应输出 debug.Stack()，实际输出:\n%s", out)
	}
}

// TestRecoverPanic_NonQuietWritesStack 锚定非静默行为不回归：
// SetQuiet(false) 时 RecoverPanic 仍输出 debug.Stack() 到 stderr。
func TestRecoverPanic_NonQuietWritesStack(t *testing.T) {
	path := captureStderr(t)
	SetQuiet(false)

	r := errors.New("boom")
	if got := RecoverPanic(r, nil, "non-quiet-test"); got == nil {
		t.Fatal("RecoverPanic 返回 nil，期望错误")
	}
	out := readCaptured(t, path)
	if !strings.Contains(out, "goroutine ") || !strings.Contains(out, "recoverx.go") {
		t.Fatalf("非 quiet 模式应输出 debug.Stack()，实际输出:\n%s", out)
	}
}
