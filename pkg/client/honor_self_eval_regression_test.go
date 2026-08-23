package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestDeleteHonor_UsesGETWithIDQuery 验证删除荣誉使用 GET，并通过 id 查询参数传递记录 ID。
func TestDeleteHonor_UsesGETWithIDQuery(t *testing.T) {
	requestSeen := 0
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		requestSeen++
		if r.URL.Path != "/api/studentMoralEduNew/deleteHonorById" {
			t.Errorf("期望路径 deleteHonorById，得到 %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("期望 GET，得到 %s", r.Method)
		}
		if got := r.URL.Query().Get("id"); got != "56241" {
			t.Errorf("期望 id=56241，得到 %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"删除成功"}`))
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	if err := c.DeleteHonor(context.Background(), "test-token", 56241); err != nil {
		t.Fatalf("DeleteHonor 失败: %v", err)
	}
	if requestSeen != 1 {
		t.Fatalf("期望业务请求恰好调用一次，实际调用 %d 次", requestSeen)
	}
}

// TestSubmitSelfEvaluationStructured_EncodesStudentCommentAsJSONString 验证结构化自评把表单 JSON 字符串放入 studentComment。
func TestSubmitSelfEvaluationStructured_EncodesStudentCommentAsJSONString(t *testing.T) {
	called := 0
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		called++
		if r.URL.Path != "/api/studentMoralEduNew/addSelfEvaluation" {
			t.Errorf("期望路径 addSelfEvaluation，得到 %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST，得到 %s", r.Method)
		}

		var outer map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&outer); err != nil {
			t.Fatalf("解析外层请求体失败: %v", err)
		}
		studentCommentRaw, ok := outer["studentComment"]
		if !ok {
			t.Fatal("请求体缺少 studentComment")
		}
		var studentComment string
		if err := json.Unmarshal(studentCommentRaw, &studentComment); err != nil {
			t.Fatalf("studentComment 应为 JSON 字符串: %v", err)
		}
		var gotForm map[string]any
		if err := json.Unmarshal([]byte(studentComment), &gotForm); err != nil {
			t.Fatalf("studentComment 应包含可解码的表单 JSON: %v", err)
		}
		if gotForm["bxqhzr"] != "本学期会做人目标" {
			t.Errorf("期望 bxqhzr=本学期会做人目标，得到 %v", gotForm["bxqhzr"])
		}
		if gotForm["sxqhzr"] != "下学期会做人目标" {
			t.Errorf("期望 sxqhzr=下学期会做人目标，得到 %v", gotForm["sxqhzr"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"提交成功"}`))
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	err := c.SubmitSelfEvaluationStructured(context.Background(), "test-token", map[string]any{
		"bxqhzr": "本学期会做人目标",
		"sxqhzr": "下学期会做人目标",
	})
	if err != nil {
		t.Fatalf("SubmitSelfEvaluationStructured 失败: %v", err)
	}
	if called != 1 {
		t.Fatalf("期望业务请求恰好调用一次，实际调用 %d 次", called)
	}
}
