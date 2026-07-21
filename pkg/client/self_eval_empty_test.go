package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestQuerySelfEvaluation_EmptySuccess 回归：未提交评价时服务端返回 code=1
// 且 returnData/dataMap/dataList 均为空，应返回 (nil, nil) 而非
// 「所有解码器均失败」错误。
//
// 与 QuerySelfEvaluationJSON 空数据契约对齐（raw_json.go 返回 nil,nil）。
func TestQuerySelfEvaluation_EmptySuccess(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentMoralEduNew/querySelfEvaluation" {
			t.Errorf("期望路径 querySelfEvaluation, 得到 %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// 业务成功但无评价数据（未提交）
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	status, err := c.QuerySelfEvaluation(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("未提交评价应返回 (nil,nil)，实际 err=%v", err)
	}
	if status != nil {
		t.Fatalf("未提交评价应返回 status=nil，实际 %+v", status)
	}
}

// TestQuerySelfEvaluation_EmptyReturnDataNull 空 returnData:null 同样是空成功。
func TestQuerySelfEvaluation_EmptyReturnDataNull(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentMoralEduNew/querySelfEvaluation" {
			t.Errorf("期望路径 querySelfEvaluation, 得到 %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":null,"dataMap":null,"dataList":null}`))
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	status, err := c.QuerySelfEvaluation(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("null 容器应返回 (nil,nil)，实际 err=%v", err)
	}
	if status != nil {
		t.Fatalf("null 容器应返回 status=nil，实际 %+v", status)
	}
}

// TestQuerySelfEvaluation_EmptyObjectMap 空对象 {} 归一化后为 nil，也应空成功。
func TestQuerySelfEvaluation_EmptyObjectMap(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentMoralEduNew/querySelfEvaluation" {
			t.Errorf("期望路径 querySelfEvaluation, 得到 %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"returnData":{}}`))
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	status, err := c.QuerySelfEvaluation(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("空对象 returnData 应返回 (nil,nil)，实际 err=%v", err)
	}
	if status != nil {
		t.Fatalf("空对象 returnData 应返回 status=nil，实际 %+v", status)
	}
}

// TestQuerySelfEvaluation_HasData 有评价内容时正常返回，避免空态修复误吞真实数据。
func TestQuerySelfEvaluation_HasData(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentMoralEduNew/querySelfEvaluation" {
			t.Errorf("期望路径 querySelfEvaluation, 得到 %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"returnData":{"id":42,"studentComment":"本学期表现良好","teacherComment":"继续加油"}}`))
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	status, err := c.QuerySelfEvaluation(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("有数据应成功，实际 err=%v", err)
	}
	if status == nil {
		t.Fatal("有数据应返回非 nil status")
	}
	if status.ID != 42 {
		t.Errorf("ID 期望 42, 得到 %d", status.ID)
	}
	if status.StudentComment != "本学期表现良好" {
		t.Errorf("StudentComment 期望「本学期表现良好」, 得到 %q", status.StudentComment)
	}
	if status.TeacherComment != "继续加油" {
		t.Errorf("TeacherComment 期望「继续加油」, 得到 %q", status.TeacherComment)
	}
}
