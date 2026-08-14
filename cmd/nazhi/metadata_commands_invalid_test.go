package main

import (
	"strings"
	"testing"
)

// 集中验证元数据查询命令的非法输入保护：
//   - 任一正整数 flag 传 0 或 -1 都必须立即走参数错误，不创建业务客户端、不发请求；
//   - 错误信息必须写到 stderr；
//   - pendingExitCode 必须设为 3（参数错误）。
//
// 这些测试用 127.0.0.1:1 当 base-url；任何网络流量都说明保护失效。

func TestCircleTypesCmd_RejectsInvalidDimensionID(t *testing.T) {
	cmd := makeMetadataTestCmd(t, "http://127.0.0.1:1")
	_ = cmd.Flags().Set("dimension-id", "-1")
	swapGlobals(t)
	_, stderr, restore := captureStdio(t)
	circleTypesCmd.Run(cmd, nil)
	restore()
	assertParamError(t, stderr.String(), "--dimension-id")
}

func TestCircleTasksCmd_RejectsInvalidTypeID(t *testing.T) {
	cmd := makeMetadataTestCmd(t, "http://127.0.0.1:1")
	_ = cmd.Flags().Set("type-id", "0")
	swapGlobals(t)
	_, stderr, restore := captureStdio(t)
	circleTasksCmd.Run(cmd, nil)
	restore()
	assertParamError(t, stderr.String(), "--type-id")
}

func TestCircleImagesCmd_RejectsInvalidPage(t *testing.T) {
	cmd := makeMetadataTestCmd(t, "http://127.0.0.1:1")
	_ = cmd.Flags().Set("page", "0")
	swapGlobals(t)
	_, stderr, restore := captureStdio(t)
	circleImagesCmd.Run(cmd, nil)
	restore()
	assertParamError(t, stderr.String(), "--page")
}

func TestCircleImagesCmd_RejectsInvalidPageSize(t *testing.T) {
	cmd := makeMetadataTestCmd(t, "http://127.0.0.1:1")
	_ = cmd.Flags().Set("page", "1")
	_ = cmd.Flags().Set("page-size", "-5")
	swapGlobals(t)
	_, stderr, restore := captureStdio(t)
	circleImagesCmd.Run(cmd, nil)
	restore()
	assertParamError(t, stderr.String(), "--page-size")
}

func TestCircleDictCmd_RejectsInvalidCateCode(t *testing.T) {
	cmd := makeMetadataTestCmd(t, "http://127.0.0.1:1")
	_ = cmd.Flags().Set("cate-code", "0")
	swapGlobals(t)
	_, stderr, restore := captureStdio(t)
	circleDictCmd.Run(cmd, nil)
	restore()
	assertParamError(t, stderr.String(), "--cate-code")
}

func TestTaskDimensionsCmd_RejectsMissingToken(t *testing.T) {
	cmd := makeMetadataTestCmd(t, "http://127.0.0.1:1")
	_ = cmd.Flags().Set("token", "")
	swapGlobals(t)
	stdout, stderr, restore := captureStdio(t)
	taskDimensionsCmd.Run(cmd, nil)
	restore()
	if !strings.Contains(stderr.String(), "--token") {
		t.Fatalf("缺 token 应输出参数错误: %s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("参数错误不应写到 stdout: %s", stdout.String())
	}
	if pendingExitCode.Load() != 3 {
		t.Fatalf("缺 token 应设置退出码 3，实际 %d", pendingExitCode.Load())
	}
}

func swapGlobals(t *testing.T) {
	t.Helper()
	originalQuiet, originalVerbose := quiet, verbose
	quiet, verbose = false, false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		quiet, verbose = originalQuiet, originalVerbose
		pendingExitCode.Store(0)
		_ = closeAllClients()
	})
}

func assertParamError(t *testing.T, stderr, flag string) {
	t.Helper()
	if !strings.Contains(stderr, flag) {
		t.Fatalf("非法输入应包含 %s: %s", flag, stderr)
	}
	if pendingExitCode.Load() != 3 {
		t.Fatalf("非法输入应设置退出码 3，实际 %d", pendingExitCode.Load())
	}
}
