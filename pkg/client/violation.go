package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// GetViolationList 分页查询当前学生的违规违纪记录。
//
// 对应前端 performanceM.vue 的 searchViolation：
// GET /api/studentMoralEduNew/getViolation?pageNo=&pageSize=&key=
func (c *Client) GetViolationList(ctx context.Context, token string, pageNo, pageSize int, key string) (*types.ViolationListResult, error) {
	path := violationListPath(pageNo, pageSize, key)
	resp, err := c.doBizAndDecode(ctx, token, "GetViolationList", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetViolationList 失败: %w", err)
	}

	records, err := types.DecodeDataList[types.ViolationRecord](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetViolationList 解析违规记录失败: %w", err)
	}
	page, err := types.DecodePageBean(*resp)
	if err != nil {
		return nil, fmt.Errorf("GetViolationList 解析分页信息失败: %w", err)
	}
	if records == nil {
		records = []types.ViolationRecord{}
	}
	return &types.ViolationListResult{Records: records, Page: page}, nil
}

// GetViolationListJSON 获取指定页违规记录的原始 JSON。
//
// 返回 {"records": [...], "page": {...}}，records 中的平台字段不经过结构化裁剪，
// 供 CLI 和需要保留未知字段的调用方使用。
func (c *Client) GetViolationListJSON(ctx context.Context, token string, pageNo, pageSize int, key string) (json.RawMessage, error) {
	resp, err := c.doBizAndDecode(ctx, token, "GetViolationListJSON", violationListPath(pageNo, pageSize, key), http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetViolationListJSON 失败: %w", err)
	}
	return assembleRecordsPageJSON(resp), nil
}

// GetViolationTypes 获取前端违纪说明使用的违规事由类型。
//
// 对应前端 performanceM.vue 的 searchViolationType：
// GET /api/studentMoralEduNew/getViolationType，主数据在 dataList。
func (c *Client) GetViolationTypes(ctx context.Context, token string) ([]types.ViolationType, error) {
	resp, err := c.doBizAndDecode(ctx, token, "GetViolationTypes", "/api/studentMoralEduNew/getViolationType", http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetViolationTypes 失败: %w", err)
	}

	items, err := types.DecodeDataList[types.ViolationType](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetViolationTypes 解析违规类型失败: %w", err)
	}
	if items != nil {
		return items, nil
	}

	var fallback []types.ViolationType
	if err := types.DecodeReturnDataSlice(*resp, &fallback); err != nil {
		return nil, fmt.Errorf("GetViolationTypes 解析 returnData 失败: %w", err)
	}
	if fallback == nil {
		fallback = []types.ViolationType{}
	}
	return fallback, nil
}

func violationListPath(pageNo, pageSize int, key string) string {
	return "/api/studentMoralEduNew/getViolation?pageNo=" + strconv.Itoa(pageNo) +
		"&pageSize=" + strconv.Itoa(pageSize) + "&key=" + url.QueryEscape(key)
}
