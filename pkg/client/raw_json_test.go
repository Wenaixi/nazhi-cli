// raw_json_test.go 验证 *JSON 方法族的语义。
//
// 核心断言：
//   - GetSubmittedCirclesJSON 自动跨页合并后，dataList 仍是合法 JSON 数组
//   - GetSubmittedCirclesJSON ctx 取消时返回 (部分合并字节, ctx.Err())
//   - GetHonorTypesJSON 在 dataList 为空时自动 fallback 到 returnData
//   - QuerySelfEvaluationJSON 在 returnData 为字符串/token 时 fallback 到 dataList[0]
//   - GetHonorListJSON 同时返回 dataList 与 pageBean 原始字节
//   - FetchTasksJSON 部分维度失败时返回 (已合并字节, partial error)
package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// rawHonorListBody 构造 getHonorByStudentId 响应，含 records + pageBean。
func rawHonorListBody(pageNo, pageSize, totalNum, totalPage int, records any) string {
	b, _ := json.Marshal(map[string]any{
		"code":     1,
		"msg":      "成功",
		"dataList": records,
		"pageBean": map[string]any{
			"pageNo":    pageNo,
			"pageSize":  pageSize,
			"totalNum":  totalNum,
			"totalPage": totalPage,
		},
	})
	return string(b)
}

// rawHonorTypesDataList 构造荣誉类型响应（带 dataList 字段名区分）。
func rawHonorTypesDataList(types any) string {
	b, _ := json.Marshal(map[string]any{
		"code":     1,
		"msg":      "成功",
		"dataList": types,
	})
	return string(b)
}

// rawHonorTypesReturnData 构造荣誉类型响应（returnData 风格）。
func rawHonorTypesReturnData(types any) string {
	b, _ := json.Marshal(map[string]any{
		"code":       1,
		"msg":        "成功",
		"dataList":   []any{},
		"returnData": types,
	})
	return string(b)
}

// TestGetSubmittedCirclesJSON_MultiPageMerging 验证自动跨页合并。
func TestGetSubmittedCirclesJSON_MultiPageMerging(t *testing.T) {
	var page1Hits, page2Hits int32
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/getStudentCircle" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		pageNo, _ := strconv.Atoi(q.Get("pageNo"))
		pageSize, _ := strconv.Atoi(q.Get("pageSize"))
		w.Header().Set("Content-Type", "application/json")
		switch pageNo {
		case 1:
			atomic.AddInt32(&page1Hits, 1)
			// 第一页：3 条记录，totalNum=4 pageSize=2 触发翻页
			pb := map[string]any{"pageNo": 1, "pageSize": pageSize, "totalNum": 4, "totalPage": 2}
			body := map[string]any{
				"code":     1,
				"dataList": []map[string]any{{"id": 100}, {"id": 101}, {"id": 102}},
				"pageBean": pb,
			}
			_ = json.NewEncoder(w).Encode(body)
		case 2:
			atomic.AddInt32(&page2Hits, 1)
			pb := map[string]any{"pageNo": 2, "pageSize": pageSize, "totalNum": 4, "totalPage": 2}
			body := map[string]any{
				"code":     1,
				"dataList": []map[string]any{{"id": 103}},
				"pageBean": pb,
			}
			_ = json.NewEncoder(w).Encode(body)
		default:
			http.Error(w, "unexpected page", http.StatusBadRequest)
		}
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL),
		client.WithSSOBase(biz.URL),
		client.WithUploadURL(biz.URL),
		client.WithSubmittedPageSize(2),
	)
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	raw, err := c.GetSubmittedCirclesJSON(context.Background(), "test-token", "")
	if err != nil {
		t.Fatalf("GetSubmittedCirclesJSON: %v", err)
	}

	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("合并结果非合法 JSON 数组: %v body=%s", err, raw)
	}
	if len(arr) != 4 {
		t.Fatalf("期望 4 条记录, 得到 %d", len(arr))
	}
	if page1Hits != 1 || page2Hits != 1 {
		t.Errorf("期望各调一次 (page1=%d, page2=%d)", page1Hits, page2Hits)
	}
}

