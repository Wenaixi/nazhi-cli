package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestMain_PanicExitCode_IsTwo 以真实二进制实测锁定「panic 后进程必须以非零退出码结束」。
//
// 为什么必须起真实二进制：main() 的 panic recover 依赖 defer 链控制流，
// 而「recover 之后 main 正常返回、尾部 os.Exit 不可达」这类缺陷既无法在
// 测试进程内观测（os.Exit 会终止测试进程），也无法用 testing 子进程观测
// （testing 框架自带 recover 会先一步截住 panic）。只有真实二进制的
// main() 入口才走生产代码的 defer 链。
//
// 既有 TestMain_PanicRecover_* 全是 AST 静态扫描（只校验 recover/printError
// 字样存在），正是因此漏掉了历史上真实发生过的「panic 后 exit 0」缺陷：
// 进程以成功退出，脚本把崩溃当成功。
//
// 本测试用「注入 panic 的临时副本」运行：复制仓库源码到临时目录、注入一个
// 会 panic 的 rootCmd.Run、构建二进制并执行。任何把 panic 重新变成 exit 0
// 的改动都会让这里变红。
func TestMain_PanicExitCode_IsTwo(t *testing.T) {
	if testing.Short() {
		t.Skip("需构建二进制，跳过 short 模式")
	}
	goTool := goToolPath(t)

	// 复制 cmd/nazhi 到临时目录并注入 panic。
	srcDir := filepath.Join("..", "..")
	tmpRoot := t.TempDir()
	srcCopy := filepath.Join(tmpRoot, "src")
	if err := copyDir(filepath.Join(srcDir, "cmd"), filepath.Join(srcCopy, "cmd")); err != nil {
		t.Fatalf("复制 cmd 目录失败: %v", err)
	}
	for _, sub := range []string{"pkg", "internal"} {
		if err := copyDir(filepath.Join(srcDir, sub), filepath.Join(srcCopy, sub)); err != nil {
			t.Fatalf("复制 %s 目录失败: %v", sub, err)
		}
	}

	// 注入 panic：追加独立文件覆盖 rootCmd.Run，完全不改 main.go 任何一行
	// （早期版本用字符串替换 main.go，极脆——main.go 结构一变就静默失效，
	//  变异验证时表现为「注入失败」而非「断言捕获缺陷」）。
	inject := "package main\n\nimport \"github.com/spf13/cobra\"\n\nfunc init() {\n\trootCmd.Run = func(cmd *cobra.Command, args []string) {\n\t\tpanic(\"intentional panic for exit code probe\")\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(srcCopy, "cmd", "nazhi", "zz_panic_probe.go"),
		[]byte(inject), 0o644); err != nil {
		t.Fatalf("写入 panic 探针文件失败: %v", err)
	}
	// 模块定义：go build 需要在含 go.mod 的目录执行。
	for _, f := range []string{"go.mod", "go.sum"} {
		b, rerr := os.ReadFile(filepath.Join(srcDir, f))
		if rerr != nil {
			t.Fatalf("读取 %s 失败: %v", f, rerr)
		}
		if werr := os.WriteFile(filepath.Join(srcCopy, f), b, 0o644); werr != nil {
			t.Fatalf("写入 %s 失败: %v", f, werr)
		}
	}
	bin := filepath.Join(tmpRoot, "nazhi-panic.exe")
	build := exec.Command(goTool, "build", "-o", bin, "./cmd/nazhi")
	// 在临时源码副本的根目录构建——测试进程的工作目录是包目录 cmd/nazhi。
	build.Dir = srcCopy
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("构建 panic 探针二进制失败: %v\n%s", err, out)
	}

	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), "NAZHI_QUIET=1")
	out, err := cmd.CombinedOutput()

	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("运行 panic 探针失败: %v\n输出: %s", err, out)
	}

	if code == 0 {
		t.Fatalf("panic 后进程以 exit 0 结束——脚本会把崩溃误判为成功。输出:\n%s",
			truncateForLog(string(out)))
	}
	if code != 2 {
		t.Errorf("panic 后退出码期望 2（服务端错误档，printError HTTP 500），实际 %d。输出:\n%s",
			code, truncateForLog(string(out)))
	}
}

// copyDir 递归复制目录（仅测试用，跳过 _test.go 与测试数据）。
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			if err := copyDir(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// goToolPath 定位 go 可执行文件。
func goToolPath(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("未找到 go 可执行文件，跳过: %v", err)
	}
	return p
}

func truncateForLog(s string) string {
	const limit = 1200
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "\n...（已截断）"
}
