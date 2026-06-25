//go:build integration
// +build integration

package integration

import (
	"os"
	"strings"
	"testing"
)

// TestHarFixtures_NoRealPII 守卫：所有 HAR fixture 不得含 CLAUDE.md
// 「敏感凭据记录」条款列出的真实学号/姓名，防止 PII 泄露回归。
//
// 背景：v0.3.2 修复轮发现 test/integration/har_fixtures/self_eval.json
// 仍含真实学号 TEST2025001 + 真实姓名「张三」，
// 与 client_test.go 占位约定（TEST2025001 / 张三）不一致。
func TestHarFixtures_NoRealPII(t *testing.T) {
	// 禁止模式：CLAUDE.md 第 281-291 行明文列出的真实凭据
	const (
		realStudentNumber = "TEST2025001"
		realStudentName   = "张三"
	)

	// 扫描 HAR fixture 目录（go test cwd = test/integration/，故用相对路径）
	fixtures := []string{
		"har_fixtures/self_eval.json",
		"har_fixtures/task_flow.json",
		"har_fixtures/military.json",
		"har_fixtures/class_meeting.json",
		"har_fixtures/labor.json",
	}

	for _, fixture := range fixtures {
		data, err := os.ReadFile(fixture)
		if err != nil {
			// fixture 可能尚未创建（按需生成），跳过不视为失败
			t.Logf("跳过（不存在）: %s", fixture)
			continue
		}
		s := string(data)
		if strings.Contains(s, realStudentNumber) {
			t.Errorf("%s 含真实学号 %s（CLAUDE.md 「敏感凭据记录」禁止）", fixture, realStudentNumber)
		}
		if strings.Contains(s, realStudentName) {
			t.Errorf("%s 含真实姓名 %s（CLAUDE.md 「敏感凭据记录」禁止）", fixture, realStudentName)
		}
	}
}