// TestGetSubmittedCirclesJSON_PreservesRawFields 验证 byte-for-byte：保留平台独有字段。
func TestGetSubmittedCirclesJSON_PreservesRawFields(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/getStudentCircle" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		pb := map[string]any{"pageNo": 1, "pageSize": 100, "totalNum": 1, "totalPage": 1}
		// 含 CircleRecord 未建模字段：customExtra + 数字精确度
		body := map[string]any{
			"code":     1,
			"dataList": []map[string]any{{"id": 999, "customExtra": "raw_only", "bigNumber": 9007199254740993}},
			"pageBean": pb,
		}
		_ = json.NewEncoder(w).Encode(body)
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL),
		client.WithSSOBase(biz.URL),
		client.WithUploadURL(biz.URL),
	)
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	raw, err := c.GetSubmittedCirclesJSON(context.Background(), "test-token", "")
	if err != nil {
		t.Fatalf("GetSubmittedCirclesJSON: %v", err)
	}
	// 关键：customExtra 字段必须保留（CircleRecord 类型无此字段）
	if !strings.Contains(string(raw), `"customExtra":"raw_only"`) {
		t.Errorf("原始字节应包含 customExtra 字段, body=%s", raw)
	}
	// 大整数必须保留为 JSON Number（不被 Go 反序列化为 float64 后丢精度）
	if !strings.Contains(string(raw), `9007199254740993`) {
		t.Errorf("大整数应原样保留, body=%s", raw)
	}
}

// TestGetSubmittedCirclesJSON_ContextCancel 返回部分合并字节 + ctx.Err()。
func TestGetSubmittedCirclesJSON_ContextCancel(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/getStudentCircle" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		pageNo, _ := strconv.Atoi(q.Get("pageNo"))
		w.Header().Set("Content-Type", "application/json")
		switch pageNo {
		case 1:
			// 第一页：立即返回 5 条 + totalPage=3，pageSize=5 让 SDK 翻页
			pb := map[string]any{"pageNo": 1, "pageSize": 5, "totalNum": 15, "totalPage": 3}
			body := map[string]any{
				"code":     1,
				"dataList": []map[string]any{{"id": 1}, {"id": 2}, {"id": 3}, {"id": 4}, {"id": 5}},
				"pageBean": pb,
			}
			_ = json.NewEncoder(w).Encode(body)
		default:
			// 后续页：延迟响应，让 ctx 提前取消触发翻页顶部 ctx.Err() 分支
			time.Sleep(500 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL),
		client.WithSSOBase(biz.URL),
		client.WithUploadURL(biz.URL),
		client.WithSubmittedPageSize(5),
	)
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	// 80ms 后取消，足以让 page1 拿到，但 page2 sleep 500ms 会被 ctx 截断
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	raw, err := c.GetSubmittedCirclesJSON(ctx, "test-token", "")
	if err == nil {
		t.Fatalf("ctx 取消应返回 error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("错误应包含 ctx 取消语义, 得到 %v", err)
	}
	if len(raw) == 0 {
		t.Fatalf("ctx 取消时应返回已合并部分字节 (page1 的 5 条)")
	}
	// 验证 bytes 仍是合法 JSON 数组
	var arr []map[string]any
	if jerr := json.Unmarshal(raw, &arr); jerr != nil {
		t.Errorf("部分合并结果必须仍是合法 JSON 数组, body=%s err=%v", raw, jerr)
	}
	if len(arr) != 5 {
		t.Errorf("期望 5 条已合并记录, 得到 %d", len(arr))
	}
}

// TestGetHonorTypesJSON_DataList 验证 dataList 通道优先。
func TestGetHonorTypesJSON_DataList(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentMoralEduNew/getHonorType" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		body, _ := json.Marshal(map[string]any{
			"code":       1,
			"dataList":   []map[string]any{{"id": 1, "name": "data-list", "extraField": "raw_only_dataList"}},
			"returnData": []map[string]any{{"id": 2, "name": "return-data"}},
		})
		_, _ = w.Write(body)
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
	if !strings.Contains(string(raw), `"extraField":"raw_only_dataList"`) {
		t.Errorf("dataList 通道字节应原样保留 extraField, body=%s", raw)
	}
	if strings.Contains(string(raw), `"name":"return-data"`) {
		t.Errorf("dataList 有记录时不应回退到 returnData, body=%s", raw)
	}
}

