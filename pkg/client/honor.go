// honor.go 荣誉申报 SDK。
// 端点映射：
//
//	GET  /api/studentMoralEduNew/getHonorType          — 获取所有荣誉类型（dataList / returnData 双通道）
//	GET  /api/studentMoralEduNew/getHonorTypeForSelect  — 获取级别下拉选项（returnData）；荣誉类型选项在 dataList
//	GET  /api/studentMoralEduNew/getHonorLevel          — 获取荣誉级别（dataList，需 honorTypeId 参数）
//	GET  /api/studentMoralEduNew/getHonorByStudentId    — 查询已有荣誉（分页，需 &key=）
//	POST /api/studentMoralEduNew/addHonor               — 申报荣誉
//	POST /api/studentMoralEduNew/deleteHonorById        — 删除荣誉
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

// GetHonorTypeOptions 获取荣誉类型下拉选项（荣誉名称列表）。
//
// 对应前端 initHonorTypeAndLevel 中 dataList 的读取。
// 与 GetHonorTypeForSelect 调用同一个接口，但读取 dataList 而非 returnData。
func (c *Client) GetHonorTypeOptions(ctx context.Context, token string) ([]types.HonorSelectOption, error) {
	resp, err := c.doBizAndDecode(ctx, token, "GetHonorTypeOptions",
		"/api/studentMoralEduNew/getHonorTypeForSelect", http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetHonorTypeOptions 失败: %w", err)
	}

	opts, err := types.DecodeDataList[types.HonorSelectOption](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetHonorTypeOptions 解析失败: %w", err)
	}
	return opts, nil
}

// GetHonorTypeForSelect 获取荣誉等级下拉选项（returnData）。
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
// key 为搜索关键字（可空，会做 URL 转义）。
//
// BREAKING：v1.3.x 起签名新增 key 参数。
func (c *Client) GetHonorList(ctx context.Context, token string, pageNo, pageSize int, key string) (*types.HonorListResult, error) {
	path := "/api/studentMoralEduNew/getHonorByStudentId?pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(pageSize) + "&key=" + url.QueryEscape(key)

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

// ensureHonorTypeName 在 typeId 有值且 typeName 为空时，
// 通过 GetHonorTypeOptions（dataList 荣誉类型选项）反查并返回 typeName。
// typeName 已非空时直接返回原值；typeId 无效或为 0 时返回空串与 nil。
func (c *Client) ensureHonorTypeName(ctx context.Context, token string, typeID int64, typeName string) (string, error) {
	if typeName != "" || typeID <= 0 {
		return typeName, nil
	}
	opts, err := c.GetHonorTypeOptions(ctx, token)
	if err != nil {
		return "", fmt.Errorf("自动反查 typeName 失败: %w", err)
	}
	for _, opt := range opts {
		if opt.Value == int(typeID) {
			c.logDebug("ensureHonorTypeName 自动补全 typeName: typeId=%d → %q", typeID, opt.Label)
			return opt.Label, nil
		}
	}
	return "", fmt.Errorf("%w: typeId=%d 未找到对应的荣誉类型", ErrInvalidPayload, typeID)
}

// AddHonor 申报一条荣誉。
//
// v1.4.0 增强：当 payload.TypeName 为空但 payload.TypeID > 0 时，
// 自动调用 GetHonorTypeOptions 反查 typeId 对应的 label 补全 typeName，
// 避免因缺少 typeName 被 API 拒绝。调用方只需传 typeId 即可。
// v2.0.0 修复：原调用 GetHonorTypeForSelect（读取 returnData 即荣誉等级），
// 改为 GetHonorTypeOptions（读取 dataList 即荣誉类型选项）。
func (c *Client) AddHonor(ctx context.Context, token string, payload types.AddHonorPayload) error {
	name, err := c.ensureHonorTypeName(ctx, token, payload.TypeID, payload.TypeName)
	if err != nil {
		return fmt.Errorf("AddHonor %w", err)
	}
	payload.TypeName = name
	// 前端新增表单不传 name，只传 typeName；服务端部分路径仍读 name。
	// 空 name 时回落 typeName，与页面「选类型即名称」行为对齐。
	if payload.Name == "" {
		payload.Name = payload.TypeName
	}

	return c.doBizVoid(ctx, token, "AddHonor", "/api/studentMoralEduNew/addHonor", http.MethodPost, payload)
}

// UpdateHonor 更新一条荣誉记录。
// POST /api/studentMoralEduNew/updateHonor
//
// 与 AddHonor 对称：当 payload 含 typeId 且 typeName 为空/缺失时，
// 自动调用 GetHonorTypeOptions 反查补全 typeName。
func (c *Client) UpdateHonor(ctx context.Context, token string, payload map[string]any) error {
	if payload != nil {
		typeID, hasTypeID := honorMapInt64(payload["typeId"])
		typeName := honorMapString(payload["typeName"])
		if hasTypeID && typeName == "" {
			name, err := c.ensureHonorTypeName(ctx, token, typeID, typeName)
			if err != nil {
				return fmt.Errorf("UpdateHonor %w", err)
			}
			if name != "" {
				payload["typeName"] = name
			}
		}
	}
	return c.doBizVoid(ctx, token, "UpdateHonor",
		"/api/studentMoralEduNew/updateHonor", http.MethodPost, payload)
}

// honorMapString 从 map[string]any 安全取字符串（缺失/非 string 视为空）。
func honorMapString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// honorMapInt64 从 map[string]any 解析 typeId 类数值。
// JSON 反序列化常为 float64；也接受 int / int64 / json.Number。
func honorMapInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return i, true
	default:
		return 0, false
	}
}
