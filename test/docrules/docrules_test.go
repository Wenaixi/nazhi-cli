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
// CLI 文档校验（docs/cli/README.md）
// ────────────────────────────────────────────────────────────────────

func TestCLI_WhoamiEmptyStatus(t *testing.T) {
	root := repoRoot(t)
	cli := mustRead(t, filepath.Join(root, "docs/cli/README.md"))
	// whoami 空数据响应的 status 必须是 "success"（envelope.Empty 用 StatusSuccess）
	mustGrep(t, "whoami 空数据 envelope",
		cli,
		`"status":"success","code":204,"message":"get_my_info_empty"`)
}

func TestCLI_TaskSubmitPayloadFieldCount(t *testing.T) {
	root := repoRoot(t)
	cli := mustRead(t, filepath.Join(root, "docs/cli/README.md"))
	// 文档应明确说是 30 字段，不能残留 29
	mustNotGrep(t, "task submit payload 字段数",
		cli, "29 字段 addCircle")
	mustNotGrep(t, "task submit payload 字段数",
		cli, "完整 29 字段定义")
	mustGrep(t, "task submit payload 字段数",
		cli, "30 字段")
}

func TestCLI_FileDownloadResponseKey(t *testing.T) {
	root := repoRoot(t)
	cli := mustRead(t, filepath.Join(root, "docs/cli/README.md"))
	// file download 响应示例的 data 必须是 "output" 键（file_download.go:66 用 key "output"）
	mustNotGrep(t, "file download 响应键名",
		cli, `"data": { "id": 12345, "path": "./photo.jpg" }`)
	mustGrep(t, "file download 响应键名",
		cli, `"output": "./photo.jpg"`)
}

// ────────────────────────────────────────────────────────────────────
// SDK 文档校验（docs/sdk/README.md）
// ────────────────────────────────────────────────────────────────────

func TestSDK_TaskSubmitPayloadFieldCount(t *testing.T) {
	root := repoRoot(t)
	sdk := mustRead(t, filepath.Join(root, "docs/sdk/README.md"))
	// 字段数应为 30（实际 TaskSubmitPayload 有 30 个 JSON tag 字段）
	mustNotGrep(t, "TaskSubmitPayload 字段数",
		sdk, "29 字段 addCircle")
	mustNotGrep(t, "TaskSubmitPayload 字段数",
		sdk, "29 字段透传")
	mustGrep(t, "TaskSubmitPayload 字段数",
		sdk, "30 字段")
}

func TestSDK_DateOnlyFormat(t *testing.T) {
	root := repoRoot(t)
	sdk := mustRead(t, filepath.Join(root, "docs/sdk/README.md"))
	// Task JSON 示例中 DateOnly 字段（creationTimeStr / startDateStr / endDateStr 等）
	// 服务端实际发送 YYYY-MM-DD，不是 RFC3339。文档示例中
	// "creationTimeStr": "2026-06-30T00:00:00+08:00" 这种写法是错的。
	mustNotGrep(t, "DateOnly 示例 RFC3339",
		sdk, `"creationTimeStr": "2026-06-30T00:00:00+08:00"`)
	mustNotGrep(t, "DateOnly 示例 RFC3339",
		sdk, `"startDateStr": "2026-06-30T00:00:00+08:00"`)
	// 应当有 YYYY-MM-DD 格式的 DateOnly 示例（字段参考表里已经正确）
	mustGrep(t, "DateOnly YYYY-MM-DD 格式示例",
		sdk, `2026-01-12`)
}

func TestSDK_SentinelListComplete(t *testing.T) {
	root := repoRoot(t)
	sdk := mustRead(t, filepath.Join(root, "docs/sdk/README.md"))
	// 哨兵错误一览主表应包含 ErrImageTooLarge / ErrUnsupportedFormat（image_prep.go 定义）
	mustGrep(t, "ErrImageTooLarge 在哨兵表中",
		sdk, "ErrImageTooLarge")
	mustGrep(t, "ErrUnsupportedFormat 在哨兵表中",
		sdk, "ErrUnsupportedFormat")
}

func TestSDK_HonorTypeOverviewNoGhostFields(t *testing.T) {
	root := repoRoot(t)
	sdk := mustRead(t, filepath.Join(root, "docs/sdk/README.md"))
	idx := extractTypeIndex(t, sdk)
	// HonorType 实际只有 5 字段（ID / Name / LevelName / Level / DimensionName）
	// 概览不应出现 SortNo（已删）；不应把 Score 列进去（HonorType 没有 Score）
	line := extractTypeRow(t, idx, "HonorType")
	if strings.Contains(line, "SortNo") {
		t.Errorf("[HonorType 概览] 幽灵字段 SortNo 应删除（实际 5 字段），当前行：%q", line)
	}
	if strings.Contains(line, "Score") {
		t.Errorf("[HonorType 概览] 幽灵字段 Score 应删除（HonorType 没有 Score 字段），当前行：%q", line)
	}
}

