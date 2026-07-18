package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// GetViolationList 分页查询违规记录。
//
// 对应前端：performanceM.vue → searchViolation
// API: GET /api/studentMoralEduNew/getViolation?pageNo={pageNo}&pageSize={pageSize}&key={key}
func (c *Client) GetViolationList(ctx context.Context, token string, pageNo, pageSize int, key string) ([]types.ViolationRecord, *types.PageBean, error) {
	path := "/api/studentMoralEduNew/getViolation?pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(pageSize) + "&key=" + key
	resp, err := c.doBizAndDecode(ctx, token, "GetViolationList", path, http.MethodGet, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("GetViolationList 失败: %w", err)
	}

	pb, err := types.DecodePageBean(*resp)
	if err != nil {
		return nil, nil, fmt.Errorf("GetViolationList 解析分页信息失败: %w", err)
	}

	records, err := types.DecodeDataList[types.ViolationRecord](*resp)
	if err != nil {
		return nil, nil, fmt.Errorf("GetViolationList 解析违规记录失败: %w", err)
	}

	return records, pb, nil
}

// GetViolationTypes 查询违规事由。
//
// 对应前端：performanceM.vue → searchViolationType
// API: GET /api/studentMoralEduNew/getViolationType
func (c *Client) GetViolationTypes(ctx context.Context, token string) ([]types.ViolationType, error) {
	resp, err := c.doBizAndDecode(ctx, token, "GetViolationTypes", "/api/studentMoralEduNew/getViolationType", http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetViolationTypes 失败: %w", err)
	}

	typesList, err := types.DecodeDataList[types.ViolationType](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetViolationTypes 解析违规类型失败: %w", err)
	}

	return typesList, nil
}

// UpdateHonor 更新荣誉记录。
//
// 对应前端：performanceM.vue → submit2
// API: POST /api/studentMoralEduNew/updateHonor
func (c *Client) UpdateHonor(ctx context.Context, token string, payload types.AddHonorPayload) error {
	_, err := c.doBizAndDecode(ctx, token, "UpdateHonor", "/api/studentMoralEduNew/updateHonor", http.MethodPost, payload)
	if err != nil {
		return fmt.Errorf("UpdateHonor 失败: %w", err)
	}
	return nil
}
