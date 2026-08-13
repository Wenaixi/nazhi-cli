package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestGetViolationListMatchesFrontendRequest 验证违规列表请求和结构化响应。
func TestGetViolationListMatchesFrontendRequest(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentMoralEduNew/getViolation" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("期望 GET，实际 %s", r.Method)
		}
		query := r.URL.Query()
		if query.Get("pageNo") != "2" || query.Get("pageSize") != "6" || query.Get("key") != "迟到 记录" {
			t.Errorf("分页或关键字参数错误: %v", query)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功","pageBean":{"pageNo":2,"pageSize":6,"totalNum":1,"totalPage":1},"dataList":[{"id":9001,"studentName":"示例学生","className":"一班","gradeName":"高一","typeName":"迟到","name":"上课迟到","fromTableName":"校级","score":"1.5","attachmentId":null,"getDateStr":"2026-08-01","creatorName":"班主任","ifshow":"是"}]}`))
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	result, err := c.GetViolationList(context.Background(), "test-token", 2, 6, "迟到 记录")
	if err != nil {
		t.Fatalf("GetViolationList 失败: %v", err)
	}
	if result == nil || len(result.Records) != 1 || result.Page == nil {
		t.Fatalf("违规列表结果不完整: %+v", result)
	}
	if result.Records[0].Score.Float64() != 1.5 || result.Records[0].AttachmentID != nil {
		t.Fatalf("违规记录字段错误: %+v", result.Records[0])
	}
	if result.Page.TotalNum != 1 {
		t.Fatalf("分页总数错误: %+v", result.Page)
	}
}

// TestGetViolationListJSONPreservesPageShape 验证原始 JSON 保留 records/page 结构。
func TestGetViolationListJSONPreservesPageShape(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentMoralEduNew/getViolation" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"dataList":[{"name":"上课迟到","rawField":"must-keep"}],"pageBean":{"pageNo":1,"pageSize":3,"totalNum":1,"totalPage":1}}`))
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	raw, err := c.GetViolationListJSON(context.Background(), "test-token", 1, 3, "")
	if err != nil {
		t.Fatalf("GetViolationListJSON 失败: %v", err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("原始结果不是合法 JSON: %v", err)
	}
	if _, ok := object["records"]; !ok {
		t.Fatalf("原始结果缺少 records: %s", raw)
	}
	if _, ok := object["page"]; !ok {
		t.Fatalf("原始结果缺少 page: %s", raw)
	}
	if !strings.Contains(string(raw), `"rawField":"must-keep"`) {
		t.Fatalf("原始字段未保留: %s", raw)
	}
}

// TestGetViolationTypesMatchesFrontendDataList 验证违规类型从 dataList 读取。
func TestGetViolationTypesMatchesFrontendDataList(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentMoralEduNew/getViolationType" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"dataList":[{"id":7,"name":"迟到"}]}`))
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	items, err := c.GetViolationTypes(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetViolationTypes 失败: %v", err)
	}
	if len(items) != 1 || items[0].ID != 7 || items[0].Name != "迟到" {
		t.Fatalf("违规类型结果错误: %+v", items)
	}
}
