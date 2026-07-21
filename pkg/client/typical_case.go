package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// AddTypicalCase 提交一条典型案例。
//
// 遵循 AddHonor 模式：doBizVoid POST → 成功返回 nil。
func (c *Client) AddTypicalCase(ctx context.Context, token string, payload types.AddTypicalCasePayload) error {
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
func (c *Client) UpdateTypicalCase(ctx context.Context, token string, payload map[string]any) error {
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
