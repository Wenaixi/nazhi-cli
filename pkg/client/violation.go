package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// GetViolationList 获取违规记录列表（分页）。
// GET /api/studentMoralEduNew/getViolation?pageNo=&pageSize=&key=
func (c *Client) GetViolationList(ctx context.Context, token string, pageNo, pageSize int, key string) (*types.ViolationListResult, error) {
	path := "/api/studentMoralEduNew/getViolation?pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(pageSize) + "&key=" + key
	resp, err := c.doBizAndDecode(ctx, token, "GetViolationList", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetViolationList 失败: %w", err)
	}
	pb, err := types.DecodePageBean(*resp)
	if err != nil {
		return nil, fmt.Errorf("GetViolationList 解析分页信息失败: %w", err)
	}
	records, err := types.DecodeDataList[types.ViolationRecord](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetViolationList 解析违规记录失败: %w", err)
	}
	return &types.ViolationListResult{Records: records, Page: pb}, nil
}

// GetViolationTypes 获取违规类型列表。
// GET /api/studentMoralEduNew/getViolationType
func (c *Client) GetViolationTypes(ctx context.Context, token string) ([]types.ViolationType, error) {
	resp, err := c.doBizAndDecode(ctx, token, "GetViolationTypes", "/api/studentMoralEduNew/getViolationType", http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetViolationTypes 失败: %w", err)
	}
	return types.DecodeDataList[types.ViolationType](*resp)
}
