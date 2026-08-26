package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// makeUserUpdateTestCmd 创建 user update 命令测试用 cobra.Command + mock server。
// captureBody 可选，用于捕获 POST updateMyInfo 的请求体。
func makeUserUpdateTestCmd(t *testing.T, payload string, captureBody *string) *cobra.Command {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
		case "/api/studentInfo/updateMyInfo":
			if r.Method != http.MethodPost {
				t.Errorf("updateMyInfo 期望 POST, 得到 %s", r.Method)
			}
			bodyBytes, _ := io.ReadAll(r.Body)
			if captureBody != nil {
				*captureBody = string(bodyBytes)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":500,"msg":"未知路径"}`))
		}
	}))
	t.Cleanup(srv.Close)

	cmd := &cobra.Command{Use: "user-update"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("payload", "", "")
	if payload != "" {
		_ = cmd.Flags().Set("payload", payload)
	}
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", srv.URL)
	cmd.Flags().Int("timeout", 5, "")
	return cmd
}

// TestUserUpdateCmd_FriendlyKeysRemap 验证 CLI 友好键（genderName 等）经
// UpdateMyInfoStructured 转换为 API 数字代码，而不是裸 map 透传。
//
// 历史 bug：cmd 层 Unmarshal 到 map[string]any 后直接调 UpdateMyInfo，
// 友好字段 genderName="男" 原样发给服务端，服务端忽略/报错。
func TestUserUpdateCmd_FriendlyKeysRemap(t *testing.T) {
	var body string
	cmd := makeUserUpdateTestCmd(t, `{"telephone":"13800138000","genderName":"男","youthLeague":"是"}`, &body)

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	userUpdateCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if got := pendingExitCode.Load(); got != 0 {
		t.Errorf("正常路径不应触发 pendingExitCode，实际 %d；stderr=%q", got, stderr)
	}
	if strings.Contains(stderr, `"status": "error"`) {
		t.Errorf("stderr 不应包含 error envelope，实际: %q", stderr)
	}
	if !strings.Contains(stdout, `"code": 204`) {
		t.Errorf("stdout 应是 envelope.Empty (code=204)，实际: %q", stdout)
	}

	// 必须 remap 为 gender=1、youthLeagueFlag=1；不得原样透传 genderName
	if strings.Contains(body, "genderName") {
		t.Errorf("请求体不应含 genderName（应 remap 为 gender），实际: %q", body)
	}
	if !strings.Contains(body, `"gender":1`) && !strings.Contains(body, `"gender": 1`) {
		t.Errorf("请求体应含 gender=1，实际: %q", body)
	}
	if strings.Contains(body, "youthLeague") && !strings.Contains(body, "youthLeagueFlag") {
		t.Errorf("请求体应含 youthLeagueFlag 而非 youthLeague，实际: %q", body)
	}
	if !strings.Contains(body, `"youthLeagueFlag":1`) && !strings.Contains(body, `"youthLeagueFlag": 1`) {
		t.Errorf("请求体应含 youthLeagueFlag=1，实际: %q", body)
	}
	if !strings.Contains(body, "13800138000") {
		t.Errorf("请求体应透传 telephone，实际: %q", body)
	}
}

// TestUserUpdateCmd_PreservesFrontendBirthday 验证 CLI 可直接接收前端 birthday 字段。
func TestUserUpdateCmd_PreservesFrontendBirthday(t *testing.T) {
	var body string
	cmd := makeUserUpdateTestCmd(t, `{"birthday":"2009-12-11T00:00:00.000Z"}`, &body)

	quiet = false
	pendingExitCode.Store(0)
	stdoutBuf, stderrBuf, restore := captureStdio(t)
	userUpdateCmd.Run(cmd, nil)
	restore()

	if pendingExitCode.Load() != 0 {
		t.Fatalf("生日更新成功路径不应设置退出码，实际 %d；stderr=%s", pendingExitCode.Load(), stderrBuf.String())
	}
	if !strings.Contains(stdoutBuf.String(), `"code": 204`) {
		t.Fatalf("生日更新应输出成功 envelope，实际: %s", stdoutBuf.String())
	}
	if !strings.Contains(body, `"birthday":"2009-12-11T00:00:00.000Z"`) && !strings.Contains(body, `"birthday": "2009-12-11T00:00:00.000Z"`) {
		t.Fatalf("前端 birthday 未透传，实际请求体: %s", body)
	}
	if strings.Contains(body, "birthdayStr") {
		t.Errorf("请求体不应出现 birthdayStr，实际请求体: %s", body)
	}
	if strings.Contains(stderrBuf.String(), `"status": "error"`) {
		t.Errorf("成功更新不应输出错误 envelope，实际: %s", stderrBuf.String())
	}
}

// TestUserUpdateCmd_InvalidGender 验证不支持的友好值返回参数错误而非裸透传。
func TestUserUpdateCmd_InvalidGender(t *testing.T) {
	cmd := makeUserUpdateTestCmd(t, `{"genderName":"未知"}`, nil)

	quiet = false
	pendingExitCode.Store(0)

	_, stderrBuf, restore := captureStdio(t)
	userUpdateCmd.Run(cmd, nil)
	restore()
	stderr := stderrBuf.String()

	// 参数错误：printError 走 5xx 或业务错误；至少不能成功（exit 0 + Empty）
	// UpdateMyInfoStructured 返回 ErrInvalidPayload
	if pendingExitCode.Load() == 0 && !strings.Contains(stderr, "error") {
		// 若 exit 仍为 0，至少 stderr 应有错误
		if !strings.Contains(stderr, "不支持") && !strings.Contains(stderr, "Invalid") && !strings.Contains(stderr, "error") {
			t.Fatalf("非法 genderName 应失败，stderr=%q exit=%d", stderr, pendingExitCode.Load())
		}
	}
	if pendingExitCode.Load() == 0 {
		t.Errorf("非法 genderName 应设置 pendingExitCode≠0，实际 0；stderr=%q", stderr)
	}
}

// TestUserUpdateCmd_UnknownTopLevelKey_Rejects 锁定 19 轮审计 user-info P2-3：
// CLI user update --payload 的未知顶层键被 json.Unmarshal 静默丢弃，
// 全零命中 USER-1 no-op 分支（uc/user_update.go:99）→ 输出 204 成功但服务端零修改。
func TestUserUpdateCmd_UnknownTopLevelKey_Rejects(t *testing.T) {
	requestHit := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
		case "/api/studentInfo/updateMyInfo":
			requestHit = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		}
	}))
	t.Cleanup(srv.Close)

	cmd := &cobra.Command{Use: "user-update"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("payload", "", "")
	_ = cmd.Flags().Set("payload", `{"telephoneX":"13800138000"}`)
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", srv.URL)
	cmd.Flags().Int("timeout", 5, "")

	quiet = false
	pendingExitCode.Store(0)

	_, stderrBuf, restore := captureStdio(t)
	userUpdateCmd.Run(cmd, nil)
	restore()
	stderr := stderrBuf.String()

	if requestHit {
		t.Fatal("未知键 payload 不应发出业务请求")
	}
	if got := pendingExitCode.Load(); got != 3 {
		t.Errorf("未知键应走参数错误退出码 3，实际 %d", got)
	}
	if !strings.Contains(stderr, `"status": "error"`) || !strings.Contains(stderr, `"code": 400`) {
		t.Errorf("stderr 应输出 400 参数错误 envelope，实际: %q", stderr)
	}
	if !strings.Contains(stderr, "未知键") {
		t.Errorf("stderr 应含未知键提示，实际: %q", stderr)
	}
}
