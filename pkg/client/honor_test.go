// honor_test.go 荣誉申报 SDK 测试。
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

// ─── 辅助 ───

// honorRecordJSON 生成一条荣誉记录（v1.4.0 snake_case 字段名对齐 API）。
func honorRecordJSON(id int64, name, statusName string) map[string]any {
	return map[string]any{
		"id":                id,
		"type_name":         name,
		"level_name":        "校",
		"level":             5,
		"dimension_name":    "思想品德",
		"approved":          true,
		"statusName":        statusName,
		"get_date":          "2026-06-30T00:00:00+08:00",
		"evaluation_agency": "示例中学",
	}
}

// ─── 测试: GetHonorTypes ───

// TestGetHonorTypes 验证 GetHonorTypes 正常返回荣誉类型列表。
func TestGetHonorTypes(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentMoralEduNew/getHonorType" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code": 1,
				"dataList": []map[string]any{
					{"id": 1147, "name": "honor-a", "level_name": "school", "level": 5, "dimension_name": "conduct", "score": "分数：+5.0"},
					{"id": 1148, "name": "honor-b", "level_name": "city", "level": 4, "dimension_name": "study", "score": "分数：+4.0"},
				},
				"returnData": []map[string]any{
					{"id": 9999, "name": "return-data"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	types, err := c.GetHonorTypes(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetHonorTypes 失败: %v", err)
	}
	if len(types) != 2 {
		t.Fatalf("期望 2 个荣誉类型，实际 %d", len(types))
	}
	if types[0].ID != 1147 || types[0].Name != "honor-a" {
		t.Errorf("字段解析错误: %+v", types[0])
	}
	if types[0].LevelName != "school" || types[0].DimensionName != "conduct" || types[0].Score != "分数：+5.0" {
		t.Errorf("真实 snake_case 字段解析错误: %+v", types[0])
	}
}

// TestGetHonorTypes_FallbackReturnDataWhenDataListEmpty 验证 dataList 为空时回退到 returnData。
func TestGetHonorTypes_FallbackReturnDataWhenDataListEmpty(t *testing.T) {
	for _, tc := range []struct {
		name     string
		dataList string
	}{
		{name: "empty array", dataList: "[]"},
		{name: "null", dataList: "null"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/studentMoralEduNew/getHonorType" {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"code":1,"dataList":` + tc.dataList + `,"returnData":[{"id":2001,"name":"fallback-type","level_name":"school","level":5,"dimension_name":"conduct","score":"+5"}]}`))
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})))
			defer biz.Close()

			c := newTestClient(nil, biz, nil)
			got, err := c.GetHonorTypes(context.Background(), "test-token")
			if err != nil {
				t.Fatalf("GetHonorTypes 失败: %v", err)
			}
			if len(got) != 1 || got[0].ID != 2001 || got[0].Name != "fallback-type" {
				t.Fatalf("期望从 returnData 回退解析 1 条类型，实际 %+v", got)
			}
		})
	}
}

// TestGetHonorTypes_BizError 验证业务错误被正确包装。
func TestGetHonorTypes_BizError(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentMoralEduNew/getHonorType" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":-1,"msg":"系统错误"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.GetHonorTypes(context.Background(), "test-token")
	if err == nil {
		t.Fatal("期望业务错误，实际 nil")
	}
}

// ─── 测试: GetHonorTypeForSelect ───

