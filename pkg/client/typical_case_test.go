package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// typicalCaseRecordJSON 生成一条典型案例记录（mock 用）。
func typicalCaseRecordJSON(id int64, title, typeName, roleName string) map[string]any {
	return map[string]any{
		"id":             id,
		"title":          title,
		"typeName":       typeName,
		"teacherName":    "王隆滨",
		"partnerName":    "庄羽强等",
		"roleName":       roleName,
		"remark":         "测试备注",
		"content":        "测试正文",
		"attachmentId":   5139876,
		"attachmentName": "test.jpg",
		"status":         0,
		"statusName":     "未审核",
		"termId":         18,
		"termName":       "2025-2026学年下学期",
		"gradeName":      "高一",
		"className":      "八班",
		"studentName":    "高博文",
	}
}

// typicalCasePageBeanJSON 生成 mock 分页信息。
func typicalCasePageBeanJSON(pageNo, pageSize, totalNum int) json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"pageNo":    pageNo,
		"pageSize":  pageSize,
		"totalNum":  totalNum,
		"totalPage": (totalNum + pageSize - 1) / pageSize,
	})
	return json.RawMessage(b)
}

// TestAddTypicalCase 验证提交典型案例成功。
func TestAddTypicalCase(t *testing.T) {
	var gotBody map[string]any
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/addTypicalCase" {
			if r.Method != http.MethodPost {
				t.Errorf("期望 POST，实际 %s", r.Method)
			}
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	err := c.AddTypicalCase(context.Background(), "test-token", types.AddTypicalCasePayload{
		Title:          "论国内外各领域AI大模型能力对比",
		Type:           "1",
		TypeName:       "研究性学习报告",
		TeacherName:    "王隆滨",
		PartnerName:    "庄羽强等",
		Role:           "1",
		RoleName:       "负责人",
		Content:        "测试正文",
		Level:          "5",
		LevelName:      "学校",
		AttachmentID:   5139876,
		AttachmentName: "test.jpg",
	})
	if err != nil {
		t.Fatalf("AddTypicalCase 失败: %v", err)
	}

	if gotBody["title"] != "论国内外各领域AI大模型能力对比" {
		t.Errorf("期望 title=论国内外各领域AI大模型能力对比，实际 %v", gotBody["title"])
	}
	if gotBody["type"] != "1" {
		t.Errorf("期望 type=1，实际 %v", gotBody["type"])
	}
	if gotBody["role"] != "1" {
		t.Errorf("期望 role=1，实际 %v", gotBody["role"])
	}
	if gotBody["level"] != "5" {
		t.Errorf("期望 level=5，实际 %v", gotBody["level"])
	}
	if gotBody["attachmentId"] != float64(5139876) {
		t.Errorf("期望 attachmentId=5139876，实际 %v", gotBody["attachmentId"])
	}
}

// TestAddTypicalCase_BizError 验证提交典型案例的业务错误。
func TestAddTypicalCase_BizError(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/addTypicalCase" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":-1,"msg":"标题不能为空"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	err := c.AddTypicalCase(context.Background(), "test-token", types.AddTypicalCasePayload{})
	if err == nil {
		t.Fatal("期望业务错误，实际 nil")
	}
	if !strings.Contains(err.Error(), "标题不能为空") {
		t.Errorf("错误消息不匹配: %v", err)
	}
}

