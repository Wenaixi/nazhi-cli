// submitted_test.go 聚合 GetSubmittedCircles 测试（内部白盒 + 外部黑盒）。
package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ─── pageSize 常量 ───

// submittedPageSize 是本文件多页/取消场景使用的测试 pageSize。
// 必须通过 WithSubmittedPageSize 注入到 Client，与生产默认
// defaultSubmittedPageSize=500 解耦，才能稳定触发翻页路径。
const submittedPageSize = 100

// ─── 辅助 ───

// submittedPageBean 生成分页信息 JSON。
func submittedPageBean(pageNo, pageSize, totalNum, totalPage int) string {
	b, _ := json.Marshal(map[string]any{
		"pageNo":    pageNo,
		"pageSize":  pageSize,
		"totalNum":  totalNum,
		"totalPage": totalPage,
	})
	return string(b)
}

// submittedRecord 生成一条写实记录（字段名对齐前端/平台真实 JSON）。
func submittedRecord(id int64, name string, status int) map[string]any {
	return map[string]any{
		"id":             id,
		"name":           name,
		"content":        "写实内容",
		"circle_task_id": id + 10000,
		"circle_type_id": 9255,
		"dimension_id":   9,
		"type_name":      "爱党爱国教育",
		"status":         status,
		"circle_date":    "2026-02-06",
		"hours":          0.5,
		"studentId":      387020,
		"class_name":     "八班",
		"ifMySelf":       1,
		"imgList":        []map[string]any{},
		"remark":         name,
	}
}

// ─── 外部黑盒测试 ───

// TestGetSubmittedCircles_SinglePage 验证单页全部返回。
func TestGetSubmittedCircles_SinglePage(t *testing.T) {
	records := []map[string]any{
		submittedRecord(1, "国旗下讲话", 0),
		submittedRecord(2, "班会", 1),
		submittedRecord(3, "劳动实践", 0),
	}

	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getStudentCircle" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": json.RawMessage(submittedPageBean(1, submittedPageSize, 3, 1)),
				"dataList": records,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	circles, err := c.GetSubmittedCircles(context.Background(), "test-token", "")
	if err != nil {
		t.Fatalf("GetSubmittedCircles 失败: %v", err)
	}
	if len(circles) != 3 {
		t.Fatalf("期望 3 条记录，实际 %d", len(circles))
	}
	if circles[0].ID != 1 {
		t.Errorf("期望 ID=1，实际 %d", circles[0].ID)
	}
}

// TestGetSubmittedCircles_MultiPage 验证多页自动分页。
//
// 客户端 pageSize 必须 < TotalNum，才会走 fetchAllCirclePages 并发翻页；
// 测试用 WithSubmittedPageSize(100)，与生产默认 500 解耦。
func TestGetSubmittedCircles_MultiPage(t *testing.T) {
	const totalRecords = 250
	const totalPages = 3 // ceil(250/100)

	var callCount int
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getStudentCircle" {
			// 按请求的 pageNo 生成页，避免并发翻页时用 callCount 当页号错位
			pageNo, _ := strconv.Atoi(r.URL.Query().Get("pageNo"))
			if pageNo < 1 {
				pageNo = 1
			}
			callCount++
			totalPage := totalPages
			totalNum := totalRecords

			start := (pageNo-1)*submittedPageSize + 1
			end := start + submittedPageSize - 1
			if end > totalNum {
				end = totalNum
			}
			count := end - start + 1
			if count < 0 {
				count = 0
			}

			records := make([]map[string]any, 0, count)
			for i := start; i <= end; i++ {
				records = append(records, submittedRecord(int64(i), "任务", 0))
			}
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": json.RawMessage(submittedPageBean(pageNo, submittedPageSize, totalNum, totalPage)),
				"dataList": records,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL),
		client.WithTimeout(5*time.Second),
		client.WithSubmittedPageSize(submittedPageSize),
	)
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	circles, err := c.GetSubmittedCircles(context.Background(), "test-token", "")
	if err != nil {
		t.Fatalf("GetSubmittedCircles 失败: %v", err)
	}
	if len(circles) != totalRecords {
		t.Fatalf("期望 %d 条记录（分页合并），实际 %d", totalRecords, len(circles))
	}
	if callCount != totalPages {
		t.Errorf("期望 %d 次 API 调用，实际 %d", totalPages, callCount)
	}
}

