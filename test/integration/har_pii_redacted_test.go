package integration

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestNoRealPII 守卫：所有测试文件 + HAR fixture 不得含已知 PII 敏感信息。
//
// 本守卫采用「双检」策略：
//   - 哈希比对：将扫描到的字符串值计算 SHA-256，
//     与已知 PII 的不可逆哈希摘要比对。哈希摘要无法反推出原文。
//   - 模式匹配：扫描已知的 PII 格式（学号 G\d{15}），捕获新值。
//
// 为什么用 SHA-256 而非明文：
// v0.3.5 之前的守卫直接把真实姓名、学号、身份证号等写在 forbidden 常量里，
// 导致守卫自身成了新的泄露源。改用不可逆的 SHA-256 后，
// 任何人看到源码也无法倒推出你的真实信息。
//
// 安全保证：map 中只存储 64 字符的 hex 字符串，不是 PII 原文。
// SHA-256 是单向函数，从 hex 值不可能反推出原文。
//
// 背景：HAR fixture 中的真实数据已在 commit 73552ff 替换为占位值
// （TEST2025001 / 张三），本守卫防止后续改动引入新的 PII 回归。
//
// 默认 tag 运行（无 build tag），确保 `go test ./...` 必跑。
func TestNoRealPII(t *testing.T) {
	// 预计算的 SHA-256 hex 摘要 → PII 类型描述
	// ⚠️ 这些 hex 字符串是离线计算的，不可逆。
	// 任何人看到这些 hash 值也无法反推出你的真实信息。
	piiHexMap := map[string]string{
		// 以下 hash 由离线计算得到，原始 PII 文件已被清理。
		// hash 值本身不构成泄露：SHA-256 是单向函数。
		"931577eabd71afd0475218f0b676da9712c5f03150f5fc3035109d9dcdd00896": "学号（含 G 前缀）",
		"8182e345670a08de6afcc00ed0688a180a812f3c544a28fb5330c9af3b7c8974": "身份证号",
		"537d18920afded93ad219b5e59370f40bbc59c07a00b21484795c8d4ff849743": "真实姓名",
		"0e6039c29959f7e811e170a073d72de4258e70a16942ff7b6166693eaaa12f2a": "姓名拼音",
		"2cfb730190610bea00cae9272b115c1c194c7b75e11fbe906fffc1edebfe7b47": "姓名首字母",
		"b77c0ca27b8b524c28fc07d27223c9d53eb931c4d3c825a841a7d3bd044a5958": "数字 PII（学生 ID）",
		"7f0371d520465dab795376fe9042a9424f62f1011982389d6375654f23e87c68": "数字 PII（用户 ID）",
		"eee64c13ed9c17d18deb6562a5382636cee3821ad81c5da6b5d1eb8970864ce9": "数字 PII（学号 ID）",
	}

	// 已知 PII 格式的正则模式（额外防线，捕获新值）
	// 注意：Go regexp (RE2) 不支持 \b 单词边界，用前置检查替代
	idCardPat := regexp.MustCompile(`[^0-9](\d{17}[\dXx])[^0-9]`)
	patterns := []struct {
		re   *regexp.Regexp
		desc string
	}{
		{regexp.MustCompile(`G\d{15}`), "学号格式：G 前缀 + 15 位数字"},
		{idCardPat, "身份证号格式：18 位数字"},
	}

	// 守卫自身路径（避免自我误报）
	const selfPath = "har_pii_redacted_test.go"

	roots := []string{
		"../../pkg/client",
		"../../pkg/types",
		"../../cmd/nazhi",
		"../../internal",
		"../../.claude",
		".",
	}

	var violations []string

	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // 路径不存在时跳过
			}
			if info.IsDir() {
				base := info.Name()
				if base == "vendor" || base == ".git" || base == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, selfPath) {
				return nil
			}

			name := info.Name()
			if !strings.HasSuffix(name, "_test.go") &&
				!(strings.HasSuffix(name, ".json") && strings.Contains(path, "har_fixtures")) {
				return nil
			}

			raw, err := os.ReadFile(path)
			if err != nil {
				t.Logf("读取失败: %s: %v", path, err)
				return nil
			}

			// 1) 模式扫描：检查已知 PII 格式（如 G\d{15}）
			for _, pat := range patterns {
				matches := pat.re.FindAllSubmatch(raw, -1)
				for _, m := range matches {
					if len(m) >= 2 {
						violations = append(violations,
							fmt.Sprintf("%s: 匹配 PII 模式 [%s]：%q", path, pat.desc, string(m[1])))
					} else if len(m) == 1 {
						violations = append(violations,
							fmt.Sprintf("%s: 匹配 PII 模式 [%s]：%q", path, pat.desc, string(m[0])))
					}
				}
			}

			// 2) 哈希比对：对字符串值计算 SHA-256 并与已知 PII 哈希集比对
			var hashViolations []string
			if strings.HasSuffix(name, "_test.go") {
				hashViolations = hashScanGoSource(path, raw, piiHexMap)
			} else if strings.HasSuffix(name, ".json") {
				hashViolations = hashScanJSON(path, raw, piiHexMap)
			}
			violations = append(violations, hashViolations...)

			return nil
		})
		if err != nil {
			t.Logf("walk %s 失败: %v", root, err)
		}
	}

	if len(violations) > 0 {
		for _, v := range violations {
			t.Error(v)
		}
		t.Fatalf("PII 守卫发现 %d 处违规，详见上方 t.Error", len(violations))
	}
}

// hashScanGoSource 解析 Go 源文件 AST，对每个字符串字面量计算 SHA-256 哈希，
// 与已知 PII 哈希集比对。
func hashScanGoSource(path string, src []byte, piiHexMap map[string]string) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil // 语法错误的文件不在本守卫职责范围
	}

	var violations []string
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		// 去掉引号（" 或 `）
		s := lit.Value
		if len(s) >= 2 {
			quote := s[0]
			if (quote == '"' || quote == '`') && s[len(s)-1] == quote {
				s = s[1 : len(s)-1]
			}
		}
		hexStr := fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
		if desc, found := piiHexMap[hexStr]; found {
			pos := fset.Position(lit.Pos())
			violations = append(violations,
				fmt.Sprintf("%s:%d: Go 字符串字面量哈希匹配 PII 类型 [%s]",
					path, pos.Line, desc))
		}
		return true
	})
	return violations
}

// hashScanJSON 解析 JSON 树，遍历所有字符串值，
// 对每个字符串值计算 SHA-256 哈希并与已知 PII 哈希集比对。
func hashScanJSON(path string, data []byte, piiHexMap map[string]string) []string {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil // 非法 JSON 不在守卫范围
	}

	var violations []string
	walkJSONValue(v, func(s string) {
		hexStr := fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
		if desc, found := piiHexMap[hexStr]; found {
			violations = append(violations,
				fmt.Sprintf("%s: JSON 字符串值哈希匹配 PII 类型 [%s]", path, desc))
		}
	})
	return violations
}

// walkJSONValue 递归遍历 JSON 值的所有字符串字段。
func walkJSONValue(v interface{}, visit func(string)) {
	switch val := v.(type) {
	case string:
		visit(val)
	case map[string]interface{}:
		for _, child := range val {
			walkJSONValue(child, visit)
		}
	case []interface{}:
		for _, child := range val {
			walkJSONValue(child, visit)
		}
	}
}
