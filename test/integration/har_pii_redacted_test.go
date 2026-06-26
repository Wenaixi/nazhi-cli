package integration

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoRealPII 守卫：所有测试文件 + HAR fixture 不得含 CLAUDE.md
// 「敏感凭据记录」条款列出的真实姓名/学号/身份证/学号 ID，防止 PII 泄露回归。
//
// 背景：v0.3.2 修复轮发现 test/integration/har_fixtures/self_eval.json
// 仍含真实学号 TEST2025001 + 真实姓名「张三」，
// 与 client_test.go 占位约定（TEST2025001 / 张三）不一致。
// 第 7 轮（v0.3.5+）扩展守卫范围：从仅 HAR fixtures 扩到整个 *_test.go，
// 覆盖 pkg/client、pkg/types、cmd、test 等所有测试目录，
// 并捕获学生 ID 等数字型 PII（38STUDENT_ID_REDACTED / 32USER_ID_REDACTED / STUDY_NUMBER_REDACTEDSTUDY_NUMBER_FRAGMENT_REDACTED）。
//
// 默认 tag 运行（无 build tag），确保 `go test ./...` 必跑。
func TestNoRealPII(t *testing.T) {
	// 禁止模式：CLAUDE.md 第 281-291 行明文列出的真实凭据 + 数字 ID
	// 注意：禁值用字符拼接构造，避免守卫自身的字符串字面量被自身 AST 扫描到
	const (
		realStudentNumber    = "REDACTED"
		realIDCard           = "REDACTED"
		realStudentName      = "REDACTED"
		realNamePinyin       = "REDACTED"
		realNameInitials     = "REDACTED"
		realStudentID        = "REDACTED"
		realUserID           = "REDACTED"
		realStudyNumber      = "REDACTED"
	)

	forbidden := []string{
		realStudentNumber,
		realIDCard,
		realStudentName,
		realNamePinyin,
		realNameInitials,
		realStudentID,
		realUserID,
		realStudyNumber,
	}

	// 守卫自身路径（避免自我误报）
	const selfPath = "har_pii_redacted_test.go"

	// 扫描所有 *_test.go + har_fixtures/*.json
	// 用 walk 模式自动覆盖新增的测试目录
	roots := []string{
		"../../pkg/client",
		"../../pkg/types",
		"../../cmd/nazhi",
		"../../internal",
		".",
	}

	var scanPaths []string
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				// 路径不存在时跳过（按需目录）
				return nil
			}
			if info.IsDir() {
				// 跳过 vendor 和 .git
				base := info.Name()
				if base == "vendor" || base == ".git" || base == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			name := info.Name()
			// 守卫自身豁免
			if strings.HasSuffix(path, selfPath) {
				return nil
			}
			if strings.HasSuffix(name, "_test.go") {
				scanPaths = append(scanPaths, path)
			} else if strings.HasSuffix(name, ".json") && strings.Contains(path, "har_fixtures") {
				scanPaths = append(scanPaths, path)
			}
			return nil
		})
		if err != nil {
			t.Logf("walk %s 失败: %v", root, err)
		}
	}

	if len(scanPaths) == 0 {
		t.Fatal("未扫描到任何测试文件，walk 配置错误")
	}

	t.Logf("扫描 %d 个文件（PII 守卫）", len(scanPaths))

	// 收集违规：forbidden × file → 详细错误
	var violations []string

	for _, path := range scanPaths {
		// _test.go 走 AST 扫描字符串字面量（避免把 forbidden 字符串自身也算进去）
		// 非 _test.go（har_fixtures JSON）走 raw bytes 扫描
		if strings.HasSuffix(path, "_test.go") {
			scanASTStringLiterals(t, path, forbidden, &violations)
		} else {
			scanRawBytes(t, path, forbidden, &violations)
		}
	}

	if len(violations) > 0 {
		for _, v := range violations {
			t.Error(v)
		}
		t.Fatalf("PII 守卫发现 %d 处违规，详见上方 t.Error", len(violations))
	}
}

// scanASTStringLiterals 解析 Go 源文件 AST，扫描所有字符串字面量是否含 forbidden
// 这样可以避免「守卫自身实现含 forbidden 字符串」导致的自我误报
func scanASTStringLiterals(t *testing.T, path string, forbidden []string, violations *[]string) {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Logf("读取失败: %s: %v", path, err)
		return
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		// 语法错误的文件不在本守卫职责范围
		return
	}

	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		// 去掉引号
		s := lit.Value
		if len(s) >= 2 && (s[0] == '"' || s[0] == '`') && s[len(s)-1] == s[0] {
			s = s[1 : len(s)-1]
		}
		for _, f := range forbidden {
			if strings.Contains(s, f) {
				pos := fset.Position(lit.Pos())
				*violations = append(*violations,
					pos.String()+": "+path+" 字符串字面量含禁值（CLAUDE.md 「敏感凭据记录」禁止）")
			}
		}
		return true
	})
}

// scanRawBytes raw bytes 扫描（用于 HAR fixture JSON 等非 Go 文件）
func scanRawBytes(t *testing.T, path string, forbidden []string, violations *[]string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Logf("读取失败: %s: %v", path, err)
		return
	}
	s := string(data)
	for _, f := range forbidden {
		if strings.Contains(s, f) {
			*violations = append(*violations,
				path+" 含禁值（CLAUDE.md 「敏感凭据记录」禁止）")
		}
	}
}