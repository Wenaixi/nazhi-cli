package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestBuildTaskPayload_PictureLimitRejectsBeforeNetwork 锁定「图片超限」是纯本地
// 判定：必须在任何网络请求（含任务元数据预取）发出之前被拒绝。
//
// 背景：countValidImages 只读入参，不依赖任务元数据。若它排在
// GetCircleTypeByTaskID 之后，元数据接口失败（网络/服务端问题）会先返回，
// 把「图片超限」这一调用方输入错误掩盖成网络/服务端错误——CLI 漏斗随之
// 从 400/exit3 漂移到 502/exit2，脚本的重试决策被误导。
//
// 本测试让元数据端点计数：若预检仍排在网络请求之后，错误会是
// 「获取任务元数据失败」而非「图片最多 2 张」，且 metaCalls > 0，断言变红。
func TestBuildTaskPayload_PictureLimitRejectsBeforeNetwork(t *testing.T) {
	var metaCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","schoolName":"测试中学"}}`))
		case "/api/studentCircleNew/getCircleTypeByTaskId":
			// 元数据端点：任何命中都说明预检排在了网络请求之后。
			metaCalls.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":1}`))
		default:
			t.Errorf("图片超限不应触达其它业务端点: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":1}`))
		}
	}))
	defer srv.Close()

	c, err := New(
		WithBaseURL(srv.URL),
		WithSSOBase(srv.URL),
		WithUploadURL(srv.URL),
		WithTimeout(5*1000*1000*1000),
	)
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer func() { _ = c.Close() }()

	input := types.TaskSubmitInput{
		TaskID:     2001,
		Content:    "c",
		Hours:      "2",
		ImagePaths: []string{"a.jpg", "b.jpg", "c.jpg"}, // 3 张 > 上限 2
	}

	_, err = c.buildTaskPayload(context.Background(), "tok", input, "TestBuildTaskPayload", nil)
	if err == nil {
		t.Fatal("3 张图片应返回错误，实际 err 为 nil")
	}
	if !strings.Contains(err.Error(), "图片最多 2 张") {
		t.Fatalf("应报图片超限，实际错误: %v", err)
	}
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("应 errors.Is(ErrInvalidPayload)，实际: %v", err)
	}
	// 核心断言：预检先于网络——元数据端点零命中。
	if n := metaCalls.Load(); n != 0 {
		t.Errorf("图片超限应在任何网络请求之前被拒绝，元数据端点却收到 %d 次请求", n)
	}
}
