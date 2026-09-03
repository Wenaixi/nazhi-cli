package types

import "testing"

// TestSetSubmittedByStatus_SubstringMatching 锁定子串匹配语义：
// circleTaskStatus 是自由文本族（docs/README.md:66 自证「上传期/已结束 × 已提交/未提交」组合，
// managementLeftBottom.vue:37 前端用 indexOf 子串匹配「已结束」），全等匹配会漏接变体文案。
func TestSetSubmittedByStatus_SubstringMatching(t *testing.T) {
	cases := []struct {
		status  string
		wantSub bool // 期望 Submitted
	}{
		{"未提交", false},
		{"上传期 未提交", false},
		{"已结束 未提交", false}, // 全等黑名单漏接的关键形态
		{"审核中 未提交", false}, // 未来新增文案的容错
		{"上传期 已提交", true},
		{"已结束 已提交", true},
		{"进行中", true},
		// H-A 修复（2026-09-03）：空串保守视为未提交——平台不返回 status
		// 字段时误判已提交会隐藏学生提交入口（原断言 {"", true} 固化的是缺陷）。
		{"", false},
		{"   ", false}, // 纯空白同样视为未提交
	}
	for _, tc := range cases {
		task := &Task{CircleTaskStatus: tc.status}
		task.SetSubmittedByStatus()
		if task.Submitted != tc.wantSub {
			t.Errorf("status=%q: 期望 Submitted=%v，实际 %v", tc.status, tc.wantSub, task.Submitted)
		}
	}
}
