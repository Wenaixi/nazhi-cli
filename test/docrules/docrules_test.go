//go:build docrules
// +build docrules

// Package docrules 提供 README 文档与代码的一致性校验测试。
// 任何断言失败都说明文档落后于代码或代码落后于文档。
//
// 运行命令：go test -tags=docrules ./test/docrules/...
package docrules

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRoot 定位仓库根目录（go.mod 所在）。
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("找不到仓库根目录")
		}
		dir = parent
	}
}

func mustRead(t *testing.T, p string) string {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读取失败 %s: %v", p, err)
	}
	return string(data)
}

func mustGrep(t *testing.T, label, haystack, pattern string) {
	t.Helper()
	if !strings.Contains(haystack, pattern) {
		t.Errorf("[%s] 文档应包含 %q 但找不到", label, pattern)
	}
}

func mustNotGrep(t *testing.T, label, haystack, pattern string) {
	t.Helper()
	if strings.Contains(haystack, pattern) {
		t.Errorf("[%s] 文档不应包含 %q 但找到了", label, pattern)
	}
}

// ────────────────────────────────────────────────────────────────────
// CLI 文档校验（docs/README.md）
// ────────────────────────────────────────────────────────────────────

func TestCLI_WhoamiDocumentationMap(t *testing.T) {
	cli := mustRead(t, filepath.Join(repoRoot(t), "docs/README.md"))
	// 当前文档是源码地图，whoami 的 CLI 实现仍应被列出。
	mustGrep(t, "whoami CLI 映射", cli, "GetMyInfo")
}

func TestCurrentDocsMap(t *testing.T) {
	docs := mustRead(t, filepath.Join(repoRoot(t), "docs/README.md"))
	for _, entry := range []struct {
		label string
		text  string
	}{
		{"CLI whoami 映射", "GetMyInfo"},
		{"写实列表映射", "GetSubmittedCircles"},
		{"自评映射", "SubmitSelfEvaluation"},
		{"文件下载映射", "DownloadFile"},
	} {
		mustGrep(t, entry.label, docs, entry.text)
	}
}
