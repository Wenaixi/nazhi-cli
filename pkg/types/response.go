// Package types 提供统一响应体 JSON 解析。
package types

import (
	"encoding/json"
	"fmt"
)

// UnifiedResponse 是目标平台的标准响应体结构。
// 使用 json.RawMessage 延迟解析，先解外层 code，再根据 code 走对应路径。
//
// 历史注：旧版本曾带 6 个全仓 0 引用的孤儿字段（DataString / PageBean / Note /
// InsertID / UpdateCount / IsAttendance），删除后收敛到实际被使用的 6 个活跃字段。
type UnifiedResponse struct {
	Code       int              `json:"code"`
	Msg        *string          `json:"msg"`
	ReturnData *json.RawMessage `json:"returnData"`
	DataList   *json.RawMessage `json:"dataList"`
	DataMap    *json.RawMessage `json:"dataMap"`
	DataInt    int              `json:"dataInt"`
}

// DecodeResponse 解码目标平台统一响应体。
//
// 注意：DecodeResponse 仅负责把 resp body json.Unmarshal 到 UnifiedResponse 结构体。
// 业务 code 检查（code != 1 时返回错误）请使用独立的 CheckCode 方法。
// 历史上有人误以为 DecodeResponse 会自动抛 ErrBusiness，实际不会——这样设计
// 是为了让调用方能在拿到 UnifiedResponse 后自由分支（如先看 code 再选择性解析
// returnData / dataList / dataMap），避免双重解码或丢失原始 body。
func DecodeResponse(data []byte) (UnifiedResponse, error) {
	var resp UnifiedResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, fmt.Errorf("解析响应体失败: %w", err)
	}
	return resp, nil
}

// CheckCode 检查 code 是否为 1（成功）。
// 如果不是，返回 *BusinessError（保留数值 code 供 errors.As 判别）。
//
// 下游可用 errors.As(err, &bizErr) 拿到 Code 数值做精细分支
// （如 code=2 重试 / code=500 致命错误），不再局限于字符串匹配。
func CheckCode(resp UnifiedResponse) error {
	if resp.Code == 1 {
		return nil
	}
	msg := "未知错误"
	if resp.Msg != nil && *resp.Msg != "" {
		msg = *resp.Msg
	}
	return &BusinessError{Code: resp.Code, Msg: msg}
}

// BusinessError 业务错误，保留数值 code 供 errors.As 精细判别。
//
// 使用方法：
//
//	var bizErr *types.BusinessError
//	if errors.As(err, &bizErr) {
//	    switch bizErr.Code {
//	    case 2: // 重试
//	    case 500: // 致命
//	    }
//	}
type BusinessError struct {
	Code int    // 业务 code（非 1）
	Msg  string // 错误描述
}

func (e *BusinessError) Error() string {
	return fmt.Sprintf("业务错误 (code=%d): %s", e.Code, e.Msg)
}

// decodeField 内部辅助，消除 DecodeReturnData / DecodeDataMap 重复。
func decodeField[T any](raw *json.RawMessage, name string) (*T, error) {
	if raw == nil {
		return nil, nil
	}
	var v T
	if err := json.Unmarshal(*raw, &v); err != nil {
		return nil, fmt.Errorf("解析 %s 失败: %w", name, err)
	}
	return &v, nil
}

// decodeFieldSlice 内部辅助，消除 DecodeDataList 重复。
func decodeFieldSlice[T any](raw *json.RawMessage, name string) ([]T, error) {
	if raw == nil {
		return nil, nil
	}
	var v []T
	if err := json.Unmarshal(*raw, &v); err != nil {
		return nil, fmt.Errorf("解析 %s 失败: %w", name, err)
	}
	return v, nil
}

// DecodeReturnData 将 returnData 解析为目标类型。
func DecodeReturnData[T any](resp UnifiedResponse) (*T, error) {
	return decodeField[T](resp.ReturnData, "returnData")
}

// DecodeDataList 将 dataList 解析为切片。
func DecodeDataList[T any](resp UnifiedResponse) ([]T, error) {
	return decodeFieldSlice[T](resp.DataList, "dataList")
}

// DecodeDataMap 将 dataMap 解析为目标类型。
func DecodeDataMap[T any](resp UnifiedResponse) (*T, error) {
	return decodeField[T](resp.DataMap, "dataMap")
}

// DecodeUnified 是"解析响应体 + 检查业务码"二合一原语（候选 #3）。
//
// Deprecated: 当前无生产调用方，仅在测试中被调用。三个月观察期。
// TODO(2026-09-28): 仍无调用方，v0.4.0 删除。
// 等待 SSO 域（auth.go）或业务域接入后，可消除 client 层 doBizAndDecode
// 的 4 行 boilerplate（DecodeResponse + CheckCode + errors.Join ErrBusinessRejected
// + 错误前缀）。
// 处置计划：2026-09-28 前仍无生产调用方 → 标记 Deprecated；v0.4.0 删除。
//
// 流程：
//  1. json.Unmarshal 到 UnifiedResponse（失败 → 返回 wrap 后的解析错误）
//  2. CheckCode 检查 code（code=1 通过；≠1 → 返回 *BusinessError）
//
// 返回值语义：
//   - 成功（code=1）：resp != nil, err == nil
//   - 业务拒绝（code≠1）：resp == nil, err 为 *BusinessError（errors.As 可取）
//   - JSON 解析失败：resp == nil, err 含 "解析响应体失败" 前缀
//
// 设计动机：消除 client 层 doBizAndDecode 的 4 行 boilerplate
// （DecodeResponse + CheckCode + errors.Join ErrBusinessRejected + 错误前缀），
// 让 SSO 域（auth.go）和业务域都能复用。types 包不依赖 client.ErrBusinessRejected，
// 业务拒绝的 sentinel wrap 由 client 层在编排时附加。
func DecodeUnified(body []byte) (*UnifiedResponse, error) {
	resp, err := DecodeResponse(body)
	if err != nil {
		return nil, err // 已是 wrap "解析响应体失败" 的格式
	}
	if err := CheckCode(resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
