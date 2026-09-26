package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// TestGetSchoolID_LargeNumericIDNotScientific 锁定 7 位及以上的 school_id
// 不会因科学计数法被误判为「非有效数字」。
//
// 背景：school 来自 map[string]any，JSON 数字解码为 float64。原实现用
// fmt.Sprintf("%v") 格式化，而 %v 对 float64 走 %g 语义——实测 1234567
// 输出 "1.234567e+06"，随后的 ParseInt 必然失败，合法学校 ID 被拒。
// 现网 school_id 是小整数（173）故未暴露，但这是平台数据依赖的隐雷。
//
// 既有 school_id_validation_test.go 只用 3 位数，抓不到该分支。
func TestGetSchoolID_LargeNumericIDNotScientific(t *testing.T) {
	ids := []string{"173", "1234567", "12345678", "123456789012345", "9007199254740993"}

	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				// 以 JSON number 形态返回（与平台一致）
				_, _ = w.Write([]byte(`{"code":1,"dataList":[{"school_id":` + id + `,"NAME":"测试中学"}]}`))
			}))
			defer srv.Close()

			c, err := New(WithSSOBase(srv.URL), WithBaseURL(srv.URL))
			if err != nil {
				t.Fatalf("New() 失败: %v", err)
			}
			defer func() { _ = c.Close() }()

			info, err := c.GetSchoolID(context.Background(), "G350181200912110035")
			if err != nil {
				t.Fatalf("school_id=%s 应被接受，实际错误: %v", id, err)
			}
			if info.SchoolID != id {
				t.Errorf("school_id 应原样返回 %s，实际 %q", id, info.SchoolID)
			}
			// 必须是纯十进制，不得含指数记号
			if _, err := strconv.ParseInt(info.SchoolID, 10, 64); err != nil {
				t.Errorf("school_id %q 不是纯十进制: %v", info.SchoolID, err)
			}
			if info.SchoolName != "测试中学" {
				t.Errorf("学校名应为 测试中学，实际 %q", info.SchoolName)
			}
		})
	}
}

// TestGetSchoolID_RejectsNonNumeric 确认真正的非数字值仍被拒绝。
func TestGetSchoolID_RejectsNonNumeric(t *testing.T) {
	for _, badVal := range []string{`"abc"`, `"12a"`, `true`, `{}`} {
		t.Run(badVal, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"code":1,"dataList":[{"school_id":` + badVal + `,"NAME":"X"}]}`))
			}))
			defer srv.Close()

			c, err := New(WithSSOBase(srv.URL), WithBaseURL(srv.URL))
			if err != nil {
				t.Fatalf("New() 失败: %v", err)
			}
			defer func() { _ = c.Close() }()

			_, err = c.GetSchoolID(context.Background(), "user")
			if err == nil {
				t.Fatalf("school_id=%s 非有效数字，应被拒绝", badVal)
			}
			if !errors.Is(err, ErrInvalidPayload) {
				t.Errorf("应 errors.Is(ErrInvalidPayload)，实际: %v", err)
			}
		})
	}
}

// TestFormatSchoolID_Types 直接锁定 helper 的各类型分支。
func TestFormatSchoolID_Types(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"float64 小整数", float64(173), "173"},
		{"float64 七位", float64(1234567), "1234567"},
		{"float64 十五位", float64(123456789012345), "123456789012345"},
		{"json.Number 保留原字面量", json.Number("123456789012345678"), "123456789012345678"},
		{"int", int(173), "173"},
		{"int64", int64(173), "173"},
		{"string 原样", "173", "173"},
		{"非数字原样", "abc", "abc"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := formatSchoolID(c.in); got != c.want {
				t.Fatalf("期望 %q，实际 %q", c.want, got)
			}
		})
	}
}