// TestGetHonorTypesJSON_FallbackReturnData dataList 为空时 fallback 到 returnData。
func TestGetHonorTypesJSON_FallbackReturnData(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentMoralEduNew/getHonorType" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(rawHonorTypesReturnData([]map[string]any{
			{"id": 2, "name": "校优干", "fallbackMarker": "via_returnData"},
		})))
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
	if !strings.Contains(string(raw), `"fallbackMarker":"via_returnData"`) {
		t.Errorf("returnData fallback 必须保留, body=%s", raw)
	}
}

// TestGetHonorTypesJSON_EmptyFallbackReturnsArray 验证空回退最终保持 JSON 数组形状。
func TestGetHonorTypesJSON_EmptyFallbackReturnsArray(t *testing.T) {
	for _, tc := range []struct {
		name       string
		dataList   string
		returnData string
	}{
		{name: "empty dataList and null returnData", dataList: "[]", returnData: "null"},
		{name: "null dataList and empty returnData", dataList: "null", returnData: `""`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/studentMoralEduNew/getHonorType" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"code":1,"dataList":` + tc.dataList + `,"returnData":` + tc.returnData + `}`))
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
				t.Fatalf("空回退应返回 JSON 数组 []，实际 %s", got)
			}
		})
	}
}

// TestQuerySelfEvaluationJSON_ReturnData 验证 returnData 通道。
func TestQuerySelfEvaluationJSON_ReturnData(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentMoralEduNew/querySelfEvaluation" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		body := map[string]any{
			"code": 1,
			"returnData": map[string]any{
				"id":             50001,
				"studentComment": "本学期总结...",
				"teacherComment": "老师评语...",
				"rawExtra":       "platform_extra_field",
			},
		}
		_ = json.NewEncoder(w).Encode(body)
	})))
	defer biz.Close()

	c, err := client.New(client.WithBaseURL(biz.URL), client.WithSSOBase(biz.URL), client.WithUploadURL(biz.URL))
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	raw, err := c.QuerySelfEvaluationJSON(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("QuerySelfEvaluationJSON: %v", err)
	}
	if !strings.Contains(string(raw), `"rawExtra":"platform_extra_field"`) {
		t.Errorf("rawExtra 字段必须保留, body=%s", raw)
	}
}

// TestQuerySelfEvaluationJSON_FallbackDataList returnData 为 token 字符串时 fallback 到 dataList[0]。
func TestQuerySelfEvaluationJSON_FallbackDataList(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentMoralEduNew/querySelfEvaluation" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		body := map[string]any{
			"code": 1,
			// 模拟 v1.0.0 修复过的真实场景：returnData 是字符串 token，dataList 才是对象
			"returnData": "some_token_string",
			"dataList": []map[string]any{
				{"id": 60001, "studentComment": "fallback 路径", "rawExtra2": "raw_from_dataList"},
			},
		}
		_ = json.NewEncoder(w).Encode(body)
	})))
	defer biz.Close()

	c, err := client.New(client.WithBaseURL(biz.URL), client.WithSSOBase(biz.URL), client.WithUploadURL(biz.URL))
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	raw, err := c.QuerySelfEvaluationJSON(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("QuerySelfEvaluationJSON: %v", err)
	}
	if !strings.Contains(string(raw), `"rawExtra2":"raw_from_dataList"`) {
		t.Errorf("dataList fallback 必须保留, body=%s", raw)
	}
}