func TestSDK_HonorRecordOverviewAccurate(t *testing.T) {
	root := repoRoot(t)
	sdk := mustRead(t, filepath.Join(root, "docs/sdk/README.md"))
	idx := extractTypeIndex(t, sdk)
	// HonorRecord 实际 9 字段（ID / TypeName / LevelName / Level / DimensionName / Approved / ApprovedName / GetDate / EvaluationAgency）
	line := extractTypeRow(t, idx, "HonorRecord")
	if strings.Contains(line, "15 字段") {
		t.Errorf("[HonorRecord 概览] 应为 9 字段，不应写 15 字段，当前行：%q", line)
	}
	for _, ghost := range []string{"TypeID", "Score", "Status", "StatusName"} {
		if strings.Contains(line, ghost) {
			t.Errorf("[HonorRecord 概览] 幽灵字段 %s 应删除，当前行：%q", ghost, line)
		}
	}
}

func TestSDK_CircleRecordOverviewAccurate(t *testing.T) {
	root := repoRoot(t)
	sdk := mustRead(t, filepath.Join(root, "docs/sdk/README.md"))
	idx := extractTypeIndex(t, sdk)
	// CircleRecord 实际字段：ID / Name / TypeName / Content / Approved / CircleDate / Hours / ImgList / ImgPreViewList / Remark
	// 概览不应写 Status（实际是 Approved bool）
	line := extractTypeRow(t, idx, "CircleRecord")
	if strings.Contains(line, "Status") {
		t.Errorf("[CircleRecord 概览] 概览行含 Status，应改为 Approved（bool），当前行：%q", line)
	}
}

func TestSDK_CircleImageOverviewAccurate(t *testing.T) {
	root := repoRoot(t)
	sdk := mustRead(t, filepath.Join(root, "docs/sdk/README.md"))
	idx := extractTypeIndex(t, sdk)
	// CircleImage 实际 6 字段（ID / CircleID / ClassID / TaskID / AttachmentID / ImgPath），
	// 概览不应只写 AttachmentID
	line := extractTypeRow(t, idx, "CircleImage")
	// 至少应出现 4 个字段名（除 AttachmentID 外还有 ID / CircleID / ClassID / TaskID / ImgPath）
	fieldCount := 0
	for _, f := range []string{"ID", "CircleID", "ClassID", "TaskID", "AttachmentID", "ImgPath"} {
		if strings.Contains(line, f) {
			fieldCount++
		}
	}
	if fieldCount < 4 {
		t.Errorf("[CircleImage 概览] 字段数过少（%d 个），实际 6 字段，当前行：%q", fieldCount, line)
	}
}

func TestSDK_TaskSubmitPayloadFieldTableHasTags(t *testing.T) {
	root := repoRoot(t)
	sdk := mustRead(t, filepath.Join(root, "docs/sdk/README.md"))
	// 字段参考表中 14 个字段的 JSON tag 列被写成 "—"，应补全真实 tag
	// 抽样几个关键字段
	for _, tag := range []string{
		`circleBeginDate`,
		`circleEndDate`,
		`checkResult`,
		`patentType`,
		`patentNum`,
		`address`,
		`termName`,
		`activityName`,
		`sportsName`,
		`teamName`,
		`orgName`,
		`resultsName`,
		`obtainTime`,
		`specialtyTechnology`,
	} {
		mustGrep(t, "TaskSubmitPayload JSON tag "+tag,
			sdk, "`"+tag+"`")
	}
}

// extractTypeIndex 提取「pkg/types 类型索引」表格（被多次复用）。
func extractTypeIndex(t *testing.T, sdk string) string {
	t.Helper()
	const start = "## pkg/types 类型索引"
	const end = "### pkg/types/response.go"
	i := strings.Index(sdk, start)
	if i < 0 {
		t.Fatalf("找不到 %q", start)
	}
	j := strings.Index(sdk, end)
	if j < 0 {
		t.Fatalf("找不到 %q", end)
	}
	return sdk[i:j]
}

// extractTypeRow 提取类型索引表里指定类型的整行（包含 Markdown 表格的 `|` 分隔符）。
func extractTypeRow(t *testing.T, indexSection, typeName string) string {
	t.Helper()
	needle := "| `" + typeName + "` |"
	i := strings.Index(indexSection, needle)
	if i < 0 {
		t.Fatalf("在类型索引表里找不到 %s 行", typeName)
	}
	// 取从这一行开始到下一个 \n 的整段
	j := strings.Index(indexSection[i:], "\n")
	if j < 0 {
		return indexSection[i:]
	}
	return indexSection[i : i+j]
}