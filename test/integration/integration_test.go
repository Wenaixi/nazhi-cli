// Package integration 包含需要真实 SSO/业务服务器环境的集成测试。
//
// 通过 build tag `integration` 启用：
//
//	NAZHI_USERNAME=学号 NAZHI_PASSWORD=密码 go test -tags=integration -v ./test/integration/...
//
// 或通过 .env 文件：
//
//	make test-integration
//
// 若 NAZHI_USERNAME / NAZHI_PASSWORD 未设置，测试自动 t.Skip 跳过。
//
//go:build integration

package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

const (
	defaultSSOBase  = "https://www.nazhisoft.com"
	defaultBizBase  = "http://139.159.205.146:8280"
	defaultUploadBase = "http://doc.nazhisoft.com"
	loginTimeout    = 90 * time.Second // OCR + 网络 + 99 次重试
	apiTimeout      = 30 * time.Second
)

// loadCreds 读取环境变量，未设置时调用 t.Skip 跳过。
func loadCreds(t *testing.T) (string, string, string, string) {
	t.Helper()
	username := os.Getenv("NAZHI_USERNAME")
	password := os.Getenv("NAZHI_PASSWORD")
	if username == "" || password == "" {
		t.Skip("跳过集成测试：未设置 NAZHI_USERNAME / NAZHI_PASSWORD 环境变量")
	}
	ssoBase := os.Getenv("NAZHI_SSO_BASE")
	if ssoBase == "" {
		ssoBase = defaultSSOBase
	}
	bizBase := os.Getenv("NAZHI_BASE_URL")
	if bizBase == "" {
		bizBase = defaultBizBase
	}
	return username, password, ssoBase, bizBase
}

// newClient 构造一个真实环境 Client。
func newClient(t *testing.T, ssoBase, bizBase string) *client.Client {
	t.Helper()
	c := client.New(
		client.WithSSOBase(ssoBase),
		client.WithBaseURL(bizBase),
		client.WithUploadURL(defaultUploadBase),
		client.WithTimeout(apiTimeout),
	)
	return c
}

// TestReal_Login 全自动 OCR 登录（真实 SSO 服务器）。
func TestReal_Login(t *testing.T) {
	username, password, ssoBase, _ := loadCreds(t)

	c := client.New(
		client.WithSSOBase(ssoBase),
		client.WithTimeout(loginTimeout),
	)

	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	defer cancel()

	t.Logf("开始登录 (学号=%s)", maskUsername(username))
	resp, err := c.Login(ctx, types.LoginRequest{
		Username: username,
		Password: password,
	})
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("登录成功但 token 为空")
	}
	t.Logf("✅ 登录成功，token 前缀: %s...", safePrefix(resp.Token, 20))
}

// TestReal_LoginThenActivate 登录 → 激活 Session → 验证用户信息（端到端）。
func TestReal_LoginThenActivate(t *testing.T) {
	username, password, ssoBase, bizBase := loadCreds(t)

	c := newClient(t, ssoBase, bizBase)

	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	defer cancel()

	t.Logf("① 全自动 OCR 登录 (学号=%s)", maskUsername(username))
	loginResp, err := c.Login(ctx, types.LoginRequest{
		Username: username,
		Password: password,
	})
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if loginResp.Token == "" {
		t.Fatal("登录成功但 token 为空")
	}
	token := loginResp.Token
	t.Logf("✅ 登录成功，token 前缀: %s...", safePrefix(token, 20))

	t.Log("② 激活业务 Session")
	if _, err := c.ActivateSession(ctx, token); err != nil {
		t.Fatalf("激活 Session 失败: %v", err)
	}
	t.Log("✅ Session 已激活")

	t.Log("③ 获取用户信息")
	info, err := c.GetMyInfo(ctx, token)
	if err != nil {
		t.Fatalf("获取用户信息失败: %v", err)
	}
	if info == nil {
		t.Fatal("用户信息为空")
	}
	t.Logf("✅ 用户: %s (%s)，学校: %s", info.Name, info.StudentNumber, info.SchoolName)
}

// TestReal_FetchTasks 登录 → 拉取全维度任务。
func TestReal_FetchTasks(t *testing.T) {
	username, password, ssoBase, bizBase := loadCreds(t)
	c := newClient(t, ssoBase, bizBase)

	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	defer cancel()

	loginResp, err := c.Login(ctx, types.LoginRequest{
		Username: username,
		Password: password,
	})
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	token := loginResp.Token

	if _, err := c.ActivateSession(ctx, token); err != nil {
		t.Fatalf("激活 Session 失败: %v", err)
	}

	tasks, err := c.FetchTasks(ctx, token)
	if err != nil {
		t.Fatalf("获取任务列表失败: %v", err)
	}
	t.Logf("✅ 共 %d 个任务", len(tasks))
	for i, task := range tasks {
		if i >= 3 {
			break
		}
		t.Logf("  - [%d] %s (维度 %d, 状态 %s)", task.ID, task.Name, task.DimensionID, task.Status)
	}
}

// TestReal_SelfEvaluation 登录 → 查询自我评价（不提交，避免污染数据）。
func TestReal_SelfEvaluation(t *testing.T) {
	username, password, ssoBase, bizBase := loadCreds(t)
	c := newClient(t, ssoBase, bizBase)

	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	defer cancel()

	loginResp, err := c.Login(ctx, types.LoginRequest{
		Username: username,
		Password: password,
	})
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	token := loginResp.Token

	if _, err := c.ActivateSession(ctx, token); err != nil {
		t.Fatalf("激活 Session 失败: %v", err)
	}

	status, err := c.QuerySelfEvaluation(ctx, token)
	if err != nil {
		t.Fatalf("查询自我评价失败: %v", err)
	}
	if status == nil {
		t.Fatal("自我评价为空")
	}
	t.Logf("✅ 学生评语: %s", truncate(status.StudentComment, 50))
	t.Logf("  教师评语: %s", truncate(status.TeacherComment, 50))
}

// ─── 工具函数 ───

// maskUsername 部分遮罩学号用于日志。
func maskUsername(u string) string {
	if len(u) <= 4 {
		return strings.Repeat("*", len(u))
	}
	return u[:2] + strings.Repeat("*", len(u)-4) + u[len(u)-2:]
}

// safePrefix 安全地取字符串前缀（不 panic）。
func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// truncate 截断字符串到指定长度。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