// TestGetTypicalCaseList 验证获取典型案例列表。
func TestGetTypicalCaseList(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getTypicalCase" {
			q := r.URL.Query()
			if q.Get("pageNo") != "1" || q.Get("pageSize") != "20" || q.Get("status") != "3" {
				t.Errorf("参数错误: pageNo=%s pageSize=%s status=%s", q.Get("pageNo"), q.Get("pageSize"), q.Get("status"))
			}
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": typicalCasePageBeanJSON(1, 20, 1),
				"dataList": []map[string]any{
					typicalCaseRecordJSON(20034, "论国内外各领域AI大模型能力对比", "研究性学习报告", "负责人"),
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	result, err := c.GetTypicalCaseList(context.Background(), "test-token", 1, 20)
	if err != nil {
		t.Fatalf("GetTypicalCaseList 失败: %v", err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("期望 1 条记录，实际 %d", len(result.Records))
	}
	if result.Records[0].Title != "论国内外各领域AI大模型能力对比" || result.Records[0].TypeName != "研究性学习报告" {
		t.Errorf("字段解析错误: %+v", result.Records[0])
	}
	if result.Records[0].RoleName != "负责人" {
		t.Errorf("期望 RoleName=负责人，实际 %s", result.Records[0].RoleName)
	}
	if result.Page == nil || result.Page.TotalNum != 1 {
		t.Errorf("分页信息错误: %+v", result.Page)
	}
}

// TestGetTypicalCaseList_Empty 验证获取空列表。
func TestGetTypicalCaseList_Empty(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getTypicalCase" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": typicalCasePageBeanJSON(1, 20, 0),
				"dataList": []map[string]any{},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	result, err := c.GetTypicalCaseList(context.Background(), "test-token", 1, 20)
	if err != nil {
		t.Fatalf("GetTypicalCaseList 失败: %v", err)
	}
	if len(result.Records) != 0 {
		t.Fatalf("期望 0 条记录，实际 %d", len(result.Records))
	}
	if result.Page == nil || result.Page.TotalPage != 0 {
		t.Errorf("分页信息错误: %+v", result.Page)
	}
}

// TestGetTypicalCaseListJSON 验证原生 JSON 输出。
func TestGetTypicalCaseListJSON(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getTypicalCase" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": json.RawMessage(`{"pageNo":1,"pageSize":20,"totalNum":1,"totalPage":1}`),
				"dataList": []map[string]any{
					typicalCaseRecordJSON(20034, "论国内外各领域AI大模型能力对比", "研究性学习报告", "负责人"),
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	raw, err := c.GetTypicalCaseListJSON(context.Background(), "test-token", 1, 20)
	if err != nil {
		t.Fatalf("GetTypicalCaseListJSON 失败: %v", err)
	}

	if len(raw) == 0 {
		t.Fatal("期望非空 JSON，实际空")
	}
	if raw[0] != '{' {
		t.Fatalf("期望 JSON 对象，实际: %s", string(raw[:20]))
	}

	// 验证结构包含 records 和 page
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("解析 JSON 失败: %v", err)
	}
	if _, ok := parsed["records"]; !ok {
		t.Error("期望结果包含 records 字段")
	}
	if _, ok := parsed["page"]; !ok {
		t.Error("期望结果包含 page 字段")
	}

	// 验证 records 是数组
	var records []any
	if err := json.Unmarshal(parsed["records"], &records); err != nil {
		t.Fatalf("解析 records 失败: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("期望 1 条记录，实际 %d", len(records))
	}
}

// TestGetTypicalCaseList_DefaultStatus 验证默认 status=3（前端「全部」）。
func TestGetTypicalCaseList_DefaultStatus(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getTypicalCase" {
			q := r.URL.Query()
			if q.Get("status") != "3" {
				t.Errorf("期望默认 status=3，实际 %s", q.Get("status"))
			}
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": typicalCasePageBeanJSON(1, 20, 0),
				"dataList": []map[string]any{},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	// 默认 status 与三参数旧调用兼容：不传 status 时用 3
	_, err := c.GetTypicalCaseList(context.Background(), "test-token", 1, 20)
	if err != nil {
		t.Fatalf("GetTypicalCaseList 失败: %v", err)
	}
}

// TestGetTypicalCaseList_CustomStatus 验证可按审核状态筛选（0 未审 / 1 通过 / 2 驳回 / 3 全部）。
func TestGetTypicalCaseList_CustomStatus(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getTypicalCase" {
			q := r.URL.Query()
			if q.Get("status") != "1" {
				t.Errorf("期望 status=1，实际 %s", q.Get("status"))
			}
			if q.Get("pageNo") != "1" || q.Get("pageSize") != "20" {
				t.Errorf("分页参数错误: pageNo=%s pageSize=%s", q.Get("pageNo"), q.Get("pageSize"))
			}
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": typicalCasePageBeanJSON(1, 20, 0),
				"dataList": []map[string]any{},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.GetTypicalCaseList(context.Background(), "test-token", 1, 20, 1)
	if err != nil {
		t.Fatalf("GetTypicalCaseList 失败: %v", err)
	}
	_, err = c.GetTypicalCaseListJSON(context.Background(), "test-token", 1, 20, 1)
	if err != nil {
		t.Fatalf("GetTypicalCaseListJSON 失败: %v", err)
	}
}
