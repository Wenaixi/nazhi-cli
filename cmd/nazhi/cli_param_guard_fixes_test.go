package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// ─── login username/password 空白串必须拒绝（TrimSpace 校验） ───

// TestLoginCmd_BlankCredentials_Rejected 锁定
// username/password 纯空白串（空格/Tab 等）此前通过 == "" 校验被原样上送 SSO，
// 现在 trim 后为空必须 400/exit3 拒绝，不发任何网络请求。
func TestLoginCmd_BlankCredentials_Rejected(t *testing.T) {
	cases := []struct {
		name     string
		username string
		password string
	}{
		{"username 纯空白", "   ", "secret"},
		{"password 纯空白", "student001", " \t "},
		{"两者都空白", "  ", "  "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "login"}
			cmd.SetContext(context.Background())
			cmd.Flags().String("username", "", "")
			_ = cmd.Flags().Set("username", tc.username)
			cmd.Flags().String("password", "", "")
			_ = cmd.Flags().Set("password", tc.password)
			cmd.Flags().String("sso-base", "", "")
			cmd.Flags().Int("timeout", 15, "")

			quiet = false
			pendingExitCode.Store(0)
			stdoutBuf, _, restore := captureStdio(t)
			loginCmd.Run(cmd, nil)
			restore()
			stdout := stdoutBuf.String()

			if got := pendingExitCode.Load(); got != 3 {
				t.Errorf("空白凭据应走参数错误退出码 3，实际 %d; stdout=%q", got, stdout)
			}
			if !strings.Contains(stdout, "为必填") {
				t.Errorf("stdout 应含必填提示，实际: %q", stdout)
			}
		})
	}
}

// ─── 边界：正常凭据不被误拒（flag 值原样透传） ───

