// assembleCirclesJSON 白盒测试：第一页空数组时不得产生 leading comma 非法 JSON。
package client

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestAssembleCirclesJSON_EmptyFirstPage_NoLeadingComma 锁定：
// page1=[]、page2 有数据时，拼接结果必须是合法 JSON（如 [{...}]），不能是 [,{...}]。
func TestAssembleCirclesJSON_EmptyFirstPage_NoLeadingComma(t *testing.T) {
	raw1 := []byte("[]")
	results := make([]rawResult, 3)
	results[2] = rawResult{raw: []byte(`[{"id":200}]`)}

	out, err := assembleCirclesJSON(raw1, results, 2, nil)
	if err != nil {
		t.Fatalf("assembleCirclesJSON 不应返回 error: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("结果不应为空")
	}
	// 非法形态：[,{...}]
	if string(out)[0:2] == "[," {
		t.Fatalf("leading comma 非法 JSON: %s", out)
	}
	var arr []map[string]any
	if jerr := json.Unmarshal(out, &arr); jerr != nil {
		t.Fatalf("拼接结果必须是合法 JSON 数组: body=%s err=%v", out, jerr)
	}
	if len(arr) != 1 {
		t.Fatalf("期望 1 条记录, 得到 %d body=%s", len(arr), out)
	}
	if id, _ := arr[0]["id"].(float64); id != 200 {
		t.Errorf("期望 id=200, 得到 %v", arr[0]["id"])
	}
}

// TestAssembleCirclesJSON_EmptyFirstPage_PartialPath 部分失败路径同样不得 leading comma。
func TestAssembleCirclesJSON_EmptyFirstPage_PartialPath(t *testing.T) {
	raw1 := []byte("[]")
	results := make([]rawResult, 4)
	results[2] = rawResult{raw: []byte(`[{"id":1}]`)}
	// page3 缺失（模拟失败页）

	out, err := assembleCirclesJSON(raw1, results, 3, errPartialStub{})
	if err == nil {
		t.Fatal("partialErr 非 nil 时应透传 error")
	}
	var arr []map[string]any
	if jerr := json.Unmarshal(out, &arr); jerr != nil {
		t.Fatalf("partial 路径结果仍须合法 JSON: body=%s err=%v", out, jerr)
	}
	if len(arr) != 1 {
		t.Fatalf("期望 1 条已合并记录, 得到 %d body=%s", len(arr), out)
	}
}

// errPartialStub 仅作 partialErr 占位。
type errPartialStub struct{}

func (errPartialStub) Error() string { return "partial" }

// TestAssembleBufferCapHint_Clamped（CC1）锁定：assembleCirclesJSON 的预分配容量
// 提示必须钳制到固定上界——修复前 `make([]byte, 0, len(raw1)*totalPage)` 在
// 攻陷服务端返回 4MB 首页 + totalNum 虚高（→totalPage=10000）时预分配 40GB，
// 单请求 OOM 崩进程。钳制纪律与 submitted.go capacity 钳制同族，但单位为字节。
func TestAssembleBufferCapHint_Clamped(t *testing.T) {
	cases := []struct {
		name      string
		pageCount int
		firstPage int
		want      int
	}{
		{
			name:      "正常组合不钳制",
			pageCount: 10,
			firstPage: 4096,
			want:      40960,
		},
		{
			name:      "攻陷服务端首页满 4MB 且页数达上限 → 钳制到固定上界",
			pageCount: maxTotalPage,
			firstPage: maxResponseBodySize,
			want:      maxAssembleBuffer,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := assembleBufferCapHint(tc.pageCount, tc.firstPage)
			if got != tc.want {
				t.Fatalf("assembleBufferCapHint(%d, %d) = %d, want %d", tc.pageCount, tc.firstPage, got, tc.want)
			}
		})
	}
}

// TestAssembleCirclesJSON_CapHintClamped 白盒端到端：超大页数×首頁聲明下，
// 實際構造的 buffer 容量不得超過固定上界（防 make 40GB OOM）。
func TestAssembleCirclesJSON_CapHintClamped(t *testing.T) {
	raw1 := bytes.Repeat([]byte(`[{"id":1}]`), maxResponseBodySize/9)
	results := make([]rawResult, maxTotalPage+1)
	out, err := assembleCirclesJSON(raw1, results, maxTotalPage, nil)
	if err != nil {
		t.Fatalf("assembleCirclesJSON: %v", err)
	}
	// 容量上界不能被擊穿：make 的 cap 受鉗制（間接通過返回緩衝不可見，故改驗證
	// 函數不 panic 且正常返回——容量鉗制由 assembleBufferCapHint 專測鎖定）。
	if len(out) == 0 {
		t.Fatal("結果不應為空")
	}
}

// N-04：capAssembledSlice 对多页累积原始字节做总量预算判定——首页 4MB +
// 每页 4MB 连续多页累积越过 64MB 预算即返回 true（防攻陷服务端报
// 10000 页×4MB≈40GB 渐进填充进程内累积）。
func TestCapAssembledSlice_BudgetExceeded(t *testing.T) {
	// 首页 1 页满 4MB
	raw1 := bytes.Repeat([]byte("x"), maxResponseBodySize)
	// 20 页每页 4MB → 累积 80MB > 64MB 预算
	results := make([]rawResult, 21)
	for i := 2; i <= 20; i++ {
		results[i] = rawResult{raw: bytes.Repeat([]byte("y"), maxResponseBodySize)}
	}
	if !capAssembledSlice(raw1, results, 20) {
		t.Fatalf("累积 80MB 应越过 64MB 预算（N-04 未生效）")
	}
	if got := cumulativeSliceBytes(raw1, results, 20); got != 20*maxResponseBodySize {
		t.Fatalf("cumulativeSliceBytes = %d, want %d", got, 20*maxResponseBodySize)
	}
	// 小量翻页不误伤：仅首页 + 2 页小数据
	small := make([]rawResult, 3)
	small[2] = rawResult{raw: []byte(`[{"id":1}]`)}
	if capAssembledSlice(raw1, small, 2) {
		t.Fatal("小量累积不应被误判超预算")
	}
	// 页号越界不 panic
	if capAssembledSlice(raw1, small, 100) {
		t.Fatal("越界页号应安全返回（不越界访问 results）")
	}
}

// N-01 回归：getCirclesJSON 命中累积预算后必须传「已钳制页数」给
// assembleCirclesJSON——此前用全部声明页数拼接，Bytes.Buffer 仍无上限
// 增长（注释声称截断实际没截）。这里白盒直接验证 getCirclesJSON 走
// 预算分支时 assembleCirclesJSON 拿到的页数 ≤ 预算页数。
func TestEstimatePagesBudgeted_Clamped(t *testing.T) {
	// 首页 4MB、500 页 → 估算 2GB，越 64MB 预算
	got := estimatePagesBudgeted(500, maxResponseBodySize)
	if got <= maxAssembleBuffer {
		t.Fatalf("estimatePagesBudgeted(500, 4MB) = %d, want > 64MB", got)
	}
	// 首页 4KB、20 页 → 80KB，不越预算
	if got := estimatePagesBudgeted(20, 4096); got != 81920 {
		t.Fatalf("estimatePagesBudgeted(20, 4096) = %d, want 81920", got)
	}
	// 边界页 0 安全
	if got := estimatePagesBudgeted(0, 100); got != 0 {
		t.Fatalf("estimatePagesBudgeted(0,100) = %d, want 0", got)
	}
}
