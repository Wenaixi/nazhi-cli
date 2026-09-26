package client

import (
	"bytes"
	"encoding/json"
	"testing"
)

// 这组测试直接锁定 appendPageRange 的 JSON 扫描行为，覆盖 offset/limit 裁剪
// 语义的唯一实现路径。补这组测试的原因：现有 limit 端到端测试的 mock 记录全是
// {"id": N} 平凡对象，无法触发字符串边界与转义分支——一旦有人误改 depth/inString
// 逻辑，现有测试会全绿而 limit 输出静默产出非法 JSON。
//
// 依据：写实记录的 content 字段（pkg/types/circle.go Content）由学生自由填写，
// 含花括号、引号、反斜杠是现实输入，不是理论构造。

// newScannerFixture 把一页 dataList 原始字节包进 assembleCirclesLimitJSON 期望的
// 数组外壳，与 appendPageRange 的实际入参形态保持一致。
func newScannerFixture(objects ...string) []byte {
	buf := bytes.NewBuffer(make([]byte, 0, 256))
	buf.WriteByte('[')
	for i, obj := range objects {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(obj)
	}
	buf.WriteByte(']')
	return buf.Bytes()
}

// runScanner 对一页原始 JSON 走一遍裁剪，返回拼装结果与计数。
func runScanner(pageRaw []byte, offset, limit int) (string, int, int) {
	buf := bytes.NewBuffer(make([]byte, 0, 2048))
	first := true
	skipped, taken := appendPageRange(buf, pageRaw, &first, taken0, offset, limit, skipped0)
	return buf.String(), skipped, taken
}

const (
	taken0   = 0
	skipped0 = 0
)

// assertLegalJSON 断言拼接结果是合法 JSON 数组，且元素个数符合预期。
// 这是本组测试的核心断言：任何切分错误都表现为非法 JSON 或元素数错乱。
func assertLegalJSON(t *testing.T, out string, wantElems int) {
	t.Helper()
	trimmed := out
	if len(trimmed) == 0 {
		t.Fatalf("拼接结果为空，期望 %d 个元素", wantElems)
	}
	// 逐元素 json.Unmarshal 校验：非法切片会在此暴露
	var elems []json.RawMessage
	if err := json.Unmarshal([]byte("["+trimmed+"]"), &elems); err != nil {
		t.Fatalf("拼接结果不是合法 JSON 数组: %v\n原始字节: %s", err, trimmed)
	}
	if len(elems) != wantElems {
		t.Fatalf("元素个数 = %d，期望 %d\n原始字节: %s", len(elems), wantElems, trimmed)
	}
}

// TestAppendPageRange_StringValueWithBraces 值内含未转义花括号时不得误判对象边界。
// 花括号出现在 JSON 字符串内是合法的；若扫描器不感知字符串边界，会在 depth==0
// 处遇到字符串内的逗号而提前切分，切出非法 JSON。
func TestAppendPageRange_StringValueWithBraces(t *testing.T) {
	page := newScannerFixture(
		`{"id":1,"content":"我用{a}做了手工活"}`,
		`{"id":2,"content":"b"}`,
	)
	out, skipped, taken := runScanner(page, 0, 10)
	assertLegalJSON(t, out, 2)
	if skipped != 2 || taken != 2 {
		t.Fatalf("skipped=%d taken=%d，期望均为 2", skipped, taken)
	}
	// 内容必须逐字保留——切分不得改动任何字节
	if !bytes.Contains([]byte(out), []byte("我用{a}做了手工活")) {
		t.Fatalf("字符串内花括号内容被破坏: %s", out)
	}
}

// TestAppendPageRange_EscapedQuotes 值内含转义引号时字符串状态不得提前退出。
// 反斜杠转义的引号不是字符串边界；若误判为边界，后续逗号会被当成顶层分隔符。
func TestAppendPageRange_EscapedQuotes(t *testing.T) {
	page := newScannerFixture(
		`{"id":1,"content":"他说\"你好\"然后走了"}`,
		`{"id":2,"content":"b"}`,
	)
	out, skipped, taken := runScanner(page, 0, 10)
	assertLegalJSON(t, out, 2)
	if skipped != 2 || taken != 2 {
		t.Fatalf("skipped=%d taken=%d，期望均为 2", skipped, taken)
	}
	if !bytes.Contains([]byte(out), []byte(`\"你好\"`)) {
		t.Fatalf("转义引号内容被破坏: %s", out)
	}
}

