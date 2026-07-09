package main

import (
	"context"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// makeTempJPEG 在 t.TempDir() 中创建一个小型合法 JPEG 测试文件，返回文件路径。
func makeTempJPEG(t *testing.T) string {
	t.Helper()
	path := t.TempDir() + "/test.jpg"
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("创建临时 JPEG 文件失败: %v", err)
	}
	defer f.Close()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 50}); err != nil {
		t.Fatalf("编码 JPEG 失败: %v", err)
	}
	return path
}

// makeFileUploadTestCmd 创建 file upload 命令的测试用 cobra.Command + mock upload server。
// filePath 是 --file flag 的值（空字符串时不设 flag）。
func makeFileUploadTestCmd(t *testing.T, filePath string) *cobra.Command {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// file upload 路径 /common/upload/uploadImage
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"id":42}}`))
	}))
	t.Cleanup(srv.Close)

	cmd := &cobra.Command{Use: "file-upload"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("file", "", "")
	if filePath != "" {
		_ = cmd.Flags().Set("file", filePath)
	}
	cmd.Flags().String("upload-url", "", "")
	_ = cmd.Flags().Set("upload-url", srv.URL)
	cmd.Flags().Int("timeout", 5, "")
	return cmd
}

// TestFileUploadCmd_MissingFile_PrintsError 验证 --file 缺省时输出 error envelope。
func TestFileUploadCmd_MissingFile_PrintsError(t *testing.T) {
	cmd := makeFileUploadTestCmd(t, "") // 不设 file flag

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	fileUploadCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	// 缺 file → envelope.Error(400, ...) → exit code 3
	if got := pendingExitCode.Load(); got != 3 {
		t.Errorf("缺 file 应触发 pendingExitCode=3（envelope.Error(400)），实际 %d", got)
	}
	// envelope.Error 走 printEnvelope（stdout），不再是 stderr JSON
	if strings.Contains(stderr, `"status": "error"`) {
		t.Errorf("envelope.Error 走 stdout，不应在 stderr，实际: %q", stderr)
	}
	if !strings.Contains(stdout, `"status": "error"`) {
		t.Errorf("stdout 应包含 envelope.Error，实际: %q", stdout)
	}
	if !strings.Contains(stdout, "file") {
		t.Errorf("stdout 应包含 file 提示，实际: %q", stdout)
	}
}

// TestFileUploadCmd_HappyPath 验证完整上传流程：JPEG 文件上传成功，输出 id + path。
func TestFileUploadCmd_HappyPath(t *testing.T) {
	jpegPath := makeTempJPEG(t)
	cmd := makeFileUploadTestCmd(t, jpegPath)

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	fileUploadCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if got := pendingExitCode.Load(); got != 0 {
		t.Errorf("正常上传不应触发 pendingExitCode=1，实际 %d", got)
	}
	if strings.Contains(stderr, `"status": "error"`) {
		t.Errorf("stderr 不应包含 error envelope，实际: %q", stderr)
	}
	if !strings.Contains(stdout, `"id": 42`) {
		t.Errorf("stdout 应包含上传返回的 id: 42，实际: %q", stdout)
	}
	if !strings.Contains(stdout, `"path":`) {
		t.Errorf("stdout 应包含 path 字段，实际: %q", stdout)
	}
	if !strings.Contains(stdout, "test.jpg") {
		t.Errorf("stdout 应包含源文件名 test.jpg，实际: %q", stdout)
	}
}

// TestFileUploadCmd_NoTokenFlag 验证 file upload 命令不接受 --token flag（F16 契约）。
func TestFileUploadCmd_NoTokenFlag(t *testing.T) {
	// fileUploadCmd 在 init() 中不注册 --token flag（注释说明）。
	// 验证其 Run 回调中的 buildClient(cmd, "upload", ...) 不依赖 token。
	var tokenFlag = fileUploadCmd.Flags().Lookup("token")
	if tokenFlag != nil {
		t.Errorf("fileUploadCmd 不应有 --token flag（F16 契约：文件服务器独立，不需要业务域鉴权）")
	}
}