func TestLoginCmd_NormalCredentials_NotTrimmed(t *testing.T) {
	cmd := &cobra.Command{Use: "login"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("username", "", "")
	_ = cmd.Flags().Set("username", " student001 ") // 前后空格保留业务语义
	cmd.Flags().String("password", "", "")
	_ = cmd.Flags().Set("password", "pass")
	cmd.Flags().String("sso-base", "", "")
	_ = cmd.Flags().Set("sso-base", "http://127.0.0.1:1")
	cmd.Flags().Int("timeout", 15, "")

	quiet = false
	pendingExitCode.Store(0)
	_, _, restore := captureStdio(t)
	loginCmd.Run(cmd, nil)
	restore()

	// 不应在空白校验处被拒（went to buildClient → 缺 base 配置？login 走 sso 类型
	// 不要求 token/base-url；buildClient 只报超时/URL 问题。此处只断言 400 必填
	// 提示不出现即可——真正校验点在下一命令真连 SSO）。
	if got := pendingExitCode.Load(); got == 3 {
		t.Errorf("带现实中前后空格的凭据不应在第 0 道空白校验被拒，实际退出码 %d", got)
	}
}

// ─── circle images --page-size 上钳 500 ───

// TestCircleImages_PageSizeCappedAt500 锁定
// circle images --page-size=501 走参数错误拒绝（400/exit3），不发业务请求；
// 500 放行（对齐 honor list / typical-case list 的 maxPageSize 纪律）。
func TestCircleImages_PageSizeCappedAt500(t *testing.T) {
	t.Run("page-size 501 拒绝", func(t *testing.T) {
		cmd := &cobra.Command{Use: "images"}
		cmd.SetContext(context.Background())
		cmd.Flags().String("token", "", "")
		_ = cmd.Flags().Set("token", "test-token")
		cmd.Flags().String("base-url", "", "")
		_ = cmd.Flags().Set("base-url", "http://127.0.0.1:1")
		cmd.Flags().Int("timeout", 5, "")
		cmd.Flags().Int("page", 1, "")
		cmd.Flags().Int("page-size", 501, "")

		quiet = false
		pendingExitCode.Store(0)
		_, _, restore := captureStdio(t)
		circleImagesCmd.Run(cmd, nil)
		restore()

		if got := pendingExitCode.Load(); got != 3 {
			t.Errorf("page-size=501 应走参数错误退出码 3，实际 %d", got)
		}
	})

	t.Run("page-size 500 放行并发出业务请求", func(t *testing.T) {
		hit := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/studentCircleNew/getCircleImg" {
				hit = true
			}
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/", "/api/studentInfo/getMenu":
				_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
			case "/api/studentInfo/getMyInfo":
				_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"示例学生","studentNumber":"TEST2025001"}}`))
			case "/api/studentCircleNew/getCircleImg":
				_, _ = w.Write([]byte(`{"code":1,"msg":"成功","dataList":[],"pageBean":{"pageNo":1,"pageSize":500,"totalNum":0,"totalPage":0}}`))
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		cmd := &cobra.Command{Use: "images"}
		cmd.SetContext(context.Background())
		cmd.Flags().String("token", "", "")
		_ = cmd.Flags().Set("token", "test-token")
		cmd.Flags().String("base-url", "", "")
		_ = cmd.Flags().Set("base-url", srv.URL)
		cmd.Flags().Int("timeout", 5, "")
		cmd.Flags().Int("page", 1, "")
		cmd.Flags().Int("page-size", 500, "")

		quiet = false
		pendingExitCode.Store(0)
		_, stderr, restore := captureStdio(t)
		circleImagesCmd.Run(cmd, nil)
		restore()

		if !hit {
			t.Fatalf("page-size=500 应放行并真正发出业务请求; stderr=%s", stderr.String())
		}
		if got := pendingExitCode.Load(); got != 0 {
			t.Errorf("page-size=500 不应设置退出码，实际 %d", got)
		}
	})
}

// ─── rejectLoneOffset 违规参数可能是 --limit 负值，文案必须点名 ───

// TestRejectLoneOffset_NegativeLimit_MessageCoversLimit 锁定
// --limit -1 被拒时文案不仅提 --offset（此前用户只看到 "--offset 需为非负数"
// 却不知自己违规的是 --limit）。
func TestRejectLoneOffset_NegativeLimit_MessageCoversLimit(t *testing.T) {
	cmd := &cobra.Command{Use: "offset-test"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", "http://127.0.0.1:1")
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().Int("offset", 0, "")
	cmd.Flags().Int("limit", 0, "")
	_ = cmd.Flags().Set("limit", "-1")

	quiet = false
	pendingExitCode.Store(0)
	stdoutBuf, _, restore := captureStdio(t)
	rejected := rejectLoneOffset(cmd)
	restore()
	stdout := stdoutBuf.String()

	if !rejected {
		t.Fatal("--limit -1 应被 rejectLoneOffset 拒绝")
	}
	if got := pendingExitCode.Load(); got != 3 {
		t.Errorf("--limit -1 应走参数错误退出码 3，实际 %d", got)
	}
	if !strings.Contains(stdout, "--limit") {
		t.Errorf("文案应点名违规参数 --limit，实际: %q", stdout)
	}
}

// ─── honor add / typical-case submit 未知键拒绝 ───

// makeAddPayloadTestCmd 创建带通用业务参数与 payload 的 add/submit 测试命令。
func makeAddPayloadTestCmd(t *testing.T, baseURL, payload string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "add-payload-test"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", baseURL)
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().String("payload", "", "")
	_ = cmd.Flags().Set("payload", payload)
	return cmd
}

// TestHonorAdd_UnknownTopLevelKey_Rejects 锁定
// honor add payload 未知键（如 evalautionAgency 拼错）此前被 struct 反序列化
// 静默丢弃并 204 报成功但零申报，现在 400/exit3 拒绝且不发业务请求。
// 注意：typeid 小写是 大小写不敏感语义下的合法键（与 typeId 等价），
// 测试用真正拼错的键 evalautionAgency 保证命中未知键判定。
func TestHonorAdd_UnknownTopLevelKey_Rejects(t *testing.T) {
	requestHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "addHonor") {
			requestHit = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
	}))
	defer server.Close()

	cmd := makeAddPayloadTestCmd(t, server.URL, `{"typeid":1147,"level":5,"evalautionAgency":"示例中学"}`)
	quiet = false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		quiet, verbose = false, false
		pendingExitCode.Store(0)
		_ = closeAllClients()
	})

	stdout, stderr, restore := captureStdio(t)
	honorAddCmd.Run(cmd, nil)
	restore()

	if requestHit {
		t.Fatal("honor add 含未知键时不应发出业务请求")
	}
	if got := pendingExitCode.Load(); got != 3 {
		t.Errorf("未知键应走参数错误退出码 3，实际 %d", got)
	}
	if !strings.Contains(stdout.String(), "未知键") && !strings.Contains(stderr.String(), "未知键") {
		t.Errorf("应含未知键提示，实际 stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
}

// TestTypicalCaseSubmit_UnknownTopLevelKey_Rejects 锁定
// typical-case submit payload 未知键（如 titlee 拼错）同款拒绝。
func TestTypicalCaseSubmit_UnknownTopLevelKey_Rejects(t *testing.T) {
	requestHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "addTypicalCase") {
			requestHit = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
	}))
	defer server.Close()

	cmd := makeAddPayloadTestCmd(t, server.URL, `{"title":"标题","titlee":"拼错的标题"}`)
	quiet = false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		quiet, verbose = false, false
		pendingExitCode.Store(0)
		_ = closeAllClients()
	})

	stdout, stderr, restore := captureStdio(t)
	typicalCaseSubmitCmd.Run(cmd, nil)
	restore()

	if requestHit {
		t.Fatal("typical-case submit 含未知键时不应发出业务请求")
	}
	if got := pendingExitCode.Load(); got != 3 {
		t.Errorf("未知键应走参数错误退出码 3，实际 %d", got)
	}
	if !strings.Contains(stdout.String(), "未知键") && !strings.Contains(stderr.String(), "未知键") {
		t.Errorf("应含未知键提示，实际 stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
}

// ─── 未知键判定大小写不敏感（对齐 task 族 EqualFold 语义） ───

// TestUnknownUpdatePayloadKeys_CaseInsensitive 锁定
// 允许键统一小写存储，用户键 ToLower 后比较——CERTIMGATTACHMENTID/Telephone
// 等大小写变体不误拒，真正未知键仍拒绝。
func TestUnknownUpdatePayloadKeys_CaseInsensitive(t *testing.T) {
	allowed := map[string]struct{}{
		"title": {}, "teachername": {}, "certimgattachmentid": {},
	}

	caseVariant := `{"Title":"大小写","teacherName":"王","CERTIMGATTACHMENTID":123}`
	if unknown := unknownUpdatePayloadKeys([]byte(caseVariant), allowed); len(unknown) != 0 {
		t.Errorf("大小写变体应视为已允许键，误报未知: %v", unknown)
	}

	trulyUnknown := `{"title":"标题","titel":"拼错"}`
	unknown := unknownUpdatePayloadKeys([]byte(trulyUnknown), allowed)
	if len(unknown) != 1 || unknown[0] != "titel" {
		t.Errorf("真正未知键 titel 应被拒绝，实际 %v", unknown)
	}
}
