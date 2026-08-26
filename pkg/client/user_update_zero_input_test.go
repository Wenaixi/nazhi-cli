package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestUpdateMyInfoStructured_ZeroInputNoOp 锁定 USER-1 契约：
// 全零输入（如 CLI --payload '{}'）应视为 no-op——不发仅含 studentUuid 的空 POST，
// 不失效本地缓存。此前实现无条件发 {"studentUuid":""} 且 InvalidateCachedUserInfo。
func TestUpdateMyInfoStructured_ZeroInputNoOp(t *testing.T) {
	var posts atomic.Int32

	biz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","schoolName":"测试中学"}}`))
		case "/api/studentInfo/updateMyInfo":
			posts.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	// 先激活填充缓存
	_, err = c.GetMyInfo(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetMyInfo 失败: %v", err)
	}

	// 全零输入
	err = c.UpdateMyInfoStructured(context.Background(), "test-token", types.UserUpdateInput{})
	if err != nil {
		t.Fatalf("UpdateMyInfoStructured 全零输入应返回 nil，实际 %v", err)
	}
	if got := posts.Load(); got != 0 {
		t.Errorf("全零输入不应发出 updateMyInfo 请求，实际 %d 次", got)
	}

	// 缓存不应被失效（fast path 命中缓存，不再发 getMyInfo）
	info, err := c.GetMyInfo(context.Background(), "test-token")
	if err != nil || info == nil {
		t.Fatalf("缓存应保留，实际 info=%v err=%v", info, err)
	}
}
