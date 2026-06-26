package client

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestTryDecodeFallback_ReturnData 验证 returnData → dataMap fallback 路径：
// returnData 有值且解析成功时应该返回 returnData 中的值。
func TestTryDecodeFallback_ReturnData(t *testing.T) {
	c, _ := New(WithTimeout(time.Second))
	defer func() { _ = c.Close() }()

	rawData := json.RawMessage(`{"id":1,"name":"张三"}`)
	resp := &types.UnifiedResponse{
		Code:       1,
		ReturnData: &rawData,
	}

	v := tryDecodeFallback(c, "test",
		func() (*testUser, error) { return types.DecodeReturnData[testUser](*resp) },
		func() (*testUser, error) { return types.DecodeDataMap[testUser](*resp) },
	)
	if v == nil {
		t.Fatal("期望非 nil，returnData 应解码成功")
	}
	if v.ID != 1 || v.Name != "张三" {
		t.Errorf("returnData 字段错误: %+v", v)
	}
}

// TestTryDecodeFallback_DataMap 验证 returnData → dataMap fallback 路径：
// returnData 为 nil，dataMap 有值且解析成功时应该返回 dataMap 中的值。
func TestTryDecodeFallback_DataMap(t *testing.T) {
	c, _ := New(WithTimeout(time.Second))
	defer func() { _ = c.Close() }()

	rawMap := json.RawMessage(`{"id":2,"name":"李四"}`)
	resp := &types.UnifiedResponse{
		Code:    1,
		DataMap: &rawMap,
	}

	v := tryDecodeFallback(c, "test",
		func() (*testUser, error) { return types.DecodeReturnData[testUser](*resp) },
		func() (*testUser, error) { return types.DecodeDataMap[testUser](*resp) },
	)
	if v == nil {
		t.Fatal("期望非 nil，dataMap 应解码成功")
	}
	if v.ID != 2 || v.Name != "李四" {
		t.Errorf("dataMap 字段错误: %+v", v)
	}
}

// TestTryDecodeFallback_AllNil 验证全部 fallback 路径返回 nil 时返回 nil。
func TestTryDecodeFallback_AllNil(t *testing.T) {
	c, _ := New(WithTimeout(time.Second))
	defer func() { _ = c.Close() }()

	resp := &types.UnifiedResponse{Code: 1}

	v := tryDecodeFallback(c, "test",
		func() (*testUser, error) { return types.DecodeReturnData[testUser](*resp) },
		func() (*testUser, error) { return types.DecodeDataMap[testUser](*resp) },
	)
	if v != nil {
		t.Fatalf("期望 nil，全部 fallback 应失败，得到 %+v", v)
	}
}

// TestTryDecodeFallback_PartialFailure 验证第一个 decoder 返回 err
// （如类型不匹配导致 unmarshal 失败）时自动尝试下一个 decoder。
func TestTryDecodeFallback_PartialFailure(t *testing.T) {
	c, _ := New(WithTimeout(time.Second))
	defer func() { _ = c.Close() }()

	rawMap := json.RawMessage(`{"id":3,"name":"王五"}`)
	resp := &types.UnifiedResponse{
		Code:       1,
		ReturnData: &rawMap,
		DataMap:    &rawMap,
	}

	// 第一个 decode 总是失败的 decoder
	var triedFirst bool
	v := tryDecodeFallback(c, "test",
		func() (*testUser, error) {
			triedFirst = true
			return nil, json.Unmarshal([]byte(`"not an object"`), new(testUser))
		},
		func() (*testUser, error) { return types.DecodeDataMap[testUser](*resp) },
	)
	if !triedFirst {
		t.Fatal("应该尝试第一个 decoder")
	}
	if v == nil {
		t.Fatal("期望非 nil，第二个 decoder 应兜底成功")
	}
	if v.ID != 3 || v.Name != "王五" {
		t.Errorf("兜底解码字段错误: %+v", v)
	}
}

// testUser 是测试用的简单结构体。
type testUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