// TestGetSubmittedCircles_Empty 验证没有记录时返回空切片。
func TestGetSubmittedCircles_Empty(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getStudentCircle" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": json.RawMessage(submittedPageBean(1, submittedPageSize, 0, 0)),
				"dataList": []map[string]any{},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	circles, err := c.GetSubmittedCircles(context.Background(), "test-token", "")
	if err != nil {
		t.Fatalf("GetSubmittedCircles 失败: %v", err)
	}
	if circles == nil {
		t.Fatal("期望空切片，实际 nil")
	}
	if len(circles) != 0 {
		t.Fatalf("期望 0 条记录，实际 %d", len(circles))
	}
}

// TestGetSubmittedCircles_BizError 验证业务错误被正确包装。
func TestGetSubmittedCircles_BizError(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getStudentCircle" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":500,"msg":"服务器错误"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.GetSubmittedCircles(context.Background(), "test-token", "")
	if err == nil {
		t.Fatal("期望业务错误，实际 nil")
	}
	if !strings.Contains(err.Error(), "服务器错误") {
		t.Errorf("错误消息应含业务错误描述: %v", err)
	}
}

// TestGetSubmittedCircles_TotalPageGTR1ButNoData 验证总页数多但第二页没数据（容错场景）。
func TestGetSubmittedCircles_TotalPageGTR1ButNoData(t *testing.T) {
	var callCount int
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getStudentCircle" {
			callCount++
			w.Header().Set("Content-Type", "application/json")
			if callCount == 1 {
				records := []map[string]any{
					submittedRecord(1, "任务1", 0),
				}
				resp := map[string]any{
					"code":     1,
					"pageBean": json.RawMessage(submittedPageBean(1, submittedPageSize, 1, 2)),
					"dataList": records,
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			// 第二页应该无数据但返回空
			resp := map[string]any{
				"code":     1,
				"pageBean": json.RawMessage(submittedPageBean(2, submittedPageSize, 1, 2)),
				"dataList": []map[string]any{},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	circles, err := c.GetSubmittedCircles(context.Background(), "test-token", "")
	if err != nil {
		t.Fatalf("GetSubmittedCircles 失败: %v", err)
	}
	if len(circles) != 1 {
		t.Fatalf("期望 1 条记录（仅第一页），实际 %d", len(circles))
	}
}

// TestGetSubmittedCircles_TotalPage0 验证 totalPage=0（无记录）时不发起第二页请求。
func TestGetSubmittedCircles_TotalPage0(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getStudentCircle" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": json.RawMessage(submittedPageBean(1, submittedPageSize, 0, 0)),
				"dataList": []map[string]any{},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	circles, err := c.GetSubmittedCircles(context.Background(), "test-token", "")
	if err != nil {
		t.Fatalf("GetSubmittedCircles 失败: %v", err)
	}
	if len(circles) != 0 {
		t.Fatalf("期望 0 条记录，实际 %d", len(circles))
	}
}

// TestGetSubmittedCircles_CustomPageSize 验证 WithSubmittedPageSize 自配置生效。
func TestGetSubmittedCircles_CustomPageSize(t *testing.T) {
	const customSize = 50

	var gotPageSize int
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getStudentCircle" {
			gotPageSize, _ = strconv.Atoi(r.URL.Query().Get("pageSize"))
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": json.RawMessage(submittedPageBean(1, customSize, 0, 0)),
				"dataList": []map[string]any{},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL),
		client.WithTimeout(time.Second),
		client.WithSubmittedPageSize(customSize),
	)
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}

	_, err = c.GetSubmittedCircles(context.Background(), "test-token", "")
	if err != nil {
		t.Fatalf("GetSubmittedCircles 失败: %v", err)
	}
	if gotPageSize != customSize {
		t.Errorf("期望 pageSize=%d，实际 %d", customSize, gotPageSize)
	}
}

// ─── 内部白盒测试（package client） ───

// TestSubmittedDecodePageBean 验证 DecodePageBean 正常解析分页信息。
func TestSubmittedDecodePageBean(t *testing.T) {
	jsonData := []byte(`{"code":1,"pageBean":{"pageNo":1,"pageSize":20,"totalNum":23,"totalPage":2}}`)
	resp, err := types.DecodeResponse(jsonData)
	if err != nil {
		t.Fatalf("DecodeResponse 失败: %v", err)
	}

	pb, err := types.DecodePageBean(resp)
	if err != nil {
		t.Fatalf("DecodePageBean 失败: %v", err)
	}
	if pb.PageNo != 1 {
		t.Errorf("期望 pageNo=1，实际 %d", pb.PageNo)
	}
	if pb.TotalNum != 23 {
		t.Errorf("期望 totalNum=23，实际 %d", pb.TotalNum)
	}
	if pb.TotalPage != 2 {
		t.Errorf("期望 totalPage=2，实际 %d", pb.TotalPage)
	}
}

