package main

import (
	"fmt"
	"io/fs"
	"os"
	"testing"
)

// TestPrintError_PathError_ExitCode3 锁定 FILE-1 契约：
// 本地文件系统错误（文件不存在/路径不可写等调用方可控输入问题）
// 应归参数档 400/exit3——此前经 mapSentinelToHTTPCode default 走 500/exit2，
// 脚本对永久性本地输入错误无限重试（与 ErrFileTooLarge 已修复的同类约定冲突）。
func TestPrintError_PathError_ExitCode3(t *testing.T) {
	orig := pendingExitCode.Load()
	defer pendingExitCode.Store(orig)
	quiet = false

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = origStderr; _ = r.Close() }()

	_, statErr := os.Stat("nonexistent-file-xyz.tmp")
	if statErr == nil {
		t.Fatal("不存在的文件不应 Stat 成功")
	}
	var pathErr *fs.PathError
	if !errorsAs(statErr, &pathErr) {
		t.Fatalf("Stat 错误应为 *fs.PathError，实际 %T", statErr)
	}
	wrapped := fmt.Errorf("读取附件失败: %w", statErr)
	printError(wrapped)

	if got := pendingExitCode.Load(); got != 3 {
		t.Errorf("PathError 应走参数档(退出码 3)，实际 %d", got)
	}
}

func errorsAs(err error, target any) bool {
	switch t := target.(type) {
	case **fs.PathError:
		v, ok := err.(*fs.PathError)
		if ok {
			*t = v
		}
		return ok
	}
	return false
}
