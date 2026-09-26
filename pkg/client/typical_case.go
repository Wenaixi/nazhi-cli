package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// 典型案例下拉展示名（对齐 classiccanter.vue el-option label）。
// 用户只选 code；SDK 在 *Name 为空时自动补全。
// type=2 为「社会调查报告」（非「社会实践报告」）；level=1 为「国际」（非写实列表的「国家」）。
var (
	typicalCaseTypeNames = map[string]string{
		"1": "研究性学习报告",
		"2": "社会调查报告",
		"3": "艺术创作作品",
		"4": "其他",
	}
	typicalCaseRoleNames = map[string]string{
		"1": "负责人", // types.TypicalCaseRoleHost
		"2": "参与者", // types.TypicalCaseRoleParticipant
	}
	typicalCaseLevelNames = map[string]string{
		"1": "国际",
		"2": "省",
		"3": "市",
		"4": "区县",
		"5": "学校",
	}
)

// fillTypicalCaseDisplayNames 在 TypeName/RoleName/LevelName 为空时按 code 填展示名。
// 已有非空 *Name 不覆盖，便于调用方自定义文案。
func fillTypicalCaseDisplayNames(p *types.AddTypicalCasePayload) {
	if p == nil {
		return
	}
	if p.TypeName == "" {
		if n, ok := typicalCaseTypeNames[p.Type]; ok {
			p.TypeName = n
		}
	}
	if p.RoleName == "" {
		if n, ok := typicalCaseRoleNames[p.Role]; ok {
			p.RoleName = n
		}
	}
	if p.LevelName == "" {
		if n, ok := typicalCaseLevelNames[p.Level]; ok {
			p.LevelName = n
		}
	}
}

// typicalCaseCodeString 把 type/role/level 统一成映射表用的字符串代码。
// 列表回填常见 number（int/float64），新增表单为 string；均需可识别。
func typicalCaseCodeString(v any) (string, bool) {
	switch n := v.(type) {
	case string:
		if n == "" {
			return "", false
		}
		return n, true
	case int:
		return strconv.Itoa(n), true
	case int64:
		return strconv.FormatInt(n, 10), true
	case float64:
		// JSON 数字默认 float64；仅接受整数值，避免 2.5 误映射
		if n != float64(int64(n)) {
			return "", false
		}
		return strconv.FormatInt(int64(n), 10), true
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return "", false
		}
		return strconv.FormatInt(i, 10), true
	default:
		return "", false
	}
}

// fillTypicalCaseDisplayNamesMap 更新路径：map 含 type/role/level 且对应 *Name 缺失时补全。
// code 支持 string 与 number（对齐 getTypicalCase 列表响应 + 手填 string）。
func fillTypicalCaseDisplayNamesMap(payload map[string]any) {
	if payload == nil {
		return
	}
	if typeName, _ := payload["typeName"].(string); typeName == "" {
		if code, ok := typicalCaseCodeString(payload["type"]); ok {
			if n, ok := typicalCaseTypeNames[code]; ok {
				payload["typeName"] = n
			}
		}
	}
	if roleName, _ := payload["roleName"].(string); roleName == "" {
		if code, ok := typicalCaseCodeString(payload["role"]); ok {
			if n, ok := typicalCaseRoleNames[code]; ok {
				payload["roleName"] = n
			}
		}
	}
	if levelName, _ := payload["levelName"].(string); levelName == "" {
		if code, ok := typicalCaseCodeString(payload["level"]); ok {
			if n, ok := typicalCaseLevelNames[code]; ok {
				payload["levelName"] = n
			}
		}
	}
}

// 典型案例 remark/content 的字数上限，对齐前端 el-input maxlength=
// classiccanter.vue:124 maxlength="198"（备注）、:130 maxlength="1500"（正文）。
// 浏览器硬截断保证线上恒发 ≤上限；SDK 不静默截断也不放行超长原文——
// 显式拒绝（ErrInvalidPayload），与 task content 的 maxTaskContentRunes 纪律同族。
const (
	maxTypicalCaseRemarkRunes  = 198
	maxTypicalCaseContentRunes = 1500
)

