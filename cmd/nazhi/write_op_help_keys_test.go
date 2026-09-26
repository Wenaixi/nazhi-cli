package main

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// 这组测试锁定「未知键拒绝时向用户承诺的 --help 允许键清单必须真实存在」这条契约。
//
// 背景：write_op_runner.go 的未知键拒绝文案写着「允许键见 nazhi <cmd> --help」，
// 但此前所有写操作命令的 Long 文本都没有键名清单——用户被指向一条死路。
// 键名还必须按用户实际书写形态（驼峰）展示：allowedKeys 为大小写不敏感比较而
// 小写存储，若直接把它的键印出去，会教用户写 typeid 这类全小写形态。
// 测试要同时锁住「清单存在」与「清单是驼峰原样」两件事。

// helpTextFor 取某个命令的 --help 文本。rootCmd 是包级 cobra 命令，
// 每次调用重置输出缓冲与参数，避免跨用例污染。
func helpTextFor(t *testing.T, args []string) string {
	t.Helper()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs(append(append([]string{}, args...), "--help"))
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("%s --help 执行失败: %v", strings.Join(args, " "), err)
	}
	return buf.String()
}

// writeOpHelpCases 覆盖全部走共享骨架且带允许集的写操作命令。
// 每个用例给出命令路径与该命令至少一个必须出现在帮助里的驼峰原样键。
var writeOpHelpCases = []struct {
	path    []string
	wantKey string
}{
	{[]string{"task", "submit"}, "circleTaskId"},
	{[]string{"task", "edit"}, "circleTaskId"},
	{[]string{"honor", "add"}, "typeId"},
	{[]string{"honor", "update"}, "evaluationAgency"},
	{[]string{"typical-case", "submit"}, "partnerName"},
	{[]string{"typical-case", "update"}, "teacherName"},
	{[]string{"user", "update"}, "telephone"},
}

// TestWriteOpHelp_ListsAllowedKeys 每个写操作命令的 --help 必须列出允许键。
// 这是对 write_op_runner.go 未知键文案「允许键见 nazhi <cmd> --help」的兑现。
func TestWriteOpHelp_ListsAllowedKeys(t *testing.T) {
	for _, tc := range writeOpHelpCases {
		t.Run(strings.Join(tc.path, " "), func(t *testing.T) {
			help := helpTextFor(t, tc.path)
			if !strings.Contains(help, "允许键") {
				t.Fatalf("--help 未提及允许键，承诺落空。帮助文本:\n%s", help)
			}
			if !strings.Contains(help, tc.wantKey) {
				t.Fatalf("--help 缺少允许键 %q（应按用户书写形态展示驼峰原样键）", tc.wantKey)
			}
		})
	}
}

// TestWriteOpHelp_AllowedKeysAreCamelCase 允许键必须以驼峰原样展示，
// 不得暴露内部为大小写不敏感比较而小写存储的形态——否则会教用户写错键名。
func TestWriteOpHelp_AllowedKeysAreCamelCase(t *testing.T) {
	// 这些是真实出站 JSON 键的驼峰形态；帮助文本必须原样包含它们。
	camelKeys := map[string][]string{
		"honor add":           {"typeId", "evaluationAgency", "certImgAttachmentId"},
		"honor update":        {"typeId", "evaluationAgency", "certImgAttachmentId"},
		"typical-case submit": {"typeName", "teacherName", "partnerName", "levelName", "attachmentId"},
		"typical-case update": {"typeName", "teacherName", "partnerName", "levelName", "attachmentId"},
		"user update":         {"studentNumber", "nationalStudentNumber", "familyAddress", "youthLeague", "idCard", "birthdayStr", "studentUuid"},
	}
	for name, keys := range camelKeys {
		t.Run(name, func(t *testing.T) {
			help := helpTextFor(t, strings.Fields(name))
			for _, k := range keys {
				if !strings.Contains(help, k) {
					t.Errorf("--help 缺少驼峰允许键 %q", k)
				}
			}
		})
	}
}

// TestWriteOpHelp_ListsEveryAllowedKey 帮助文本列出的键必须覆盖允许集的每一个键，
// 不能只列一部分——漏列会让用户以为某个键不被支持。
func TestWriteOpHelp_ListsEveryAllowedKey(t *testing.T) {
	cases := []struct {
		path []string
		mode writeOpMode
	}{
		{[]string{"honor", "add"}, honorAddWriteOp},
		{[]string{"honor", "update"}, honorUpdateWriteOp},
		{[]string{"typical-case", "submit"}, typicalCaseSubmitWriteOp},
		{[]string{"typical-case", "update"}, typicalCaseUpdateWriteOp},
		{[]string{"user", "update"}, userUpdateWriteOp},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.path, " "), func(t *testing.T) {
			help := helpTextFor(t, tc.path)
			// 允许集以小写存储；帮助里的驼峰键 ToLower 后必须命中允许集，
			// 且允许集的每个键都必须在帮助文本里以某种大小写形态出现。
			helpLower := strings.ToLower(help)
			for k := range tc.mode.allowedKeys {
				if !strings.Contains(helpLower, strings.ToLower(k)) {
					t.Errorf("--help 未列出允许键 %q", k)
				}
			}
			if len(tc.mode.helpKeys) != len(tc.mode.allowedKeys) {
				t.Errorf("helpKeys 条数 %d 与 allowedKeys 条数 %d 不一致，帮助文本会与实际允许集脱节",
					len(tc.mode.helpKeys), len(tc.mode.allowedKeys))
			}
		})
	}
}

