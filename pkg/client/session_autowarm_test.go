// Package client_test 外部黑盒测试。
package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestSubmitTask_AutoActivatesSession 回归测试：SubmitTask 必须在请求前
// 自动调用 ActivateSession 完成 4 步预热。
//
// 历史 bug：SubmitTask / GetDimensions / GetCircleTypeByTaskId / GetMyInfo /
// QuerySelfEvaluation / QuerySelfGradEvaluation / SubmitSelfEvaluation 都
// 跳过 session 预热，HAR 验证的 4 步序列（/ → getMenu → getMenu → getMyInfo）
// 没跑，后续接口返回空数据且 code=1 静默"成功"。
func TestSubmitTask_AutoActivatesSession(t *testing.T) {
	var (
		mu        sync.Mutex
		callOrder []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callOrder = append(callOrder, r.Method+" "+r.URL.Path)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"ok","returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"ok","returnData":{"name":"测试用户"}}`))
		case "/api/studentCircleNew/addCircle":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"ok","returnData":null}`))
		default:
			t.Errorf("未预期请求路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c, _ := client.New(
		client.WithSSOBase(srv.URL),
		client.WithBaseURL(srv.URL),
		client.WithTimeout(5*time.Second),
		client.WithCustomOCR(&mockOCR{text: "AB12"}),
	)

	// 关键：直接调 SubmitTask，**不**先调 FetchTasks
	_, err := c.SubmitTask(context.Background(), "test-token", types.TaskSubmitPayload{
		CircleTaskID: 123,
		CircleTypeID: 456,
	})
	if err != nil {
		t.Fatalf("SubmitTask 失败: %v", err)
	}

	expected := []string{
		"GET /",                                // 步骤 1
		"GET /api/studentInfo/getMenu",         // 步骤 2
		"GET /api/studentInfo/getMenu",         // 步骤 3
		"GET /api/studentInfo/getMyInfo",       // 步骤 4
		"POST /api/studentCircleNew/addCircle", // SubmitTask 实际请求
	}
	if !reflect.DeepEqual(callOrder, expected) {
		t.Errorf("调用顺序错误（session 预热缺失或顺序错）\n实际: %v\n期望: %v", callOrder, expected)
	}
}
