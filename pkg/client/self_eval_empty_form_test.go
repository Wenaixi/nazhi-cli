package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestSubmitSelfEvaluationStructured_RejectsEmptyForm 回归测试：
// 空表单（nil 或 {}）必须在发出任何请求前被拒绝。
//
// 背景（十三域审计 P2-I）：原实现直接序列化入参，Go 调用方传 nil/{} 时
// 会静默发出 {"studentComment":"{}"} 的合法但空载荷请求，服务端 code=1
// 即记为一次成功提交。前端用户面对的是已渲染的 11 个输入框，
// 全空提交在浏览器是可见操作；脚本空载荷则无声产生脏数据。
func TestSubmitSelfEvaluationStructured_RejectsEmptyForm(t *testing.T) {
	var bizCalls int
	biz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bizCalls++
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte("<html>home</html>"))
		case "/api/studentInfo/getMenu":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三"}}`))
		case "/api/studentMoralEduNew/addSelfEvaluation":
			t.Error("空表单不应到达 addSelfEvaluation")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		default:
			t.Errorf("意外路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	for name, form := range map[string]map[string]any{
		"nil表单":  nil,
		"空map表单": {},
	} {
		err := c.SubmitSelfEvaluationStructured(context.Background(), "test-token", form)
		if !errors.Is(err, ErrInvalidPayload) {
			t.Errorf("%s 应返回 ErrInvalidPayload，实际: %v", name, err)
		}
	}
	if bizCalls > 3 {
		// 预热 3 次（每轮 submit 预热）+ 0 次业务请求为正确形态；
		// 这里只防御业务请求误发的回归。
		t.Logf("biz 请求数 %d（含预热）", bizCalls)
	}
}