// TestAppendPageRange_EscapedBackslashBeforeQuote 转义反斜杠紧邻引号时必须整体跳过。
//
// 构造要点：路径文本 C:\\ 后紧跟一个闭引号。若反斜杠未被跳过，那个引号会被
// 误判为字符串闭边界，inString 提前翻转为 false，后续所有字节被当成结构字符
// 扫描——depth 与顶层逗号判定随之失真，切出的 JSON 立即非法。
//
// 注意不要用「值内含 \\d」这类普通转义字母的用例：跳过失效时 d 仍是非引号
// 非括号字符，结果碰巧正确，测试会恒绿而测不到任何东西。
func TestAppendPageRange_EscapedBackslashBeforeQuote(t *testing.T) {
	page := newScannerFixture(
		`{"id":1,"content":"路径C:\\"}`,
		`{"id":2,"content":"b"}`,
	)
	out, skipped, taken := runScanner(page, 0, 10)
	assertLegalJSON(t, out, 2)
	if skipped != 2 || taken != 2 {
		t.Fatalf("skipped=%d taken=%d，期望均为 2", skipped, taken)
	}
	if !bytes.Contains([]byte(out), []byte(`C:\\`)) {
		t.Fatalf("转义反斜杠内容被破坏: %s", out)
	}
}

// TestAppendPageRange_EscapedQuoteThenBraceUnderDepth 转义引号之后紧跟花括号时，
// 深度跟踪与字符串状态必须同时正确。
//
// 这是本组测试里唯一能区分「跳过转义字符」实现与「不跳过」实现的构造：
// 值内先出现转义引号 \"，其后紧跟一个位于字符串内的右花括号 } 与逗号。
// 不跳过转义时，那个引号被误判为闭边界，字符串状态提前退出，随后的 }
// 被计入 depth 递减，depth 从 0 变负，其后的逗号便在 depth==0 处被误判为
// 顶层元素分隔符——数组被从对象内部切断，拼出非法 JSON。
func TestAppendPageRange_EscapedQuoteThenBraceUnderDepth(t *testing.T) {
	page := newScannerFixture(
		`{"id":1,"content":"他说\"}，然后走了"}`,
		`{"id":2,"content":"b"}`,
	)
	out, skipped, taken := runScanner(page, 0, 10)
	assertLegalJSON(t, out, 2)
	if skipped != 2 || taken != 2 {
		t.Fatalf("skipped=%d taken=%d，期望均为 2", skipped, taken)
	}
	if !bytes.Contains([]byte(out), []byte(`\"}，然后走了`)) {
		t.Fatalf("转义引号后的花括号内容被破坏: %s", out)
	}
}

// TestAppendPageRange_UnbalancedBracesInStringValue 值内出现多于结构本身的左花括号时，
// 字符串状态必须阻止 depth 被虚增。
//
// 这是唯一能区分「字符串起始被正确标记」与「未标记」两种实现的构造：
// 进入对象时 depth 已是 1，值内两个 { 会把它抬到 3。若扫描器未进入字符串状态
// （inString 恒为 false），这两个 { 被计入深度，随后的 } 把 depth 拉回 0，
// 其后的顶层逗号便在 depth==0 处被误判为元素分隔符——数组在对象内部被切断，
// 拼出非法 JSON。学生正文里出现连续花括号是现实输入，不是理论构造。
func TestAppendPageRange_UnbalancedBracesInStringValue(t *testing.T) {
	page := newScannerFixture(
		`{"id":1,"content":"我用{{{ 做标记"}`,
		`{"id":2,"content":"b"}`,
	)
	out, skipped, taken := runScanner(page, 0, 10)
	assertLegalJSON(t, out, 2)
	if skipped != 2 || taken != 2 {
		t.Fatalf("skipped=%d taken=%d，期望均为 2", skipped, taken)
	}
	if !bytes.Contains([]byte(out), []byte("我用{{{ 做标记")) {
		t.Fatalf("字符串内花括号内容被破坏: %s", out)
	}
}