// TestSubmittedDecodePageBean_Nil 验证 pageBean 为 nil 时安全返回。
func TestSubmittedDecodePageBean_Nil(t *testing.T) {
	jsonData := []byte(`{"code":1}`)
	resp, _ := types.DecodeResponse(jsonData)
	pb, err := types.DecodePageBean(resp)
	if err != nil {
		t.Fatalf("DecodePageBean nil 时不应报错: %v", err)
	}
	if pb != nil {
		t.Fatal("期望 nil")
	}
}

// TestSubmittedDecodeCircleRecord 验证 CircleRecord 解码。
func TestSubmittedDecodeCircleRecord(t *testing.T) {
	// 使用平台真实 camelCase 混用命名（imgList/ifMySelf/creationTimeStr）
	jsonData := `{"id":1,"name":"国旗下讲话","content":"写实内容","type_name":"爱党爱国教育","approved":false,"circle_date":"2026-02-06T00:00:00Z","hours":0.5,"imgList":[{"id":1,"attachment_id":9,"imgPath":".jpg"}],"ifMySelf":1,"creationTimeStr":"2026-02-06 10:00","remark":"国旗下讲话"}`
	var rec types.CircleRecord
	if err := json.Unmarshal([]byte(jsonData), &rec); err != nil {
		t.Fatalf("Unmarshal CircleRecord 失败: %v", err)
	}
	if rec.ID != 1 || rec.Name != "国旗下讲话" || rec.Approved.Bool() {
		t.Errorf("字段匹配失败: %+v", rec)
	}
	if rec.Hours != 0.5 {
		t.Errorf("期望 hours=0.5，实际 %f", rec.Hours)
	}
	if rec.TypeName != "爱党爱国教育" {
		t.Errorf("期望 typeName=爱党爱国教育，实际 %s", rec.TypeName)
	}
	if len(rec.ImgList) != 1 || rec.ImgList[0].ImgPath != ".jpg" {
		t.Errorf("期望 imgList 解码成功，实际 %+v", rec.ImgList)
	}
	if rec.IfMySelf != 1 {
		t.Errorf("期望 ifMySelf=1，实际 %d", rec.IfMySelf)
	}
	if rec.CreationTimeStr != "2026-02-06 10:00" {
		t.Errorf("期望 creationTimeStr，实际 %q", rec.CreationTimeStr)
	}
}