// validateTypicalCaseLengths 校验典型案例 remark/content 的 rune 长度上限。
// 超长即返回 ErrInvalidPayload（调用方输入问题 → CLI 漏斗 400/exit3），不发业务请求。
func validateTypicalCaseLengths(payload *types.AddTypicalCasePayload) error {
	if payload == nil {
		return fmt.Errorf("%w: 典型案例 payload 为空", ErrInvalidPayload)
	}
	if len([]rune(payload.Remark)) > maxTypicalCaseRemarkRunes {
		return fmt.Errorf("%w: remark 超过 %d 字上限（收到 %d 字）",
			ErrInvalidPayload, maxTypicalCaseRemarkRunes, len([]rune(payload.Remark)))
	}
	if len([]rune(payload.Content)) > maxTypicalCaseContentRunes {
		return fmt.Errorf("%w: content 超过 %d 字上限（收到 %d 字）",
			ErrInvalidPayload, maxTypicalCaseContentRunes, len([]rune(payload.Content)))
	}
	return nil
}

// validateTypicalCaseLengthsMap 校验典型案例更新路径（map）的 remark/content
// rune 上限。与 validateTypicalCaseLengths（Add 路径）同族：超长即
// ErrInvalidPayload，不发业务请求。：此前 Update 无校验、Add 有，
// 长度纪律不对称。
func validateTypicalCaseLengthsMap(payload map[string]any) error {
	if len([]rune(firstStringFromMap(payload, "remark"))) > maxTypicalCaseRemarkRunes {
		return fmt.Errorf("%w: remark 超过 %d 字上限（收到 %d 字）",
			ErrInvalidPayload, maxTypicalCaseRemarkRunes, len([]rune(firstStringFromMap(payload, "remark"))))
	}
	if len([]rune(firstStringFromMap(payload, "content"))) > maxTypicalCaseContentRunes {
		return fmt.Errorf("%w: content 超过 %d 字上限（收到 %d 字）",
			ErrInvalidPayload, maxTypicalCaseContentRunes, len([]rune(firstStringFromMap(payload, "content"))))
	}
	return nil
}

// firstStringFromMap 读取 map 中首个非空字符串键值（兼容 string/[]byte/数字）。
func firstStringFromMap(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return ""
	}
}

// AddTypicalCase 提交一条典型案例。
//
// 用户只需填标题/类别代码/角色代码/级别代码/指导教师等；
// TypeName/RoleName/LevelName 为空时按前端下拉自动补全。
// 遵循 AddHonor 模式：doBizVoid POST → 成功返回 nil。
func (c *Client) AddTypicalCase(ctx context.Context, token string, payload types.AddTypicalCasePayload) error {
	// ：remark/content 长度校验必须先于任何请求——前端 maxlength 截断保证
	// 线上恒发 ≤上限（198/1500 字），SDK 对超长原文显式拒绝（ErrInvalidPayload）。
	if err := validateTypicalCaseLengths(&payload); err != nil {
		return err
	}
	fillTypicalCaseDisplayNames(&payload)
	return c.doBizVoid(ctx, token, "AddTypicalCase",
		"/api/studentCircleNew/addTypicalCase", http.MethodPost, payload)
}

// 典型案例审核状态（与前端 classiccanter.vue 下拉一致）。
const (
	TypicalCaseStatusPending  = 0 // 未审核
	TypicalCaseStatusApproved = 1 // 通过
	TypicalCaseStatusRejected = 2 // 驳回
	TypicalCaseStatusAll      = 3 // 全部（前端默认）
)