// TestAppendPageRange_NestedObjectAndArray 嵌套对象与数组不得影响顶层分隔判定。
// 嵌套使 depth 变化，只有 depth 回到 0 后的逗号才是数组元素分隔符。
func TestAppendPageRange_NestedObjectAndArray(t *testing.T) {
	page := newScannerFixture(
		`{"id":1,"meta":{"tags":["a","b"],"nested":{"deep":1}}}`,
		`{"id":2,"list":[1,2,3]}`,
	)
	out, skipped, taken := runScanner(page, 0, 10)
	assertLegalJSON(t, out, 2)
	if skipped != 2 || taken != 2 {
		t.Fatalf("skipped=%d taken=%d，期望均为 2", skipped, taken)
	}
}

// TestAppendPageRange_EscapedQuoteAndBraceCombined 转义引号与花括号混合出现时
// 两种状态机必须同时正确——这正是现实正文里最容易触发扫描错误的组合。
func TestAppendPageRange_EscapedQuoteAndBraceCombined(t *testing.T) {
	page := newScannerFixture(
		`{"id":1,"content":"\"{x}\" 结束"}`,
		`{"id":2,"content":"b"}`,
	)
	out, skipped, taken := runScanner(page, 0, 10)
	assertLegalJSON(t, out, 2)
	if skipped != 2 || taken != 2 {
		t.Fatalf("skipped=%d taken=%d，期望均为 2", skipped, taken)
	}
}

// TestAppendPageRange_TrailingBackslashBeforeQuote 行尾反斜杠紧邻闭引号时不得越界读取。
// 字符串以 \\ 结尾时闭引号前的反斜杠是转义对的一半，扫描指针不得跳过闭引号，
// 否则字符串状态永不闭合，整页剩余内容会被当成字符串吞掉。
func TestAppendPageRange_TrailingBackslashBeforeQuote(t *testing.T) {
	page := newScannerFixture(
		`{"id":1,"content":"结尾\\"}`,
		`{"id":2,"content":"b"}`,
	)
	out, skipped, taken := runScanner(page, 0, 10)
	assertLegalJSON(t, out, 2)
	if skipped != 2 || taken != 2 {
		t.Fatalf("skipped=%d taken=%d，期望均为 2", skipped, taken)
	}
}

// TestAppendPageRange_OffsetAndLimitAcrossPages offset 跨页裁剪时元素边界不得错位。
// offset 落在第二页时，第一页全部跳过、第二页从 offset 位置开始取。
func TestAppendPageRange_OffsetAndLimitAcrossPages(t *testing.T) {
	page := newScannerFixture(
		`{"id":1,"content":"含{a}的花括号"}`,
		`{"id":2,"content":"含\"引号\"的内容"}`,
		`{"id":3,"content":"c"}`,
	)
	out, skipped, taken := runScanner(page, 1, 1)
	assertLegalJSON(t, out, 1)
	if skipped != 2 || taken != 1 {
		t.Fatalf("skipped=%d taken=%d，期望 2/1", skipped, taken)
	}
	if !bytes.Contains([]byte(out), []byte(`含\"引号\"的内容`)) {
		t.Fatalf("offset 裁剪取错元素: %s", out)
	}
}

// TestAppendPageRange_LimitZeroTakesNothing limit=0 时不得写入任何元素。
// 提前退出条件 taken < limit 在 taken>=0 时立即为假，扫描器应不产出。
func TestAppendPageRange_LimitZeroTakesNothing(t *testing.T) {
	page := newScannerFixture(`{"id":1}`, `{"id":2}`)
	out, _, taken := runScanner(page, 0, 0)
	if taken != 0 {
		t.Fatalf("limit=0 时 taken=%d，期望 0", taken)
	}
	if out != "" {
		t.Fatalf("limit=0 时不应产出内容，实际: %s", out)
	}
}

// TestAppendPageRange_EmptyPage 空页与空数组不得产出悬挂逗号或非法 JSON。
func TestAppendPageRange_EmptyPage(t *testing.T) {
	for name, page := range map[string][]byte{
		"空数组":   []byte(`[]`),
		"空对象数组": []byte(`[{},{}]`),
	} {
		t.Run(name, func(t *testing.T) {
			out, skipped, taken := runScanner(page, 0, 10)
			if len(out) > 0 {
				assertLegalJSON(t, out, skipped)
			}
			if taken != skipped {
				t.Fatalf("空页场景 taken=%d 与 skipped=%d 不一致", taken, skipped)
			}
		})
	}
}