// TestGetHonorListJSON_ReturnsRawRecordsAndPage 返回含 records 和 page 的拼装 JSON。
func TestGetHonorListJSON_ReturnsRawRecordsAndPage(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentMoralEduNew/getHonorByStudentId" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(rawHonorListBody(1, 20, 1, 1, []map[string]any{
			{"id": 1, "name": "校三好", "rawHonorExtra": "honor_raw"},
		})))
	})))
	defer biz.Close()

	c, err := client.New(client.WithBaseURL(biz.URL), client.WithSSOBase(biz.URL), client.WithUploadURL(biz.URL))
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	raw, err := c.GetHonorListJSON(context.Background(), "test-token", 1, 20, "")
	if err != nil {
		t.Fatalf("GetHonorListJSON: %v", err)
	}
	if !strings.Contains(string(raw), `"rawHonorExtra":"honor_raw"`) {
		t.Errorf("应保留平台字段, body=%s", raw)
	}
	if !strings.Contains(string(raw), `"page":`) {
		t.Errorf("应包含 page 字段, body=%s", raw)
	}
	if !strings.Contains(string(raw), `"records":`) {
		t.Errorf("应包含 records 字段, body=%s", raw)
	}
	// 验证是合法 JSON
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("返回非合法 JSON: %v body=%s", err, raw)
	}
}