// TestGetHonorTypeForSelect 验证获取下拉选项。
func TestGetHonorTypeForSelect(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentMoralEduNew/getHonorTypeForSelect" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code": 1,
				"returnData": []map[string]any{
					{"label": "校", "value": 5},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	opts, err := c.GetHonorTypeForSelect(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetHonorTypeForSelect 失败: %v", err)
	}
	if len(opts) != 1 {
		t.Fatalf("期望 1 个选项，实际 %d", len(opts))
	}
	if opts[0].Label != "校" || opts[0].Value != 5 {
		t.Errorf("字段解析错误: %+v", opts[0])
	}
}

// ─── 测试: GetHonorLevel ───

// TestGetHonorLevel 验证获取指定荣誉类型的级别。
func TestGetHonorLevel(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentMoralEduNew/getHonorLevel" {
			q := r.URL.Query()
			if q.Get("honorTypeId") != "1147" {
				t.Errorf("期望 honorTypeId=1147，实际 %s", q.Get("honorTypeId"))
			}
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code": 1,
				"dataList": []map[string]any{
					{"label": "校", "value": 5},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	opts, err := c.GetHonorLevel(context.Background(), "test-token", 1147)
	if err != nil {
		t.Fatalf("GetHonorLevel 失败: %v", err)
	}
	if len(opts) != 1 {
		t.Fatalf("期望 1 个级别，实际 %d", len(opts))
	}
	if opts[0].Value != 5 {
		t.Errorf("期望 value=5，实际 %d", opts[0].Value)
	}
}

// ─── 测试: GetHonorList ───

// TestGetHonorList 验证获取荣誉申报列表。
func TestGetHonorList(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentMoralEduNew/getHonorByStudentId" {
			if r.URL.Query().Get("pageNo") != "1" {
				t.Errorf("期望 pageNo=1，实际 %s", r.URL.Query().Get("pageNo"))
			}
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": json.RawMessage(`{"pageNo":1,"pageSize":20,"totalNum":1,"totalPage":1}`),
				"dataList": []map[string]any{
					honorRecordJSON(56241, "阅读之星", "审核通过"),
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	result, err := c.GetHonorList(context.Background(), "test-token", 1, 20, "")
	if err != nil {
		t.Fatalf("GetHonorList 失败: %v", err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("期望 1 条记录，实际 %d", len(result.Records))
	}
	if result.Records[0].TypeName != "阅读之星" || result.Records[0].ApprovedName != "审核通过" {
		t.Errorf("字段解析错误: %+v", result.Records[0])
	}
	if result.Page == nil || result.Page.TotalNum != 1 {
		t.Errorf("分页信息错误: %+v", result.Page)
	}
}

// TestGetHonorList_Empty 验证获取空列表。
func TestGetHonorList_Empty(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentMoralEduNew/getHonorByStudentId" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": json.RawMessage(`{"pageNo":1,"pageSize":20,"totalNum":0,"totalPage":0}`),
				"dataList": []map[string]any{},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	result, err := c.GetHonorList(context.Background(), "test-token", 1, 20, "")
	if err != nil {
		t.Fatalf("GetHonorList 失败: %v", err)
	}
	if len(result.Records) != 0 {
		t.Fatalf("期望 0 条记录，实际 %d", len(result.Records))
	}
	if result.Page == nil || result.Page.TotalPage != 0 {
		t.Errorf("分页信息错误: %+v", result.Page)
	}
}

// ─── 测试: AddHonor ───

// TestAddHonor 验证申报荣誉。
func TestAddHonor(t *testing.T) {
	var gotBody map[string]any
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentMoralEduNew/addHonor" {
			if r.Method != http.MethodPost {
				t.Errorf("期望 POST，实际 %s", r.Method)
			}
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"新增成功"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	err := c.AddHonor(context.Background(), "test-token", types.AddHonorPayload{
		Name:             "校学生优秀干部",
		TypeID:           1147,
		TypeName:         "校学生优秀干部",
		Level:            5,
		EvaluationAgency: "示例中学",
		GetDate:          "2026-06-30",
	})
	if err != nil {
		t.Fatalf("AddHonor 失败: %v", err)
	}

	if gotBody["name"] != "校学生优秀干部" {
		t.Errorf("期望 name=校学生优秀干部，实际 %v", gotBody["name"])
	}
	if gotBody["typeId"] != float64(1147) {
		t.Errorf("期望 typeId=1147，实际 %v", gotBody["typeId"])
	}
	if gotBody["level"] != float64(5) {
		t.Errorf("期望 level=5，实际 %v", gotBody["level"])
	}
}

// TestAddHonor_BizError 验证申报荣誉的业务错误。
func TestAddHonor_BizError(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentMoralEduNew/addHonor" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":-1,"msg":"荣誉名称不能为空"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	err := c.AddHonor(context.Background(), "test-token", types.AddHonorPayload{})
	if err == nil {
		t.Fatal("期望业务错误，实际 nil")
	}
	if !strings.Contains(err.Error(), "荣誉名称不能为空") {
		t.Errorf("错误消息不匹配: %v", err)
	}
}

// ─── 测试: GetHonorTypes (returnData 兼容) ───

// TestGetHonorTypes_ReturnData 验证 GetHonorTypes 也支持 returnData 路径。
func TestGetHonorTypes_ReturnData(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentMoralEduNew/getHonorType" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":       1,
				"returnData": []map[string]any{},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.GetHonorTypes(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetHonorTypes (returnData) 失败: %v", err)
	}
}

// ─── 测试: GetHonorList 分页参数 ───

// TestGetHonorList_PageParam 验证 pageNo/pageSize 参数正确传递。
func TestGetHonorList_PageParam(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentMoralEduNew/getHonorByStudentId" {
			q := r.URL.Query()
			if q.Get("pageNo") != "2" || q.Get("pageSize") != "50" {
				t.Errorf("期望 pageNo=2&pageSize=50，实际 pageNo=%s&pageSize=%s", q.Get("pageNo"), q.Get("pageSize"))
			}
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": json.RawMessage(`{"pageNo":2,"pageSize":50,"totalNum":0,"totalPage":0}`),
				"dataList": []map[string]any{},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.GetHonorList(context.Background(), "test-token", 2, 50, "")
	if err != nil {
		t.Fatalf("GetHonorList 分页失败: %v", err)
	}
}

// ─── 测试: GetHonorTypes 空结果 ───

// TestGetHonorTypes_Empty 验证空列表正常返回。
func TestGetHonorTypes_Empty(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentMoralEduNew/getHonorType" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"dataList": []map[string]any{},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	types, err := c.GetHonorTypes(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetHonorTypes 失败: %v", err)
	}
	if types == nil {
		t.Fatal("期望空切片，实际 nil")
	}
	if len(types) != 0 {
		t.Fatalf("期望 0 个类型，实际 %d", len(types))
	}
}

// ─── 测试: UpdateHonor typeName 自动补全 ───

// TestUpdateHonor_AutoFillTypeName 验证 UpdateHonor 在有 typeId 无 typeName 时
// 自动反查 GetHonorTypeOptions（dataList）补全 typeName，与 AddHonor 对称。
func TestUpdateHonor_AutoFillTypeName(t *testing.T) {
	var gotBody map[string]any
	var optionsCalled bool
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentMoralEduNew/getHonorTypeForSelect":
			optionsCalled = true
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code": 1,
				// dataList 才是荣誉类型选项（与 AddHonor 一致）
				"dataList": []map[string]any{
					{"label": "校三好学生", "value": 1147},
					{"label": "校学生优秀干部", "value": 1148},
				},
				// returnData 是等级选项，UpdateHonor 不应读这里
				"returnData": []map[string]any{
					{"label": "校", "value": 5},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		case "/api/studentMoralEduNew/updateHonor":
			if r.Method != http.MethodPost {
				t.Errorf("期望 POST，实际 %s", r.Method)
			}
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"更新成功"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	err := c.UpdateHonor(context.Background(), "test-token", map[string]any{
		"id":     56241,
		"typeId": 1147,
		// 故意不传 typeName，期望 SDK 自动补全
		"level":            5,
		"evaluationAgency": "示例中学",
		"getDate":          "2026-06-30",
		"name":             "校三好学生",
	})
	if err != nil {
		t.Fatalf("UpdateHonor 失败: %v", err)
	}
	if !optionsCalled {
		t.Fatal("期望调用 getHonorTypeForSelect 反查 typeName，实际未调用")
	}
	if gotBody["typeName"] != "校三好学生" {
		t.Errorf("期望 typeName=校三好学生，实际 %v", gotBody["typeName"])
	}
	if gotBody["typeId"] != float64(1147) {
		t.Errorf("期望 typeId=1147，实际 %v", gotBody["typeId"])
	}
}

// TestUpdateHonor_SkipAutoFillWhenTypeNamePresent 已有 typeName 时不应再反查。
func TestUpdateHonor_SkipAutoFillWhenTypeNamePresent(t *testing.T) {
	var optionsCalled bool
	var gotBody map[string]any
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentMoralEduNew/getHonorTypeForSelect":
			optionsCalled = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"dataList":[]}`))
			return
		case "/api/studentMoralEduNew/updateHonor":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"更新成功"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	err := c.UpdateHonor(context.Background(), "test-token", map[string]any{
		"id":       56241,
		"typeId":   1147,
		"typeName": "已有名称",
	})
	if err != nil {
		t.Fatalf("UpdateHonor 失败: %v", err)
	}
	if optionsCalled {
		t.Fatal("已有 typeName 时不应调用 getHonorTypeForSelect")
	}
	if gotBody["typeName"] != "已有名称" {
		t.Errorf("期望保留 typeName=已有名称，实际 %v", gotBody["typeName"])
	}
}

// TestAddHonor_NameDefaultsToTypeName 前端新增表单不传 name，只传 typeName；
// SDK 在 Name 为空时用 TypeName 填 name，与页面行为对齐。
func TestAddHonor_NameDefaultsToTypeName(t *testing.T) {
	var gotBody map[string]any
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentMoralEduNew/addHonor" {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"新增成功"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	err := c.AddHonor(context.Background(), "test-token", types.AddHonorPayload{
		// Name 故意为空
		TypeID:           1147,
		TypeName:         "校学生优秀干部",
		Level:            5,
		EvaluationAgency: "示例中学",
		GetDate:          "2026-06-30",
	})
	if err != nil {
		t.Fatalf("AddHonor 失败: %v", err)
	}
	if gotBody["name"] != "校学生优秀干部" {
		t.Errorf("期望 name 回落为 typeName=校学生优秀干部，实际 %v", gotBody["name"])
	}
	if gotBody["typeName"] != "校学生优秀干部" {
		t.Errorf("期望 typeName=校学生优秀干部，实际 %v", gotBody["typeName"])
	}
}

// TestGetHonorList_StatusField 验证列表响应 status 整型进入 HonorRecord。
func TestGetHonorList_StatusField(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentMoralEduNew/getHonorByStudentId" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": json.RawMessage(`{"pageNo":1,"pageSize":20,"totalNum":1,"totalPage":1}`),
				"dataList": []map[string]any{
					{
						"id":                int64(1),
						"type_name":         "校三好学生",
						"level_name":        "校",
						"level":             5,
						"dimension_name":    "思想品德",
						"status":            0,
						"statusName":        "未审核",
						"get_date":          "2026-06-30",
						"evaluation_agency": "示例中学",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	result, err := c.GetHonorList(context.Background(), "test-token", 1, 20, "")
	if err != nil {
		t.Fatalf("GetHonorList 失败: %v", err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("期望 1 条，实际 %d", len(result.Records))
	}
	if result.Records[0].Status != 0 {
		t.Errorf("期望 Status=0，实际 %d", result.Records[0].Status)
	}
	if result.Records[0].ApprovedName != "未审核" {
		t.Errorf("期望 ApprovedName=未审核，实际 %q", result.Records[0].ApprovedName)
	}
}

// TestGetHonorList_KeyParam 验证 key 查询参数正确透传（含空格，经 QueryEscape）。
func TestGetHonorList_KeyParam(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentMoralEduNew/getHonorByStudentId" {
			q := r.URL.Query()
			if q.Get("key") != "三好 学生" {
				t.Errorf("期望 key=三好 学生，实际 key=%q", q.Get("key"))
			}
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": json.RawMessage(`{"pageNo":1,"pageSize":20,"totalNum":0,"totalPage":0}`),
				"dataList": []map[string]any{},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.GetHonorList(context.Background(), "test-token", 1, 20, "三好 学生")
	if err != nil {
		t.Fatalf("GetHonorList key 透传失败: %v", err)
	}
}
