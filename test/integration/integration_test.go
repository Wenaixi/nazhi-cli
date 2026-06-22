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
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

const (
	defaultSSOBase    = "https://www.nazhisoft.com"
	defaultBizBase    = "http://139.159.205.146:8280"
	defaultUploadBase = "http://doc.nazhisoft.com"
	loginTimeout      = 90 * time.Second // OCR + 网络 + 99 次重试
	apiTimeout        = 30 * time.Second
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

// sharedLogin 登录一次，所有测试复用 token。失败时 t.Fatal。
func sharedLogin(t *testing.T, c *client.Client, username, password string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	defer cancel()

	t.Logf("① 全自动 OCR 登录 (学号=%s)", maskUsername(username))
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
	return resp.Token
}

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

// ────────────────────────────────────────────────────────────
// 测试用例
// ────────────────────────────────────────────────────────────

// TestReal_FullChain 端到端跑完所有能测的 SDK 方法。
// 不依赖期末数据，专注验证 SDK 与真实服务器的对齐度。
func TestReal_FullChain(t *testing.T) {
	username, password, ssoBase, bizBase := loadCreds(t)
	c := newClient(t, ssoBase, bizBase)

	// 1. 登录拿 token
	token := sharedLogin(t, c, username, password)
	if token == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()

	// 2. InitSession（已由 Login 内部调用，这里显式测一下）
	t.Log("② InitSession (SSO Session)")
	if err := c.InitSession(ctx); err != nil {
		t.Errorf("InitSession: %v", err)
	}

	// 3. GetSchoolID
	t.Log("③ GetSchoolID")
	schoolID, schoolName, err := c.GetSchoolID(ctx, username)
	if err != nil {
		t.Errorf("GetSchoolID: %v", err)
	} else {
		t.Logf("   ✅ 学校: %s (ID=%s)", schoolName, schoolID)
	}

	// 4. ActivateSession（4 步 HAR 对齐）
	t.Log("④ ActivateSession (4 步 HAR 对齐)")
	_, err = c.ActivateSession(ctx, token)
	if err != nil {
		t.Errorf("ActivateSession: %v", err)
	}

	// 5. GetMyInfo / whoami
	t.Log("⑤ GetMyInfo (whoami)")
	info, err := c.GetMyInfo(ctx, token)
	if err != nil {
		t.Errorf("GetMyInfo: %v", err)
	} else if info != nil {
		t.Logf("   ✅ %s / %s / %s / 座号 %d", info.Name, info.SchoolName, info.ClassName, info.Seat)
	}

	// 6. GetDimensions（不需要任务）
	t.Log("⑥ GetDimensions (维度列表)")
	dims, err := c.GetDimensions(ctx, token)
	if err != nil {
		t.Errorf("GetDimensions: %v", err)
	} else {
		t.Logf("   ✅ %d 个维度", len(dims))
		for i, d := range dims {
			if i >= 3 {
				break
			}
			t.Logf("     - 维度 %d: %s", d.ID, d.Name)
		}
	}

	// 7. FetchTasks（期末未到，预期空列表）
	t.Log("⑦ FetchTasks (任务列表，期末未到预期空)")
	tasks, err := c.FetchTasks(ctx, token)
	if err != nil {
		t.Errorf("FetchTasks: %v", err)
	} else {
		t.Logf("   ✅ 任务数: %d（期末未到属正常）", len(tasks))
	}

	// 8. QuerySelfEvaluation（自我评价 + 教师评语）
	t.Log("⑧ QuerySelfEvaluation (自我评价 + 教师评语)")
	status, err := c.QuerySelfEvaluation(ctx, token)
	if err != nil {
		t.Errorf("QuerySelfEvaluation: %v", err)
	} else if status != nil {
		t.Logf("   ✅ 教师评语: %s", truncate(status.TeacherComment, 60))
	}

	// 9. QuerySelfGradEvaluation
	t.Log("⑨ QuerySelfGradEvaluation (学期评价)")
	grad, err := c.QuerySelfGradEvaluation(ctx, token)
	if err != nil {
		t.Logf("   ⚠️  QuerySelfGradEvaluation: %v", err)
	} else if grad != nil {
		t.Logf("   ✅ 学期评价: %v", truncate(fmtMap(grad), 80))
	} else {
		t.Log("   ℹ️  学期评价为空（正常）")
	}

	// 10. UploadFile（图片上传，5MB 压缩 + JPG 转换）
	t.Log("⑩ UploadFile (图片上传)")
	tmpImg := createTestImage(t)
	defer os.Remove(tmpImg)

	id, err := c.UploadFile(ctx, tmpImg)
	if err != nil {
		t.Errorf("UploadFile: %v", err)
	} else {
		t.Logf("   ✅ 上传成功，图片 ID: %d", id)
	}
}

// ────────────────────────────────────────────────────────────
// 辅助：创建测试图片（PNG 格式，让 SDK 走"任意格式→JPG"转换路径）
// ────────────────────────────────────────────────────────────

func createTestImage(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.png")

	// 创建 800x600 PNG（透明背景，让 SDK 走 flattenOnWhite）
	img := image.NewRGBA(image.Rect(0, 0, 800, 600))
	// 填一些渐变颜色 + 透明区域
	for y := 0; y < 600; y++ {
		for x := 0; x < 800; x++ {
			alpha := uint8(255)
			// 右下半透明
			if x > 400 && y > 300 {
				alpha = 128
			}
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(x % 256),
				G: uint8(y % 256),
				B: uint8((x + y) % 256),
				A: alpha,
			})
		}
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("创建测试图片失败: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("PNG 编码失败: %v", err)
	}
	return path
}

// fmtMap 把 map 简单转字符串（用于日志）。
func fmtMap(m *map[string]any) string {
	if m == nil {
		return "<nil>"
	}
	parts := make([]string, 0, len(*m))
	for k, v := range *m {
		parts = append(parts, k+"="+truncate(anyToString(v), 20))
	}
	return strings.Join(parts, ", ")
}

func anyToString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return "<非字符串>"
}
