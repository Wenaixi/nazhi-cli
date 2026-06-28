//go:build verify

// 包 verify 包含只在显式标签下运行的仓库元数据检查。
//
// 默认 `go test ./...` 不会编译这个包（避免无端触发 git 调用）。
// 手动跑：`go test -tags=verify ./test/integration/verify_gitignore/`
package verify

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot 返回仓库根目录（go.mod 所在）。
// 测试运行时 cwd 不一定是仓库根，必须显式 cd 才能跑 git ls-files。
func repoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		t.Fatalf("go env GOMOD 失败: %v", err)
	}
	mod := strings.TrimSpace(string(out))
	if mod == "" {
		t.Fatalf("不在 Go 模块内（找不到 go.mod）")
	}
	return filepath.Dir(mod)
}

// TestCLAUDEMdNotTracked 守卫：CLAUDE.md 不得出现在 git index 中。
//
// .gitignore 第 49 行已声明 `CLAUDE.md`,但已跟踪文件需要
// `git rm --cached` 显式 untrack,否则 push 时会泄露到远端。
// (fix: review-tdd round 14 F1)
func TestCLAUDEMdNotTracked(t *testing.T) {
	root := repoRoot(t)
	cmd := exec.Command("git", "-C", root, "ls-files", "CLAUDE.md")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files 执行失败: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "" {
		t.Fatalf("CLAUDE.md 仍在 git index 中（会被 push 泄露）: %q", got)
	}
}
