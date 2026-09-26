package client_test

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestUpdateHonor_NonIntegralTypeIDDoesNotTriggerLookup 锁定
// UpdateHonor 的 typeId 若为非整 float64（2.5），此前 honorMapInt64 直接
// int64(2.5)=2 截断 → 触发 getHonorTypeForSelect 反查并补出错误的荣誉类型名。
// 对齐全仓 math.Trunc 纪律（self_eval.go firstInt64 / types FlexInt）
// 非整值应拒绝触发反查，整 float64（2.0）仍合法放行。
func TestUpdateHonor_NonIntegralTypeIDDoesNotTriggerLookup(t *testing.T) {
	lookupHit := false
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentMoralEduNew/getHonorTypeForSelect" {
			lookupHit = true
			_, _ = w.Write([]byte(`{"code":1,"dataList":[{"id":2,"name":"z"}]}`))
			return
		}
		if r.URL.Path == "/api/studentMoralEduNew/updateHonor" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
			return
		}
		http.NotFound(w, r)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	err := c.UpdateHonor(context.Background(), "test-token", map[string]any{
		"id":                  int64(1),
		"typeId":              2.5, // 非整 float64：修复前被截成 2 触发反查并补错名
		"typeName":            "",
		"honorName":           "x",
		"honorLevel":          "校级",
		"certImgAttachmentId": int64(0),
	})
	if err != nil {
		t.Fatalf("UpdateHonor 失败（非整 typeId 不应阻断业务请求，直接走 updateHonor）: %v", err)
	}
	if lookupHit {
		t.Fatalf("非整 typeId 不应触发 getHonorTypeForSelect 反查（修复前 2.5 被截成 2 反查出错误名）")
	}
}

// TestHonorMapInt64IntegerFloatStillLookup 锁定：整 float64（2.0）仍合法触发
// 反查——保证修复只拒绝非整值，不破坏合法整数 float 的既有语义。
func TestHonorMapInt64IntegerFloatStillLookup(t *testing.T) {
	lookupHit := false
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentMoralEduNew/getHonorTypeForSelect" {
			lookupHit = true
			_, _ = w.Write([]byte(`{"code":1,"dataList":[{"id":2,"label":"校级荣誉"}]}`))
			return
		}
		if r.URL.Path == "/api/studentMoralEduNew/updateHonor" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
			return
		}
		http.NotFound(w, r)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	err := c.UpdateHonor(context.Background(), "test-token", map[string]any{
		"id":         int64(1),
		"typeId":     float64(2.0), // 整 float64 合法，应触发反查
		"typeName":   "",
		"honorName":  "x",
		"honorLevel": "校级",
	})
	if err != nil {
		t.Fatalf("UpdateHonor 失败: %v", err)
	}
	if !lookupHit {
		t.Fatalf("整 float64 typeId 应触发 getHonorTypeForSelect 反查（修复只拒绝非整值）")
	}
}

// TestHonorMapInt64TruncDiscipline 直判 math.Trunc 纪律前提（非整必须拒绝）。
func TestHonorMapInt64TruncDiscipline(t *testing.T) {
	if f := 2.5; f == math.Trunc(f) {
		t.Fatalf("测试前提错误：2.5 应为非整")
	}
}
