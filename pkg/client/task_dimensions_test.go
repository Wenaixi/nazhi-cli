// Package client_test 包含 task 维度的针对性测试。
package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetDimensions 验证 GetDimensions 返回完整维度列表。
// 用途：Finding #6 抽取 fetchDimensions helper 后此测试继续通过，
// 证明 helper 行为与原 GetDimensions 等价。
func TestGetDimensions(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(1, "成功", nil, []map[string]any{
				{"id": 0, "name": "全部"},
				{"id": 9, "name": "思想品德"},
				{"id": 14, "name": "劳动素养"},
			})))
		default:
			t.Errorf("未预期的请求: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	dims, err := c.GetDimensions(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetDimensions 失败: %v", err)
	}
	if len(dims) != 3 {
		t.Fatalf("期望 3 个维度, 得到 %d", len(dims))
	}
	if dims[0].ID != 0 || dims[0].Name != "全部" {
		t.Errorf("维度 0 错误: %+v", dims[0])
	}
	if dims[1].ID != 9 || dims[1].Name != "思想品德" {
		t.Errorf("维度 1 错误: %+v", dims[1])
	}
	if dims[2].ID != 14 || dims[2].Name != "劳动素养" {
		t.Errorf("维度 2 错误: %+v", dims[2])
	}
}

// TestGetDimensions_BizError 验证 GetDimensions 在业务错误时返回错误。
func TestGetDimensions_BizError(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// code=0 业务错误
			_, _ = w.Write([]byte(unifiedJSON(0, "未授权", nil, nil)))
		default:
			t.Errorf("未预期的请求: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.GetDimensions(context.Background(), "test-token")
	if err == nil {
		t.Fatal("期望业务错误，但得到 nil")
	}
}
