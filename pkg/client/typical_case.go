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
var (
	typicalCaseTypeNames = map[string]string{
		"1": "研究性学习报告",
		"2": "社会实践报告",
		"3": "艺术创作作品",
		"4": "其他",
	}
	typicalCaseRoleNames = map[string]string{
		"1": "负责人", // types.TypicalCaseRoleHost
		"2": "参与者", // types.TypicalCaseRoleParticipant
	}
	typicalCaseLevelNames = map[string]string{
		"1": "国家",
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

// fillTypicalCaseDisplayNamesMap 更新路径：map 含 type/role/level 且对应 *Name 缺失时补全。
func fillTypicalCaseDisplayNamesMap(payload map[string]any) {
	if payload == nil {
		return
	}
	if typeName, _ := payload["typeName"].(string); typeName == "" {
		if code, ok := payload["type"].(string); ok {
			if n, ok := typicalCaseTypeNames[code]; ok {
				payload["typeName"] = n
			}
		}
	}
	if roleName, _ := payload["roleName"].(string); roleName == "" {
		if code, ok := payload["role"].(string); ok {
			if n, ok := typicalCaseRoleNames[code]; ok {
				payload["roleName"] = n
			}
		}
	}
	if levelName, _ := payload["levelName"].(string); levelName == "" {
		if code, ok := payload["level"].(string); ok {
			if n, ok := typicalCaseLevelNames[code]; ok {
				payload["levelName"] = n
			}
		}
	}
}

// AddTypicalCase 提交一条典型案例。
//
// 用户只需填标题/类别代码/角色代码/级别代码/指导教师等；
// TypeName/RoleName/LevelName 为空时按前端下拉自动补全。
// 遵循 AddHonor 模式：doBizVoid POST → 成功返回 nil。
func (c *Client) AddTypicalCase(ctx context.Context, token string, payload types.AddTypicalCasePayload) error {
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
func (c *Client) UpdateTypicalCase(ctx context.Context, token string, payload map[string]any) error {
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
func (c *Client) DeleteBatchTypicalCase(ctx context.Context, token string, ids []int64) error {
	return c.doBizVoid(ctx, token, "DeleteBatchTypicalCase",
		"/api/studentCircleNew/deleteBatchTypicalCase", http.MethodPost, ids)
}
