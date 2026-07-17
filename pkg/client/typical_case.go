package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// AddTypicalCase 提交一条典型案例。
//
// 遵循 AddHonor 模式：doBizAndDecode POST → 成功返回 nil。
func (c *Client) AddTypicalCase(ctx context.Context, token string, payload types.AddTypicalCasePayload) error {
	_, err := c.doBizAndDecode(ctx, token, "AddTypicalCase",
		"/api/studentCircleNew/addTypicalCase", http.MethodPost, payload)
	if err != nil {
		return fmt.Errorf("AddTypicalCase 失败: %w", err)
	}
	return nil
}

// GetTypicalCaseList 查询已提交典型案例列表（分页）。
//
// 遵循 GetHonorList 模式：path 拼参数 → doBizAndDecode → 解析 dataList + pageBean。
// status=3 表示"已提交"状态的记录（HAR 确认）。
func (c *Client) GetTypicalCaseList(ctx context.Context, token string, pageNo, pageSize int) (*types.TypicalCaseListResult, error) {
	path := "/api/studentCircleNew/getTypicalCase?pageNo=" + strconv.Itoa(pageNo) +
		"&pageSize=" + strconv.Itoa(pageSize) + "&status=3"

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

// GetTypicalCaseListJSON 返回已提交典型案例列表的原始 JSON（CLI 1:1 对齐）。
//
// 遵循 GetHonorListJSON 模式：拼装 {"records":..., "page":...}，
// records 和 page 字段值都是平台原始字节。
func (c *Client) GetTypicalCaseListJSON(ctx context.Context, token string, pageNo, pageSize int) (json.RawMessage, error) {
	path := "/api/studentCircleNew/getTypicalCase?pageNo=" + strconv.Itoa(pageNo) +
		"&pageSize=" + strconv.Itoa(pageSize) + "&status=3"

	resp, err := c.doBizAndDecode(ctx, token, "GetTypicalCaseListJSON", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetTypicalCaseListJSON 失败: %w", err)
	}

	var recordsRaw json.RawMessage
	if resp.DataList != nil {
		recordsRaw = *resp.DataList
	}
	var pageBeanRaw json.RawMessage
	if resp.PageBean != nil {
		pageBeanRaw = *resp.PageBean
	}

	if len(recordsRaw) == 0 {
		recordsRaw = json.RawMessage("[]")
	}

	var buf bytes.Buffer
	buf.WriteString(`{"records":`)
	buf.Write(recordsRaw)
	if len(pageBeanRaw) > 0 {
		buf.WriteString(`,"page":`)
		buf.Write(pageBeanRaw)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
