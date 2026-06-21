// Package types 提供统一响应体 JSON 解析。
package types

import (
	"encoding/json"
	"fmt"
)

// UnifiedResponse 是目标平台的标准响应体结构。
// 使用 json.RawMessage 延迟解析，先解外层 code，再根据 code 走对应路径。
type UnifiedResponse struct {
	Code        int              `json:"code"`
	Msg         *string          `json:"msg"`
	ReturnData  *json.RawMessage `json:"returnData"`
	DataList    *json.RawMessage `json:"dataList"`
	DataMap     *json.RawMessage `json:"dataMap"`
	DataInt     int              `json:"dataInt"`
	DataString  *string          `json:"dataString"`
	PageBean    *json.RawMessage `json:"pageBean"`
	Note        *string          `json:"note"`
	InsertID    int64            `json:"insertID"`
	UpdateCount int              `json:"updateCount"`
	IsAttendance int             `json:"isAttendance"`
}

// DecodeResponse 解码目标平台统一响应体。
// 非 1 的 code 返回 ErrBusiness(code, msg)。
func DecodeResponse(data []byte) (UnifiedResponse, error) {
	var resp UnifiedResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, fmt.Errorf("解析响应体失败: %w", err)
	}
	return resp, nil
}

// CheckCode 检查 code 是否为 1（成功）。
// 如果不是，返回包含错误信息的 error。
func CheckCode(resp UnifiedResponse) error {
	if resp.Code == 1 {
		return nil
	}
	msg := "未知错误"
	if resp.Msg != nil && *resp.Msg != "" {
		msg = *resp.Msg
	}
	return fmt.Errorf("业务错误 (code=%d): %s", resp.Code, msg)
}

// DecodeReturnData 将 returnData 解析为目标类型。
func DecodeReturnData[T any](resp UnifiedResponse) (*T, error) {
	if resp.ReturnData == nil {
		return nil, nil
	}
	var v T
	if err := json.Unmarshal(*resp.ReturnData, &v); err != nil {
		return nil, fmt.Errorf("解析 returnData 失败: %w", err)
	}
	return &v, nil
}

// DecodeDataList 将 dataList 解析为切片。
func DecodeDataList[T any](resp UnifiedResponse) ([]T, error) {
	if resp.DataList == nil {
		return nil, nil
	}
	var v []T
	if err := json.Unmarshal(*resp.DataList, &v); err != nil {
		return nil, fmt.Errorf("解析 dataList 失败: %w", err)
	}
	return v, nil
}

// DecodeDataMap 将 dataMap 解析为目标类型。
func DecodeDataMap[T any](resp UnifiedResponse) (*T, error) {
	if resp.DataMap == nil {
		return nil, nil
	}
	var v T
	if err := json.Unmarshal(*resp.DataMap, &v); err != nil {
		return nil, fmt.Errorf("解析 dataMap 失败: %w", err)
	}
	return &v, nil
}

// EnforceCode 确保 code == 1，否则返回 error。同 CheckCode 但直接操作原始响应。
func EnforceCode(data []byte) error {
	aux := struct {
		Code int `json:"code"`
	}{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("解析响应 code 失败: %w", err)
	}
	if aux.Code != 1 {
		var msg string
		var resp UnifiedResponse
		if err := json.Unmarshal(data, &resp); err == nil && resp.Msg != nil {
			msg = *resp.Msg
		}
		return fmt.Errorf("业务错误 (code=%d): %s", aux.Code, msg)
	}
	return nil
}