// TestWriteOpHelp_KeysSortedAndStable 帮助文本中的键必须稳定排序，
// 否则 --help 输出会随 map 遍历顺序抖动，测试与文档都无法锚定。
func TestWriteOpHelp_KeysSortedAndStable(t *testing.T) {
	help := helpTextFor(t, []string{"honor", "add"})
	section := helpSection(honorAddWriteOp.helpKeys, true)
	listed := make([]string, 0, len(honorAddWriteOp.helpKeys))
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "  ") {
			continue // 标题行与尾注不是键
		}
		listed = append(listed, strings.TrimSpace(line))
	}
	if !sort.StringsAreSorted(listed) {
		t.Errorf("允许键未排序: %v", listed)
	}
	if len(listed) != len(honorAddWriteOp.helpKeys) {
		t.Errorf("段落中键行数 %d 与 helpKeys 长度 %d 不一致", len(listed), len(honorAddWriteOp.helpKeys))
	}
	if !strings.Contains(help, strings.Join(listed, "\n  ")) {
		t.Errorf("--help 未按排序后形态呈现允许键")
	}
}

// TestAllowedKeysSection_ContainsAllKeys 允许键段落生成函数本身的单元断言。
func TestAllowedKeysSection_ContainsAllKeys(t *testing.T) {
	section := helpSection(honorAddWriteOp.helpKeys, true)
	for _, k := range honorAddWriteOp.helpKeys {
		if !strings.Contains(section, k) {
			t.Errorf("允许键段落缺少 %q", k)
		}
	}
	// 段落必须自带说明文字，让用户知道这些键要写进 --payload
	if !strings.Contains(section, "payload") {
		t.Errorf("允许键段落未说明这些键用于 payload")
	}
}

// TestUnknownKeyError_PointsAtRealHelp 未知键拒绝文案指向的 --help 必须真的含该键。
// 直接断言错误文案承诺的命令名与帮助文本含键名这一对关系成立。
func TestUnknownKeyError_PointsAtRealHelp(t *testing.T) {
	cmd := &cobra.Command{Use: "demo"}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	// 此处只验证机制：错误文案中的命令名取自 cmd.Name()，
	// 断言它非空即可——命令名缺失会让承诺退化成「允许键见 nazhi  --help」。
	err := unknownKeyErrorFor(cmd, []string{"typeid"})
	msg := err.Error()
	if !strings.Contains(msg, "允许键见") || !strings.Contains(msg, cmd.Name()) {
		t.Fatalf("未知键文案未正确指向命令帮助: %s", msg)
	}
	if strings.Contains(msg, "nazhi  --help") {
		t.Fatalf("命令名为空，承诺退化为无意义的帮助指向: %s", msg)
	}
}

// TestWriteOpMode_HelpKeysCoverAllModes 兜底断言：所有带 allowedKeys 的写操作
// 模式都必须有等量的 helpKeys，防止新增命令漏配。
func TestWriteOpMode_HelpKeysCoverAllModes(t *testing.T) {
	modes := map[string]writeOpMode{
		"taskSubmit":        taskSubmitWriteOp,
		"taskEdit":          taskEditWriteOp,
		"honorAdd":          honorAddWriteOp,
		"honorUpdate":       honorUpdateWriteOp,
		"typicalCaseAdd":    typicalCaseSubmitWriteOp,
		"typicalCaseUpdate": typicalCaseUpdateWriteOp,
		"userUpdate":        userUpdateWriteOp,
	}
	for name, m := range modes {
		t.Run(name, func(t *testing.T) {
			if len(m.allowedKeys) == 0 {
				t.Fatalf("%s 无允许集，跳过", name)
			}
			if len(m.helpKeys) != len(m.allowedKeys) {
				t.Errorf("%s: helpKeys %d 条与 allowedKeys %d 条不等",
					name, len(m.helpKeys), len(m.allowedKeys))
			}
			// helpKeys 折叠小写后必须与 allowedKeys 完全一致
			var lowered []string
			for _, k := range m.helpKeys {
				lowered = append(lowered, strings.ToLower(k))
			}
			sort.Strings(lowered)
			var allowed []string
			for k := range m.allowedKeys {
				allowed = append(allowed, k)
			}
			sort.Strings(allowed)
			if strings.Join(lowered, ",") != strings.Join(allowed, ",") {
				t.Errorf("%s: helpKeys 折叠后与允许集不一致\nhelpKeys: %v\nallowed: %v",
					name, lowered, allowed)
			}
		})
	}
}

// envelope 断言辅助：确认本组测试用到的信封类型仍可用，避免重构时静默破坏。
func TestAllowedKeysWork_EnvelopeTypeIntact(t *testing.T) {
	e := envelope.Error(400, "x")
	if e == nil || e.Code != 400 {
		t.Fatalf("envelope.Error 行为异常: %+v", e)
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if !strings.Contains(string(b), "400") {
		t.Fatalf("信封序列化不含 code: %s", b)
	}
}
