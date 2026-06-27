// Package types 内部白盒测试。
package types

import (
	"errors"
	"strings"
	"testing"
)

// TestCheckCode_ReturnsBusinessErrorWithCode 回归测试：CheckCode 必须返回
// *BusinessError 让 errors.As 拿回 code 数值。
//
// 历史 bug：CheckCode 用 fmt.Errorf 把 code 嵌入错误信息字符串，
// 下游 errors.As 拿不到结构化 code，无法区分 code=2 vs code=500 vs code=999。
func TestCheckCode_ReturnsBusinessErrorWithCode(t *testing.T) {
	err := CheckCode(UnifiedResponse{Code: 500, Msg: strPtr("密码错误")})
	if err == nil {
		t.Fatal("code=500 应返回 error")
	}
	var bizErr *BusinessError
	if !errors.As(err, &bizErr) {
		t.Fatalf("CheckCode 应返回可 errors.As 的 *BusinessError，实际类型 %T", err)
	}
	if bizErr.Code != 500 {
		t.Errorf("Code 错：%d", bizErr.Code)
	}
	if bizErr.Msg != "密码错误" {
		t.Errorf("Msg 错：%s", bizErr.Msg)
	}
}

// TestCheckCode_Code1ReturnsNil 验证成功码 1 不返回 error。
func TestCheckCode_Code1ReturnsNil(t *testing.T) {
	if err := CheckCode(UnifiedResponse{Code: 1, Msg: strPtr("ok")}); err != nil {
		t.Errorf("code=1 应返回 nil，实际 %v", err)
	}
}

// TestCheckCode_MissingMsgFallback 验证 msg 缺失时使用"未知错误"占位。
func TestCheckCode_MissingMsgFallback(t *testing.T) {
	err := CheckCode(UnifiedResponse{Code: 999})
	var bizErr *BusinessError
	if !errors.As(err, &bizErr) {
		t.Fatal("应返回 *BusinessError")
	}
	if bizErr.Msg != "未知错误" {
		t.Errorf("Msg 缺失时应 fallback '未知错误'，实际 %q", bizErr.Msg)
	}
}

func strPtr(s string) *string { return &s }

// ─── DecodeUnified 原语测试（候选 #3）───

// TestDecodeUnified_Code1_ReturnsResponse 验证 code=1 时直接返回 *UnifiedResponse。
func TestDecodeUnified_Code1_ReturnsResponse(t *testing.T) {
	body := []byte(`{"code":1,"msg":"成功","returnData":{"id":1,"name":"张三"}}`)
	resp, err := DecodeUnified(body)
	if err != nil {
		t.Fatalf("code=1 不应返回 error，实际: %v", err)
	}
	if resp == nil {
		t.Fatal("resp 不应为 nil")
	}
	if resp.Code != 1 {
		t.Errorf("Code 错：%d", resp.Code)
	}
	if resp.ReturnData == nil {
		t.Error("ReturnData 应有值")
	}
}

// TestDecodeUnified_BadJSON_ReturnsParseError 验证解析失败时返回 wrap 后错误。
func TestDecodeUnified_BadJSON_ReturnsParseError(t *testing.T) {
	body := []byte(`这不是 JSON`)
	_, err := DecodeUnified(body)
	if err == nil {
		t.Fatal("解析失败应返回 error")
	}
	// 必须包含原始错误上下文（便于排查 server 端异常）
	if !strings.Contains(err.Error(), "解析响应体失败") {
		t.Errorf("错误应包含解析上下文，实际: %v", err)
	}
}

// TestDecodeUnified_Code0_ReturnsBusinessError 验证 code≠1 时返回 *BusinessError
// （由 CheckCode 产生），便于 errors.As 拿结构化 code 数值做精细分支。
//
// 注：types 包不依赖 client.ErrBusinessRejected（避免包间依赖污染），
// wrap ErrBusinessRejected 的语义在 client 层 doBizAndDecode 路径保留。
func TestDecodeUnified_Code0_ReturnsBusinessError(t *testing.T) {
	body := []byte(`{"code":0,"msg":"业务拒绝"}`)
	_, err := DecodeUnified(body)
	if err == nil {
		t.Fatal("code=0 应返回 error")
	}
	var bizErr *BusinessError
	if !errors.As(err, &bizErr) {
		t.Fatalf("应返回 *BusinessError，实际类型 %T", err)
	}
	if bizErr.Code != 0 {
		t.Errorf("Code 错：%d", bizErr.Code)
	}
	if bizErr.Msg != "业务拒绝" {
		t.Errorf("Msg 错：%s", bizErr.Msg)
	}
}

// TestDecodeUnified_MissingMsgFallback 验证 msg 缺失时 BusinessError.Msg = "未知错误"。
func TestDecodeUnified_MissingMsgFallback(t *testing.T) {
	body := []byte(`{"code":999}`)
	_, err := DecodeUnified(body)
	var bizErr *BusinessError
	if !errors.As(err, &bizErr) {
		t.Fatal("应返回 *BusinessError")
	}
	if bizErr.Msg != "未知错误" {
		t.Errorf("Msg 缺失时应 fallback '未知错误'，实际 %q", bizErr.Msg)
	}
}
