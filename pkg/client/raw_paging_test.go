package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// pagingTestServer 启动一个能通过 session 预热、且按 pageNo 返回不同
// dataList 的 mock 服务。
//
// 预热响应契约沿用 benchWarmupBizServer 已验证的形状：getMyInfo 必须带
// studentNumber 与完整学校字段，否则 ActivateSession 出口会走学校信息降级
// 路径多发一次请求。pageSize=500、totalNum=1500 使服务端声明 3 页。
func pagingTestServer(tb testing.TB, failPages map[string]bool) *httptest.Server {
	tb.Helper()
	menuBody := []byte(`{"code":1,"msg":"成功"}`)
	myInfoBody := []byte(`{"code":1,"msg":"成功","returnData":{"name":"tester","studentNumber":"T001","className":"八班","seat":1,"schoolId":173,"schoolName":"测试学校"}}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			_, _ = w.Write(menuBody)
			return
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write(myInfoBody)
			return
		}

		pn := r.URL.Query().Get("pageNo")
		if failPages[pn] {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功","dataList":[{"id":` + pn +
			`,"content":"第` + pn + `页内容"}],"pageBean":{"pageNo":` + pn +
			`,"pageSize":500,"totalNum":1500,"totalPage":3}}`))
	}))
	tb.Cleanup(srv.Close)
	return srv
}
// 每页字节必须落在自己页号对应的槽位上——这是分页保序的基础，
// 槽位错位会让拼接结果页序混乱。
func TestFetchRawCirclePages_LandsByPageNumber(t *testing.T) {
	srv := pagingTestServer(t, nil)
	c := benchClient(t, srv.URL)

	results := make([]rawResult, 4)
	results[1] = rawResult{raw: []byte(`[{"id":1}]`)}

	if err := c.fetchRawCirclePages(context.Background(), "tok", 3, 500, 1, "", results); err != nil {
		t.Fatalf("并发抓取不应失败: %v", err)
	}

	for pn := 2; pn <= 3; pn++ {
		if len(results[pn].raw) == 0 {
			t.Errorf("第 %d 页槽位应已落位，实际为空", pn)
			continue
		}
		want := `"id":` + string(rune('0'+pn))
		if !strings.Contains(string(results[pn].raw), want) {
			t.Errorf("第 %d 页槽位内容与页号不符，期望含 %s，实际 %s", pn, want, results[pn].raw)
		}
	}
	if results[0].raw != nil {
		t.Errorf("槽位 [0] 恒为零值（首页固定在 [1]），实际 %q", results[0].raw)
	}
}

// TestFetchRawCirclePages_LeavesFailedPageZero 锁定失败页的槽位不被写入：
// 调用方据此判断哪些页真实到手，拼 partial 结果时不会混入空槽。
func TestFetchRawCirclePages_LeavesFailedPageZero(t *testing.T) {
	srv := pagingTestServer(t, map[string]bool{"2": true})
	c := benchClient(t, srv.URL)

	results := make([]rawResult, 3)
	results[1] = rawResult{raw: []byte(`[]`)}

	err := c.fetchRawCirclePages(context.Background(), "tok", 2, 500, 1, "", results)
	if err == nil {
		t.Fatal("第 2 页失败时应返回错误")
	}
	if !strings.Contains(err.Error(), "第 2 页失败") {
		t.Errorf("错误应带「第 2 页失败」页号上下文，实际 %q", err.Error())
	}
	if results[2].raw != nil {
		t.Errorf("失败页槽位应保持零值，实际 %q", results[2].raw)
	}
}

// TestFetchRawCirclePages_SinglePageNoop 锁定边界：只需首页时不发任何请求，
// 直接返回 nil。避免为一个必然为空的循环建 errgroup。
func TestFetchRawCirclePages_SinglePageNoop(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := benchClient(t, srv.URL)

	results := make([]rawResult, 2)
	results[1] = rawResult{raw: []byte(`[]`)}
	if err := c.fetchRawCirclePages(context.Background(), "tok", 1, 500, 1, "", results); err != nil {
		t.Fatalf("单页情形不应返回错误: %v", err)
	}
	if hits != 0 {
		t.Errorf("单页情形不应发出请求，实际发出 %d 次", hits)
	}
}
