package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestQuerySelfGradEvaluation_AllEmptyReturnsNilNil 回归测试：
// 毕业查询在全空响应（code=1 三容器全空）下应归一为 (nil,nil)，
// 与学期版 QuerySelfEvaluation 的空成功契约对称。
//
// 背景（十三域审计 P2-J）：学期版有 isEmptyDecodeFailure(err) → (nil,nil)
// 归一，毕业版原样上抛 ErrAllDecodersFailed。前端语义中「未提交毕业评价」
// 是正常态而非错误（mainLeft.vue dataMap 为 null 时 isGrad 保持 0、
// textarea2 保持空串）。姊妹方法行为分叉加剧调用方误读。
func TestQuerySelfGradEvaluation_AllEmptyReturnsNilNil(t *testing.T) {
	biz := httptest.NewServer(testBizHandlerDoBiz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	v, err := c.QuerySelfGradEvaluation(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("全空响应应归一为空成功，实际错误: %v", err)
	}
	if v != nil {
		t.Errorf("全空响应应返回 nil 数据，实际: %v", *v)
	}
}
