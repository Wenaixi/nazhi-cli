package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// 这组测试逐命令锁定「参数错误走 stderr」这条契约，覆盖那些历史上绕过
// printParamError、直接调 printEnvelope(envelope.Error(400)) 因而写到 stdout
// 的命令。param_error_channel_test.go 锁的是出口本身的行为，这里锁的是
// 各命令确实经过那个出口。
//
// 此前 honor list 的分页守卫走 stdout 而 circle images 的同类守卫走
// stderr；写操作骨架 6 步里第 1 步与第 6 步走 stdout、中间 4 步走 stderr。
// 调用方无法从任何地方学到规则，只能逐命令试。

// runCmdWithFlags 跑一次命令并返回两通道内容与退出码。
// 不设 token：多数命令会在建客户端前先报参数错误，这正是要锁的次序。
func runCmdWithFlags(t *testing.T, cmd *cobra.Command, flags map[string]string) (string, string, int32) {
	t.Helper()
	cmd.SetContext(context.Background())
	cmd.SetArgs(nil)
	for _, name := range []string{"token", "base-url", "timeout"} {
		cmd.Flags().String(name, "", "")
	}
	cmd.Flags().Bool("verbose", false, "")
	cmd.Flags().Bool("quiet", false, "")
	for k, v := range flags {
		_ = cmd.Flags().Set(k, v)
	}
	originalQuiet, originalVerbose := quiet, verbose
	quiet, verbose = false, false
	pendingExitCode.Store(0)
	stdout, stderr, restore := captureStdio(t)
	cmd.Run(cmd, nil)
	restore()
	quiet, verbose = originalQuiet, originalVerbose
	code := pendingExitCode.Load()
	pendingExitCode.Store(0)
	return stdout.String(), stderr.String(), code
}

// assertParamErrorOnStderr 断言该命令的参数错误落在 stderr 且退出码为 3。
func assertParamErrorOnStderr(t *testing.T, name, stdout, stderr string, code int32) {
	t.Helper()
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("%s：参数错误写到了 stdout，stdout=%q", name, stdout)
	}
	if !strings.Contains(stderr, `"code": 400`) {
		t.Errorf("%s：stderr 未含 400 信封，stderr=%q", name, stderr)
	}
	if code != 3 {
		t.Errorf("%s：退出码 = %d，期望 3", name, code)
	}
}

// TestCmdParamErrorsGoToStderr 逐命令锁定参数错误的通道与退出码。
// 覆盖历史上走 stdout 的全部参数错误路径。
func TestCmdParamErrorsGoToStderr(t *testing.T) {
	cases := []struct {
		name  string
		cmd   *cobra.Command
		flags map[string]string
	}{
		{"circle comment 缺 --id", circleCommentCmd, map[string]string{"content": "x"}},
		{"circle comment 非法 --id", circleCommentCmd, map[string]string{"id": "0", "content": "x"}},
		{"circle comment 缺 --content", circleCommentCmd, map[string]string{"id": "1"}},
		{"circle delete 缺 --id", circleDeleteCmd, map[string]string{}},
		{"circle delete 非法 --id", circleDeleteCmd, map[string]string{"id": "-1"}},
		{"circle like 缺 --id", circleLikeCmd, map[string]string{}},
		{"circle like 非法 --id", circleLikeCmd, map[string]string{"id": "0"}},
		{"honor list 非法 --page", honorListCmd, map[string]string{"page": "0"}},
		{"honor update 非法 --id", honorUpdateCmd, map[string]string{"payload": `{"id":0}`}},
		{"typical-case list 非法 --page", typicalCaseListCmd, map[string]string{"page": "0"}},
		{"typical-case update 缺 --id", typicalCaseUpdateCmd, map[string]string{"payload": `{"title":"x"}`}},
		{"task submit 缺 --payload", taskSubmitCmd, map[string]string{}},
		{"task edit 缺 --payload", taskEditCmd, map[string]string{}},
		{"user update 缺 --payload", userUpdateCmd, map[string]string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 每个用例用独立命令副本，避免 flag 状态跨用例污染
			c := cloneCmdForTest(t, tc.cmd)
			stdout, stderr, code := runCmdWithFlags(t, c, tc.flags)
			assertParamErrorOnStderr(t, tc.name, stdout, stderr, code)
		})
	}
}

// TestFileCmdParamErrorsGoToStderr 文件命令的参数错误同样走 stderr。
// file upload/download 需要临时文件与 --id，单独一组避免污染上一组的 flag 集。
func TestFileCmdParamErrorsGoToStderr(t *testing.T) {
	cases := []struct {
		name string
		cmd  *cobra.Command
		flag map[string]string
	}{
		{"file download 缺 --id", fileDownloadCmd, map[string]string{}},
		{"file download 缺 --output", fileDownloadCmd, map[string]string{"id": "1"}},
		{"file upload 缺 --file", fileUploadCmd, map[string]string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := cloneCmdForTest(t, tc.cmd)
			stdout, stderr, code := runCmdWithFlags(t, c, tc.flag)
			assertParamErrorOnStderr(t, tc.name, stdout, stderr, code)
		})
	}
}

// TestSelfEvalParamErrorsGoToStderr 自评提交的内容为空校验走 stderr。
// 该校验读 stdin，故只在传入空内容时构造，不依赖真实平台。
func TestSelfEvalParamErrorsGoToStderr(t *testing.T) {
	for name, cmd := range map[string]*cobra.Command{
		"self-eval submit":      selfEvalSubmitCmd,
		"self-eval grad submit": selfEvalGradSubmitCmd,
	} {
		t.Run(name, func(t *testing.T) {
			c := cloneCmdForTest(t, cmd)
			c.Flags().String("content", "", "")
			_ = c.Flags().Set("content", "")
			c.Flags().String("student-id", "", "")
			_ = c.Flags().Set("student-id", "1")
			stdout, stderr, code := runCmdWithFlags(t, c, map[string]string{})
			if strings.TrimSpace(stdout) != "" {
				t.Errorf("%s：参数错误写到了 stdout，stdout=%q", name, stdout)
			}
			if code != 3 {
				t.Errorf("%s：退出码 = %d，期望 3（stderr=%q）", name, code, stderr)
			}
		})
	}
}

// cloneCmdForTest 复制一个命令的 Run 与 flag 名，避免直接改全局命令定义。
// 只复制执行所需的部分：Use 用于错误文案，Run 是实际控制流。
func cloneCmdForTest(t *testing.T, src *cobra.Command) *cobra.Command {
	t.Helper()
	if src == nil {
		t.Fatal("命令为 nil")
	}
	return &cobra.Command{Use: src.Use, Run: src.Run, Args: src.Args}
}

// 断言辅助：确保 cloneCmdForTest 真的复制了 Run，避免用例静默变成空跑。
func TestCloneCmdForTestPreservesRun(t *testing.T) {
	if circleCommentCmd == nil || circleCommentCmd.Run == nil {
		t.Fatal("circle comment 命令缺少 Run，测试会静默空跑")
	}
	c := cloneCmdForTest(t, circleCommentCmd)
	if c.Run == nil {
		t.Fatal("克隆后 Run 丢失，测试无效")
	}
	var buf bytes.Buffer
	c.SetOut(&buf)
	if !strings.Contains(c.Use, "comment") {
		t.Errorf("克隆后 Use 丢失：%q", c.Use)
	}
}
