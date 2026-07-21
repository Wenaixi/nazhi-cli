package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// 本文件覆盖「用户输入 vs SDK 自动填充」原则的 TDD 用例：
//   - 典型案例：type/role/level 代码 → 自动 typeName/roleName/levelName
//   - 荣誉新增：score 默认 0（前端 form 无 v-model）
//   - 荣誉类型：getHonorType 的 snake_case 字段

// TestAddTypicalCase_AutoFillDisplayNames 只传代码时自动补展示名。
func TestAddTypicalCase_AutoFillDisplayNames(t *testing.T) {
	var gotBody map[string]any
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/addTypicalCase" {
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
		Title:       "仅代码不传展示名",
		Type:        "1",
		TeacherName: "王老师",
		PartnerName: "同学甲",
		Role:        "1",
		Remark:      "备注",
		Content:     "正文内容",
		Level:       "5",
	})
	if err != nil {
		t.Fatalf("AddTypicalCase 失败: %v", err)
	}
	if gotBody["typeName"] != "研究性学习报告" {
		t.Errorf("期望 typeName=研究性学习报告，实际 %v", gotBody["typeName"])
	}
	if gotBody["roleName"] != "负责人" {
		t.Errorf("期望 roleName=负责人，实际 %v", gotBody["roleName"])
	}
	if gotBody["levelName"] != "学校" {
		t.Errorf("期望 levelName=学校，实际 %v", gotBody["levelName"])
	}
}

// TestAddTypicalCase_ExplicitNamesPreserved 已填 *Name 时不覆盖。
func TestAddTypicalCase_ExplicitNamesPreserved(t *testing.T) {
	var gotBody map[string]any
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/addTypicalCase" {
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
		Title:       "自定义名",
		Type:        "2",
		TypeName:    "自定义类别名",
		Role:        "2",
		RoleName:    "自定义角色",
		Level:       "3",
		LevelName:   "自定义级别",
		Content:     "x",
		TeacherName: "t",
		PartnerName: "p",
		Remark:      "r",
	})
	if err != nil {
		t.Fatalf("AddTypicalCase 失败: %v", err)
	}
	if gotBody["typeName"] != "自定义类别名" {
		t.Errorf("期望保留 typeName，实际 %v", gotBody["typeName"])
	}
	if gotBody["roleName"] != "自定义角色" {
		t.Errorf("期望保留 roleName，实际 %v", gotBody["roleName"])
	}
	if gotBody["levelName"] != "自定义级别" {
		t.Errorf("期望保留 levelName，实际 %v", gotBody["levelName"])
	}
}

// TestAddTypicalCase_FrontendOptionLabels 对齐 classiccanter.vue el-option：
// type "2"→社会调查报告；level "1"→国际（非写实列表的「国家」文案）。
func TestAddTypicalCase_FrontendOptionLabels(t *testing.T) {
	var gotBody map[string]any
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/addTypicalCase" {
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
		Title:       "对齐前端 option",
		Type:        "2",
		TeacherName: "t",
		PartnerName: "p",
		Role:        "2",
		Remark:      "r",
		Content:     "c",
		Level:       "1",
	})
	if err != nil {
		t.Fatalf("AddTypicalCase 失败: %v", err)
	}
	if gotBody["typeName"] != "社会调查报告" {
		t.Errorf("type=2 typeName: got %v，期望 社会调查报告（classiccanter el-option）", gotBody["typeName"])
	}
	if gotBody["roleName"] != "参与者" {
		t.Errorf("role=2 roleName: got %v", gotBody["roleName"])
	}
	if gotBody["levelName"] != "国际" {
		t.Errorf("level=1 levelName: got %v，期望 国际（非「国家」）", gotBody["levelName"])
	}
}

