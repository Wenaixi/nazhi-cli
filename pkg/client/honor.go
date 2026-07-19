// honor.go 荣誉申报 SDK。
// 端点映射：
//
//	GET  /api/studentMoralEduNew/getHonorType          — 获取所有荣誉类型（dataList / returnData 双通道）
//	GET  /api/studentMoralEduNew/getHonorTypeForSelect  — 获取级别下拉选项（returnData）
//	GET  /api/studentMoralEduNew/getHonorLevel          — 获取荣誉级别（dataList，需 honorTypeId 参数）
//	GET  /api/studentMoralEduNew/getHonorByStudentId    — 查询已有荣誉（分页，需 &key=）
//	POST /api/studentMoralEduNew/addHonor               — 申报荣誉
//	POST /api/studentMoralEduNew/deleteHonorById        — 删除荣誉
package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// GetHonorTypes 获取所有可申报的荣誉类型。
//
// 实测服务端同时支持 dataList（丰富字段）和 returnData（兼容字段）两条路径，
// 本方法优先解析 dataList，fallback 到 returnData。
func (c *Client) GetHonorTypes(ctx context.Context, token string) ([]types.HonorType, error) {
	resp, err := c.doBizAndDecode(ctx, token, "GetHonorTypes", "/api/studentMoralEduNew/getHonorType", http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetHonorTypes 失败: %w", err)
	}

	// 优先 dataList（丰富字段），fallback 到 returnData
	if resp.DataList != nil {
		honorTypes, err := types.DecodeDataList[types.HonorType](*resp)
		if err != nil {
			return nil, fmt.Errorf("GetHonorTypes 解析失败: %w", err)
		}
		return honorTypes, nil
	}

	// returnData 路径（简化字段）
	var opts []types.HonorType
	if err := types.DecodeReturnDataSlice(*resp, &opts); err != nil {
		return nil, fmt.Errorf("GetHonorTypes 解析 returnData 失败: %w", err)
	}
	return opts, nil
}

// GetHonorTypeForSelect 获取荣誉类型的级别下拉选项。
// 返回标签/值对（如 [{label:"校",value:5}]）。
func (c *Client) GetHonorTypeForSelect(ctx context.Context, token string) ([]types.HonorSelectOption, error) {
	resp, err := c.doBizAndDecode(ctx, token, "GetHonorTypeForSelect", "/api/studentMoralEduNew/getHonorTypeForSelect", http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetHonorTypeForSelect 失败: %w", err)
	}

	opts, err := types.DecodeReturnData[[]types.HonorSelectOption](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetHonorTypeForSelect 解析失败: %w", err)
	}
	if opts == nil {
		return []types.HonorSelectOption{}, nil
	}
	return *opts, nil
}

// GetHonorLevel 获取指定荣誉类型的可用级别。
func (c *Client) GetHonorLevel(ctx context.Context, token string, honorTypeID int64) ([]types.HonorSelectOption, error) {
	path := "/api/studentMoralEduNew/getHonorLevel?honorTypeId=" + strconv.FormatInt(honorTypeID, 10)

	resp, err := c.doBizAndDecode(ctx, token, "GetHonorLevel", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetHonorLevel 失败: %w", err)
	}

	opts, err := types.DecodeDataList[types.HonorSelectOption](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetHonorLevel 解析失败: %w", err)
	}
	return opts, nil
}

// GetHonorList 查询当前学生已申报的荣誉记录（分页）。
//
// 服务端要求同时带上 &key= 参数（可空值），否则返回 HTTP 400。
// pageNo 从 1 开始，pageSize 建议 20。
func (c *Client) GetHonorList(ctx context.Context, token string, pageNo, pageSize int) (*types.HonorListResult, error) {
	path := "/api/studentMoralEduNew/getHonorByStudentId?pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(pageSize) + "&key="

	resp, err := c.doBizAndDecode(ctx, token, "GetHonorList", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetHonorList 失败: %w", err)
	}

	// 解析分页信息
	pb, err := types.DecodePageBean(*resp)
	if err != nil {
		return nil, fmt.Errorf("GetHonorList 解析分页信息失败: %w", err)
	}

	// 解析记录列表
	records, err := types.DecodeDataList[types.HonorRecord](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetHonorList 解析荣誉记录失败: %w", err)
	}

	return &types.HonorListResult{
		Records: records,
		Page:    pb,
	}, nil
}

// DeleteHonor 删除一条荣誉记录。
//
// 真实抓包确认：接口为 GET 请求，ID 通过查询参数传递。
func (c *Client) DeleteHonor(ctx context.Context, token string, honorID int64) error {
	path := "/api/studentMoralEduNew/deleteHonorById?id=" + strconv.FormatInt(honorID, 10)
	return c.doBizVoid(ctx, token, "DeleteHonor", path, http.MethodGet, nil)
}

// AddHonor 申报一条荣誉。
//
// v1.4.0 增强：当 payload.TypeName 为空但 payload.TypeID > 0 时，
// 自动调用 GetHonorTypeForSelect 反查 typeId 对应的 label 补全 typeName，
// 避免因缺少 typeName 被 API 拒绝。调用方只需传 typeId 即可。
func (c *Client) AddHonor(ctx context.Context, token string, payload types.AddHonorPayload) error {
	// 自动补全 typeName：有 typeId 无 typeName 时自动反查
	if payload.TypeName == "" && payload.TypeID > 0 {
		opts, err := c.GetHonorTypeForSelect(ctx, token)
		if err != nil {
			return fmt.Errorf("AddHonor 自动反查 typeName 失败: %w", err)
		}

		found := false
		for _, opt := range opts {
			if opt.Value == int(payload.TypeID) {
				payload.TypeName = opt.Label
				c.logDebug("AddHonor 自动补全 typeName: typeId=%d → %q", payload.TypeID, opt.Label)
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("%w: typeId=%d 未找到对应的荣誉类型", ErrInvalidPayload, payload.TypeID)
		}
	}

	return c.doBizVoid(ctx, token, "AddHonor", "/api/studentMoralEduNew/addHonor", http.MethodPost, payload)
}

// UpdateHonor 更新一条荣誉记录。
// POST /api/studentMoralEduNew/updateHonor
func (c *Client) UpdateHonor(ctx context.Context, token string, payload map[string]any) error {
	return c.doBizVoid(ctx, token, "UpdateHonor",
		"/api/studentMoralEduNew/updateHonor", http.MethodPost, payload)
}
