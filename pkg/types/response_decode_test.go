// Package types 内部白盒测试。
package types

import (
	"encoding/json"
	"testing"
)

// TestDecodeReturnData_Nil 验证 returnData 为 nil 时返回 nil, nil。
func TestDecodeReturnData_Nil(t *testing.T) {
	resp := UnifiedResponse{Code: 1, ReturnData: nil}
	v, err := DecodeReturnData[string](resp)
	if err != nil {
		t.Fatalf("nil returnData 不应返回 err: %v", err)
	}
	if v != nil {
		t.Fatalf("nil returnData 应返回 nil，得到 %v", *v)
	}
}

// TestDecodeReturnData_Success 验证正常解析 returnData。
func TestDecodeReturnData_Success(t *testing.T) {
	raw := json.RawMessage(`"hello"`)
	resp := UnifiedResponse{Code: 1, ReturnData: &raw}
	v, err := DecodeReturnData[string](resp)
	if err != nil {
		t.Fatalf("正常解析不应 err: %v", err)
	}
	if v == nil {
		t.Fatal("正常解析不应返回 nil")
	}
	if *v != "hello" {
		t.Fatalf("期望 hello，得到 %s", *v)
	}
}

// TestDecodeReturnData_ParseError 验证 returnData 解析失败返回错误。
func TestDecodeReturnData_ParseError(t *testing.T) {
	raw := json.RawMessage(`not valid json`)
	resp := UnifiedResponse{Code: 1, ReturnData: &raw}
	_, err := DecodeReturnData[string](resp)
	if err == nil {
		t.Fatal("解析失败应返回 err")
	}
}

// TestDecodeDataList_Nil 验证 dataList 为 nil 时返回 nil, nil。
func TestDecodeDataList_Nil(t *testing.T) {
	resp := UnifiedResponse{Code: 1, DataList: nil}
	v, err := DecodeDataList[string](resp)
	if err != nil {
		t.Fatalf("nil dataList 不应返回 err: %v", err)
	}
	if v != nil {
		t.Fatalf("nil dataList 应返回 nil，得到 %v", v)
	}
}

// TestDecodeDataList_Success 验证正常解析 dataList。
func TestDecodeDataList_Success(t *testing.T) {
	raw := json.RawMessage(`["a","b"]`)
	resp := UnifiedResponse{Code: 1, DataList: &raw}
	v, err := DecodeDataList[string](resp)
	if err != nil {
		t.Fatalf("正常解析不应 err: %v", err)
	}
	if len(v) != 2 || v[0] != "a" || v[1] != "b" {
		t.Fatalf("期望 [a b]，得到 %v", v)
	}
}

// TestDecodeDataList_ParseError 验证 dataList 解析失败返回错误。
func TestDecodeDataList_ParseError(t *testing.T) {
	raw := json.RawMessage(`not valid`)
	resp := UnifiedResponse{Code: 1, DataList: &raw}
	_, err := DecodeDataList[string](resp)
	if err == nil {
		t.Fatal("解析失败应返回 err")
	}
}

// TestDecodeDataMap_Nil 验证 dataMap 为 nil 时返回 nil, nil。
func TestDecodeDataMap_Nil(t *testing.T) {
	resp := UnifiedResponse{Code: 1, DataMap: nil}
	v, err := DecodeDataMap[string](resp)
	if err != nil {
		t.Fatalf("nil dataMap 不应返回 err: %v", err)
	}
	if v != nil {
		t.Fatalf("nil dataMap 应返回 nil，得到 %v", *v)
	}
}

// TestDecodeDataMap_Success 验证正常解析 dataMap。
func TestDecodeDataMap_Success(t *testing.T) {
	raw := json.RawMessage(`"world"`)
	resp := UnifiedResponse{Code: 1, DataMap: &raw}
	v, err := DecodeDataMap[string](resp)
	if err != nil {
		t.Fatalf("正常解析不应 err: %v", err)
	}
	if v == nil || *v != "world" {
		t.Fatalf("期望 world，得到 %v", v)
	}
}

// TestDecodeDataMap_ParseError 验证 dataMap 解析失败返回错误。
func TestDecodeDataMap_ParseError(t *testing.T) {
	raw := json.RawMessage(`bad`)
	resp := UnifiedResponse{Code: 1, DataMap: &raw}
	_, err := DecodeDataMap[string](resp)
	if err == nil {
		t.Fatal("解析失败应返回 err")
	}
}