// TestFetchTasksJSON_PartialFailureReturnsRawBytes 部分维度失败时返回已有合并字节。
func TestFetchTasksJSON_PartialFailureReturnsRawBytes(t *testing.T) {
	var okHits, failHits int32
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			body := map[string]any{
				"code":     1,
				"dataList": []map[string]any{{"id": 1, "name": "维度A"}, {"id": 2, "name": "维度B"}},
			}
			_ = json.NewEncoder(w).Encode(body)
		case r.URL.Path == "/api/studentCircleNew/getCircleStatistics":
			q := r.URL.Query()
			dimID := q.Get("dimensionId")
			w.Header().Set("Content-Type", "application/json")
			if dimID == "1" {
				atomic.AddInt32(&okHits, 1)
				body := map[string]any{
					"code":     1,
					"dataList": []map[string]any{{"id": 1001, "name": "任务X", "rawTaskExtra": "raw"}},
				}
				_ = json.NewEncoder(w).Encode(body)
			} else {
				atomic.AddInt32(&failHits, 1)
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"code":500,"msg":"simulated failure"}`))
			}
		default:
			http.NotFound(w, r)
		}
	})))
	defer biz.Close()

	c, err := client.New(client.WithBaseURL(biz.URL), client.WithSSOBase(biz.URL), client.WithUploadURL(biz.URL))
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	raw, err := c.FetchTasksJSON(context.Background(), "test-token")
	if err == nil {
		t.Fatalf("部分维度失败应返回非 nil error")
	}
	if !errors.Is(err, client.ErrBusinessRejected) {
		t.Errorf("错误应包装 ErrBusinessRejected, 得到 %v", err)
	}
	if len(raw) == 0 {
		t.Fatalf("失败时应返回已合并字节")
	}
	if !strings.Contains(string(raw), `"rawTaskExtra":"raw"`) {
		t.Errorf("已合并字节应保留 raw 字段, body=%s", raw)
	}
	// 验证仍是合法 JSON
	var probe any
	if jerr := json.Unmarshal(raw, &probe); jerr != nil {
		t.Errorf("部分合并结果必须仍是合法 JSON, body=%s err=%v", raw, jerr)
	}
}

// TestGetCirclesJSON_BufferAssemblyConsistency 验证 getCirclesJSON 成功/错误路径的 buffer 拼接一致性。
//
// 测试逻辑：
//   - 构造一个多页数据的 mock 服务
//   - 第一页立即返回，第二页延迟返回（模拟慢请求）
//   - 用短超时的 ctx，让 g.Wait() 在第二页返回前超时
//   - 验证错误路径返回的部分数据仍是合法 JSON 数组
//   - 对比成功路径和错误路径的 buffer 拼接结果格式一致性
func TestGetCirclesJSON_BufferAssemblyConsistency(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/getStudentCircle" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		pageNo, _ := strconv.Atoi(q.Get("pageNo"))
		pageSize, _ := strconv.Atoi(q.Get("pageSize"))
		w.Header().Set("Content-Type", "application/json")
		switch pageNo {
		case 1:
			// 第一页：立即返回 2 条记录
			pb := map[string]any{"pageNo": 1, "pageSize": pageSize, "totalNum": 4, "totalPage": 2}
			body := map[string]any{
				"code":     1,
				"dataList": []map[string]any{{"id": 100}, {"id": 101}},
				"pageBean": pb,
			}
			_ = json.NewEncoder(w).Encode(body)
		case 2:
			// 第二页：延迟返回，模拟慢请求
			time.Sleep(500 * time.Millisecond)
			pb := map[string]any{"pageNo": 2, "pageSize": pageSize, "totalNum": 4, "totalPage": 2}
			body := map[string]any{
				"code":     1,
				"dataList": []map[string]any{{"id": 102}, {"id": 103}},
				"pageBean": pb,
			}
			_ = json.NewEncoder(w).Encode(body)
		default:
			http.Error(w, "unexpected page", http.StatusBadRequest)
		}
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL),
		client.WithSSOBase(biz.URL),
		client.WithUploadURL(biz.URL),
		client.WithSubmittedPageSize(2),
	)
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	// 用短超时的 ctx，让第二页的延迟触发超时
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	raw, err := c.GetSubmittedCirclesJSON(ctx, "test-token", "")
	if err == nil {
		t.Fatalf("ctx 超时应返回 error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("错误应包含 ctx 取消语义, 得到 %v", err)
	}
	if len(raw) == 0 {
		t.Fatalf("ctx 超时时应返回已合并部分字节 (page1 的 2 条)")
	}

	// 验证 bytes 仍是合法 JSON 数组
	var arr []map[string]any
	if jerr := json.Unmarshal(raw, &arr); jerr != nil {
		t.Errorf("部分合并结果必须仍是合法 JSON 数组, body=%s err=%v", raw, jerr)
	}
	if len(arr) != 2 {
		t.Errorf("期望 2 条已合并记录, 得到 %d", len(arr))
	}
	// 验证 JSON 格式与成功路径一致（首尾都是 []，不是 [ ] 之间有额外逗号）
	if !strings.HasPrefix(string(raw), "[") || !strings.HasSuffix(string(raw), "]") {
		t.Errorf("结果应以 [ 开头和 ] 结尾, body=%s", raw)
	}
}

// TestGetCirclesLimitJSON_ConcurrentPagination 验证 getCirclesLimitJSON 使用并发翻页。
//
// 测试逻辑：
//   - 构造一个多页数据的 mock 服务
//   - 第一页立即返回，第二页延迟返回（模拟慢请求）
//   - 用短超时的 ctx，让并发翻页在第二页返回前超时
//   - 验证错误路径返回的部分数据仍是合法 JSON 数组
func TestGetCirclesLimitJSON_ConcurrentPagination(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/getStudentCircle" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		pageNo, _ := strconv.Atoi(q.Get("pageNo"))
		pageSize, _ := strconv.Atoi(q.Get("pageSize"))
		w.Header().Set("Content-Type", "application/json")
		switch pageNo {
		case 1:
			// 第一页：立即返回 2 条记录
			pb := map[string]any{"pageNo": 1, "pageSize": pageSize, "totalNum": 4, "totalPage": 2}
			body := map[string]any{
				"code":     1,
				"dataList": []map[string]any{{"id": 100}, {"id": 101}},
				"pageBean": pb,
			}
			_ = json.NewEncoder(w).Encode(body)
		case 2:
			// 第二页：延迟返回，模拟慢请求
			time.Sleep(500 * time.Millisecond)
			pb := map[string]any{"pageNo": 2, "pageSize": pageSize, "totalNum": 4, "totalPage": 2}
			body := map[string]any{
				"code":     1,
				"dataList": []map[string]any{{"id": 102}, {"id": 103}},
				"pageBean": pb,
			}
			_ = json.NewEncoder(w).Encode(body)
		default:
			http.Error(w, "unexpected page", http.StatusBadRequest)
		}
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL),
		client.WithSSOBase(biz.URL),
		client.WithUploadURL(biz.URL),
		client.WithSubmittedPageSize(2),
	)
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	// 用短超时的 ctx，让第二页的延迟触发超时
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// offset=0, limit=4 表示全量，应该触发翻页
	raw, _, err := c.GetSubmittedCirclesLimitJSON(ctx, "test-token", 0, 4, "")
	if err == nil {
		t.Fatalf("ctx 超时应返回 error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("错误应包含 ctx 取消语义, 得到 %v", err)
	}
	if len(raw) == 0 {
		t.Fatalf("ctx 超时时应返回已合并部分字节 (page1 的 2 条)")
	}

	// 验证 bytes 仍是合法 JSON 数组
	var arr []map[string]any
	if jerr := json.Unmarshal(raw, &arr); jerr != nil {
		t.Errorf("部分合并结果必须仍是合法 JSON 数组, body=%s err=%v", raw, jerr)
	}
	if len(arr) != 2 {
		t.Errorf("期望 2 条已合并记录, 得到 %d", len(arr))
	}
}

// TestGetSubmittedCirclesJSON_KeyParam 验证 JSON 路径透传 key 到 getStudentCircle。
func TestGetSubmittedCirclesJSON_KeyParam(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/getStudentCircle" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("key") != "劳动实践" {
			t.Errorf("期望 key=劳动实践，实际 key=%q", r.URL.Query().Get("key"))
		}
		if r.URL.Query().Get("type") != "3" {
			t.Errorf("期望 type=3，实际 type=%s", r.URL.Query().Get("type"))
		}
		w.Header().Set("Content-Type", "application/json")
		pb := map[string]any{"pageNo": 1, "pageSize": 100, "totalNum": 0, "totalPage": 0}
		body := map[string]any{"code": 1, "dataList": []map[string]any{}, "pageBean": pb}
		_ = json.NewEncoder(w).Encode(body)
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL),
		client.WithSSOBase(biz.URL),
		client.WithUploadURL(biz.URL),
	)
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	_, err = c.GetSubmittedCirclesJSON(context.Background(), "test-token", "劳动实践")
	if err != nil {
		t.Fatalf("GetSubmittedCirclesJSON key 透传失败: %v", err)
	}
}

// TestGetHonorListJSON_KeyParam 验证 GetHonorListJSON 透传 key。
func TestGetHonorListJSON_KeyParam(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentMoralEduNew/getHonorByStudentId" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("key") != "校级" {
			t.Errorf("期望 key=校级，实际 key=%q", r.URL.Query().Get("key"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(rawHonorListBody(1, 20, 0, 0, []map[string]any{})))
	})))
	defer biz.Close()

	c, err := client.New(client.WithBaseURL(biz.URL), client.WithSSOBase(biz.URL), client.WithUploadURL(biz.URL))
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	_, err = c.GetHonorListJSON(context.Background(), "test-token", 1, 20, "校级")
	if err != nil {
		t.Fatalf("GetHonorListJSON key 透传失败: %v", err)
	}
}
