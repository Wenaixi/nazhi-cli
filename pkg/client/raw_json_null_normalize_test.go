// raw_json_null_normalize_test.go — 锁定原始 JSON 路径对 dataList null 形态的归一契约。
//
// 背景（Cycle 94 cli F1）：rawListBytes 只做 json.Valid 校验，null 形态存在漏网：
//   - 字面 dataList:null 在 json.Unmarshal 到 *json.RawMessage 时已变 nil 指针，
//     走 rawListBytes 的分支 1 返回 nil（这条原本安全）；
//   - 字符串 dataList:"null"（RawMessage 为带引号 6 字节）json.Valid 通过，
//     fetchCirclePageJSON 的 []json.RawMessage 校验会整页报 ErrInvalidResponse
//     （full 模式），而 limit 模式却能归一 [] —— 同一空数据两套行为；
//   - GetHonorTypesJSON 对字符串 "null" 原样透传 envelope.data:"null"，破坏 jq。
//
// 契约（文件头）：成功路径恒为合法 JSON 数组，dataList null 时归一为 []。
package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// circlePageNullBody 构造 getStudentCircle 响应，dataList 用给定原始值。
func circlePageNullBody(dataList string) string {
	return `{"code":1,"dataList":` + dataList + `,"pageBean":{"pageNo":1,"pageSize":500,"totalNum":0,"totalPage":1}}`
}

// TestGetSubmittedCirclesJSON_StringNullDataListNormalized（RED）
// 服务端返回 dataList:"null"（字符串形态，json.Valid 通过）时，full 拉取路径
// 必须归一为 []，而不是整页报 ErrInvalidResponse 或透传 "null"。
func TestGetSubmittedCirclesJSON_StringNullDataListNormalized(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/getStudentCircle" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(circlePageNullBody(`"null"`)))
	})))
	defer biz.Close()

	c, err := client.New(client.WithBaseURL(biz.URL), client.WithSSOBase(biz.URL), client.WithUploadURL(biz.URL))
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	raw, err := c.GetSubmittedCirclesJSON(context.Background(), "test-token", "")
	if err != nil {
		t.Fatalf("dataList 为 null 形态不应报错（空数据归 []），实际 err=%v", err)
	}
	if got := string(raw); got != "[]" {
		t.Fatalf("dataList null 形态应归一为 []，实际 %s", got)
	}
}

// TestGetSubmittedCirclesJSON_LiteralNullDataListNormalized（行为锁定）
// 字面 dataList:null 同样归一为 []（解码层已变 nil 指针，回归护栏）。
func TestGetSubmittedCirclesJSON_LiteralNullDataListNormalized(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/getStudentCircle" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(circlePageNullBody(`null`)))
	})))
	defer biz.Close()

	c, err := client.New(client.WithBaseURL(biz.URL), client.WithSSOBase(biz.URL), client.WithUploadURL(biz.URL))
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	raw, err := c.GetSubmittedCirclesJSON(context.Background(), "test-token", "")
	if err != nil {
		t.Fatalf("字面 dataList:null 不应报错，实际 err=%v", err)
	}
	if got := string(raw); got != "[]" {
		t.Fatalf("字面 dataList:null 应归一为 []，实际 %s", got)
	}
}

// TestGetHonorTypesJSON_StringNullDataListFallsBack（RED）
// 字符串形态 "null" 同样表示空列表：GetHonorTypesJSON 应走 returnData fallback，
// 无 returnData 时返回 []，而不是把带引号 "null" 原样透传。
func TestGetHonorTypesJSON_StringNullDataListFallsBack(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentMoralEduNew/getHonorType" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"dataList":"null","returnData":null}`))
	})))
	defer biz.Close()

	c, err := client.New(client.WithBaseURL(biz.URL), client.WithSSOBase(biz.URL), client.WithUploadURL(biz.URL))
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	raw, err := c.GetHonorTypesJSON(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetHonorTypesJSON: %v", err)
	}
	if got := string(raw); got != "[]" {
		t.Fatalf("字符串 null 应归一为 []（或 fallback returnData），实际 %s", got)
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("结果必须是合法 JSON 数组: %v", err)
	}
}