// TestGetSubmittedCircles_CancelDuringPaging 验证翻页过程中 context 取消时返回已有数据 + error。
//
// 设计（对齐并发翻页 errgroup）：
//  1. pageSize=100，TotalNum=300 → 强制拉 page 2..3
//  2. page1 立即返回 2 条；后续页阻塞到 ctx 取消，再写空页
//  3. 主 goroutine 在 page2 到达后 cancel，期望 g.Wait 失败并带回 partial
//  4. 全程带超时，避免再出现「不翻页 → 永久阻塞 page2Started」
func TestGetSubmittedCircles_CancelDuringPaging(t *testing.T) {
	const (
		page1Records = 2
		totalPages   = 3
		totalNum     = totalPages * submittedPageSize // 300 > pageSize，触发翻页
	)

	var (
		callCount atomic.Int32
		once      sync.Once
	)
	page2Started := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/getStudentCircle" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		pageNo, _ := strconv.Atoi(r.URL.Query().Get("pageNo"))
		if pageNo < 1 {
			pageNo = 1
		}
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")

		if pageNo == 1 {
			records := make([]map[string]any, page1Records)
			for i := range records {
				records[i] = submittedRecord(int64(i+1), "任务", 0)
			}
			resp := map[string]any{
				"code":     1,
				"pageBean": json.RawMessage(submittedPageBean(1, submittedPageSize, totalNum, totalPages)),
				"dataList": records,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// 后续页：通知主测试后阻塞到 cancel；并发多页用 Once 避免 double-close
		once.Do(func() { close(page2Started) })
		select {
		case <-ctx.Done():
		case <-time.After(10 * time.Second):
			// 测试异常时仍返回，避免 mock 永久占连接
		}
		resp := map[string]any{
			"code":     1,
			"pageBean": json.RawMessage(submittedPageBean(pageNo, submittedPageSize, totalNum, totalPages)),
			"dataList": []map[string]any{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL),
		client.WithTimeout(5*time.Second),
		client.WithSubmittedPageSize(submittedPageSize),
	)
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	type getResult struct {
		circles []types.CircleRecord
		err     error
	}
	resultCh := make(chan getResult, 1)
	go func() {
		circles, err := c.GetSubmittedCircles(ctx, "test-token", "")
		resultCh <- getResult{circles, err}
	}()

	select {
	case <-page2Started:
		cancel()
	case <-time.After(5 * time.Second):
		t.Fatal("超时：未等到 page2 请求（检查 pageSize/TotalNum 是否触发翻页）")
	}

	var r getResult
	select {
	case r = <-resultCh:
	case <-time.After(5 * time.Second):
		t.Fatal("超时：GetSubmittedCircles 未在 cancel 后返回")
	}

	if r.err == nil {
		t.Fatal("context 取消时应返回 error（调用方需感知截断），实际 nil")
	}
	if !strings.Contains(r.err.Error(), "context canceled") && !errors.Is(r.err, context.Canceled) {
		t.Errorf("error 应包含 context canceled 描述: %v", r.err)
	}
	// 至少应带回第一页已成功解析的数据
	if len(r.circles) < page1Records {
		t.Fatalf("期望至少 %d 条记录（第一页数据），实际 %d", page1Records, len(r.circles))
	}
}

// TestSubmittedDecodeCircleImg 验证 CircleImage 解码（imgPath 为 camelCase）。
func TestSubmittedDecodeCircleImg(t *testing.T) {
	jsonData := `{"id":4987641,"circle_id":5389265,"class_id":162647,"task_id":18296,"attachment_id":5005765,"imgPath":".jpg"}`
	var img types.CircleImage
	if err := json.Unmarshal([]byte(jsonData), &img); err != nil {
		t.Fatalf("Unmarshal CircleImage 失败: %v", err)
	}
	if img.ID != 4987641 || img.CircleID != 5389265 || img.ClassID != 162647 || img.TaskID != 18296 || img.AttachmentID != 5005765 || img.ImgPath != ".jpg" {
		t.Errorf("字段匹配失败: %+v", img)
	}
}

// ─── PeekSubmittedTotal 测试 ───

// TestPeekSubmittedTotal_Normal 验证正常返回总记录数。
func TestPeekSubmittedTotal_Normal(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getStudentCircle" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": json.RawMessage(submittedPageBean(1, 1, 23, 23)),
				"dataList": []map[string]any{submittedRecord(1, "国旗下讲话", 0)},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	total, err := c.PeekSubmittedTotal(context.Background(), "test-token", "")
	if err != nil {
		t.Fatalf("PeekSubmittedTotal 失败: %v", err)
	}
	if total != 23 {
		t.Errorf("期望 total=23，实际 %d", total)
	}
}

// TestPeekSubmittedTotal_Zero 验证无记录时返回 0 不报错。
func TestPeekSubmittedTotal_Zero(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getStudentCircle" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": json.RawMessage(submittedPageBean(1, 1, 0, 0)),
				"dataList": []map[string]any{},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	total, err := c.PeekSubmittedTotal(context.Background(), "test-token", "")
	if err != nil {
		t.Fatalf("PeekSubmittedTotal 不应报错: %v", err)
	}
	if total != 0 {
		t.Errorf("期望 total=0，实际 %d", total)
	}
}

// TestPeekSubmittedTotal_BizError 验证业务错误被正确包装。
func TestPeekSubmittedTotal_BizError(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getStudentCircle" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":500,"msg":"服务器错误"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.PeekSubmittedTotal(context.Background(), "test-token", "")
	if err == nil {
		t.Fatal("期望业务错误，实际 nil")
	}
	if !strings.Contains(err.Error(), "服务器错误") {
		t.Errorf("错误消息应含业务错误描述: %v", err)
	}
}

// TestPeekSubmittedTotal_VerifyQuery 验证请求参数 pageNo=1&pageSize=1。
func TestPeekSubmittedTotal_VerifyQuery(t *testing.T) {
	var gotQuery string
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getStudentCircle" {
			gotQuery = r.URL.RawQuery
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": json.RawMessage(submittedPageBean(1, 1, 5, 5)),
				"dataList": []map[string]any{submittedRecord(1, "国旗下讲话", 0)},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, _ = c.PeekSubmittedTotal(context.Background(), "test-token", "")
	wantQuery := "type=3&pageNo=1&pageSize=1&key="
	if gotQuery != wantQuery {
		t.Errorf("期望 query=%q，实际 %q", wantQuery, gotQuery)
	}
}