// TestUpdateTypicalCase_AutoFillWithNumericCodes 列表响应里 type/role/level 常为 number；
// 更新 map 路径必须与 string 代码一样能补 *Name（对齐 openUpdate 回填后改选场景）。
func TestUpdateTypicalCase_AutoFillWithNumericCodes(t *testing.T) {
	var gotBody map[string]any
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/updateTypicalCase" {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	err := c.UpdateTypicalCase(context.Background(), "test-token", map[string]any{
		"id":          int64(99),
		"title":       "数字代码更新",
		"type":        2, // 列表 JSON number
		"role":        float64(2),
		"level":       1,
		"teacherName": "t",
		"partnerName": "p",
		"remark":      "r",
		"content":     "c",
	})
	if err != nil {
		t.Fatalf("UpdateTypicalCase 失败: %v", err)
	}
	if gotBody["typeName"] != "社会调查报告" {
		t.Errorf("type=2(number) typeName: got %v，期望 社会调查报告", gotBody["typeName"])
	}
	if gotBody["roleName"] != "参与者" {
		t.Errorf("role=2(number) roleName: got %v", gotBody["roleName"])
	}
	if gotBody["levelName"] != "国际" {
		t.Errorf("level=1(number) levelName: got %v，期望 国际", gotBody["levelName"])
	}
}

// TestAddHonor_ScoreDefaultsToZero 前端 score 默认 0 且无输入；请求体应带 score。
func TestAddHonor_ScoreDefaultsToZero(t *testing.T) {
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
		TypeID:           1147,
		TypeName:         "校三好学生",
		Level:            5,
		EvaluationAgency: "示例中学",
		GetDate:          "2026-06-30",
	})
	if err != nil {
		t.Fatalf("AddHonor 失败: %v", err)
	}
	score, ok := gotBody["score"]
	if !ok {
		t.Fatal("期望请求体含 score 字段")
	}
	if score != float64(0) && score != 0 {
		t.Errorf("期望 score=0，实际 %v (%T)", score, score)
	}
}

// TestAddHonor_ExplicitScorePreserved 显式 score 保留。
func TestAddHonor_ExplicitScorePreserved(t *testing.T) {
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
		TypeID:           1,
		TypeName:         "测试",
		Level:            5,
		EvaluationAgency: "校",
		GetDate:          "2026-01-01",
		Score:            8,
	})
	if err != nil {
		t.Fatalf("AddHonor 失败: %v", err)
	}
	if gotBody["score"] != float64(8) {
		t.Errorf("期望 score=8，实际 %v", gotBody["score"])
	}
}

// TestGetHonorTypes_FrontendSnakeFields 德育说明表 dimension_name/level_name/score。
func TestGetHonorTypes_FrontendSnakeFields(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentMoralEduNew/getHonorType" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code": 1,
				"dataList": []map[string]any{
					{
						"id":             10,
						"name":           "校三好学生",
						"dimension_name": "思想品德",
						"level_name":     "校",
						"level":          5,
						// 平台真实类型为展示字符串，不是 number
						"score": "分数：+5.0",
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
	typesList, err := c.GetHonorTypes(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetHonorTypes 失败: %v", err)
	}
	if len(typesList) != 1 {
		t.Fatalf("期望 1 条，实际 %d", len(typesList))
	}
	ht := typesList[0]
	if ht.DimensionName != "思想品德" {
		t.Errorf("DimensionName: got %q，期望解码 dimension_name", ht.DimensionName)
	}
	if ht.LevelName != "校" {
		t.Errorf("LevelName: got %q，期望解码 level_name", ht.LevelName)
	}
	if ht.Score != "分数：+5.0" {
		t.Errorf("Score: got %q，期望展示文案", ht.Score)
	}
}

// TestUpdateMyInfoStructured_SkipsNationalStudentNumber 前端全国学号只读；
// Structured 即使 input 带了也不写入请求体。
func TestUpdateMyInfoStructured_SkipsNationalStudentNumber(t *testing.T) {
	var gotBody map[string]any
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentInfo/updateMyInfo" {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
			return
		}
		// session 预热
		if r.URL.Path == "/" || r.URL.Path == "/api/studentInfo/getMenu" || r.URL.Path == "/api/studentInfo/getMyInfo" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"测"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	err := c.UpdateMyInfoStructured(context.Background(), "test-token", types.UserUpdateInput{
		Telephone:             "13800138000",
		NationalStudentNumber: "不应出现在请求体",
	})
	if err != nil {
		t.Fatalf("UpdateMyInfoStructured 失败: %v", err)
	}
	if _, ok := gotBody["nationalStudentNumber"]; ok {
		t.Errorf("不应发送 nationalStudentNumber，实际 body=%v", gotBody)
	}
	if gotBody["telephone"] != "13800138000" {
		t.Errorf("期望 telephone 透传，实际 %v", gotBody["telephone"])
	}
}

// 防止未使用 client 包名被 goimports 误删时的编译提示（newTestClient 已用）。
var _ = client.ErrInvalidPayload