// GetTypicalCaseList 查询典型案例列表（分页）。
//
// status 为可选变参：不传时默认 3（全部），与前端默认一致。
// 取值：0 未审 / 1 通过 / 2 驳回 / 3 全部。
//
// 签名：GetTypicalCaseList(ctx, token, pageNo, pageSize, status...int)
// 多传 status 时仅用第一个。
func (c *Client) GetTypicalCaseList(ctx context.Context, token string, pageNo, pageSize int, status ...int) (*types.TypicalCaseListResult, error) {
	st := TypicalCaseStatusAll
	if len(status) > 0 {
		st = status[0]
	}
	path := "/api/studentCircleNew/getTypicalCase?pageNo=" + strconv.Itoa(pageNo) +
		"&pageSize=" + strconv.Itoa(pageSize) + "&status=" + strconv.Itoa(st)

	resp, err := c.doBizAndDecode(ctx, token, "GetTypicalCaseList", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetTypicalCaseList 失败: %w", err)
	}

	pb, err := types.DecodePageBean(*resp)
	if err != nil {
		return nil, fmt.Errorf("GetTypicalCaseList 解析分页信息失败: %w", err)
	}

	records, err := types.DecodeDataList[types.TypicalCaseRecord](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetTypicalCaseList 解析记录失败: %w", err)
	}

	return &types.TypicalCaseListResult{Records: records, Page: pb}, nil
}

// GetTypicalCaseListJSON 返回典型案例列表的原始 JSON（CLI 1:1 对齐）。
//
// status 变参语义同 GetTypicalCaseList：默认 3（全部）。
// 拼装 {"records":..., "page":...}，records 和 page 均为平台原始字节。
func (c *Client) GetTypicalCaseListJSON(ctx context.Context, token string, pageNo, pageSize int, status ...int) (json.RawMessage, error) {
	st := TypicalCaseStatusAll
	if len(status) > 0 {
		st = status[0]
	}
	path := "/api/studentCircleNew/getTypicalCase?pageNo=" + strconv.Itoa(pageNo) +
		"&pageSize=" + strconv.Itoa(pageSize) + "&status=" + strconv.Itoa(st)

	resp, err := c.doBizAndDecode(ctx, token, "GetTypicalCaseListJSON", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetTypicalCaseListJSON 失败: %w", err)
	}
	return assembleRecordsPageJSON(resp), nil
}

// UpdateTypicalCase 更新一条典型案例。
// POST /api/studentCircleNew/updateTypicalCase
//
// 与 AddTypicalCase 对称：type/role/level 有值且对应 *Name 为空时自动补展示名。
// remark/content 长度校验与 Add 同族（198/1500 rune，ErrInvalidPayload）。
func (c *Client) UpdateTypicalCase(ctx context.Context, token string, payload map[string]any) error {
	if err := validateTypicalCaseLengthsMap(payload); err != nil {
		return err
	}
	fillTypicalCaseDisplayNamesMap(payload)
	return c.doBizVoid(ctx, token, "UpdateTypicalCase",
		"/api/studentCircleNew/updateTypicalCase", http.MethodPost, payload)
}

// DeleteTypicalCase 删除一条典型案例。
// GET /api/studentCircleNew/deleteTypicalCase?id=
func (c *Client) DeleteTypicalCase(ctx context.Context, token string, id int64) error {
	path := "/api/studentCircleNew/deleteTypicalCase?id=" + strconv.FormatInt(id, 10)
	return c.doBizVoid(ctx, token, "DeleteTypicalCase", path, http.MethodGet, nil)
}

// DeleteBatchTypicalCase 批量删除典型案例。
// POST /api/studentCircleNew/deleteBatchTypicalCase
//
// 请求体是纯 JSON 数组 [1, 2, 3]（前端源码确认）。
// 空/nil 切片返回 ErrInvalidPayload 而非发出字面 null 请求体
// （对齐前端 classiccanter.vue 的空数组守卫与 CLI 层参数校验口径）。
func (c *Client) DeleteBatchTypicalCase(ctx context.Context, token string, ids []int64) error {
	if len(ids) == 0 {
		return fmt.Errorf("%w: 批量删除需要至少一个 id", ErrInvalidPayload)
	}
	return c.doBizVoid(ctx, token, "DeleteBatchTypicalCase",
		"/api/studentCircleNew/deleteBatchTypicalCase", http.MethodPost, ids)
}
