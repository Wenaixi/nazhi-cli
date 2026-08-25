package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestDoBizAndDecode_BadJSONHasInvalidResponseSentinel 回归测试：
// 200 + 非 JSON body（nginx 维护页 / WAF 挑战页）的解码失败必须携带
// ErrInvalidResponse 哨兵。
//
// 背景（十三域审计 P2-F）：非 2xx 分支经 classifyHTTPStatus 恒有哨兵，
// 唯独「服务端异常但状态码说谎」的 200+HTML 路径走裸 fmt.Errorf 包装，
// SDK 用户按文档推荐的 errors.Is(err, ErrInvalidResponse) 判定落空——
// 同一台服务器 502+HTML 有哨兵而 200+HTML 没有，分类体系在此失效。
func TestDoBizAndDecode_BadJSONHasInvalidResponseSentinel(t *testing.T) {
	biz := httptest.NewServer(testBizHandlerDoBiz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>维护中</html>"))
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	_, err = c.doBizAndDecode(context.Background(), "test-token", "TestOp", "/api/test", http.MethodGet, nil)
	if err == nil {
		t.Fatal("期望解析错误，但得到 nil")
	}
	if !errors.Is(err, ErrInvalidResponse) {
		t.Errorf("200+非 JSON 解码失败应包装 ErrInvalidResponse，实际: %v", err)
	}
}
