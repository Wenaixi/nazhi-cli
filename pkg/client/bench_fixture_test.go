package client

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ─── benchmark / perf-budget 共用 fixture ───
//
// 全部为 package client 白盒（需直接调用 httpDo / assembleCirclesJSON /
// appendPageRange / fetchCirclePageJSON 等未导出函数）。
//
// 构造 Client 用结构体字面量而非 New()：
//   - New() 会走 Option 处理并对明文 HTTP 打 Warn，污染测量
//   - 需要精确控制 logger 级别（P1-1 的触发条件就是「日志未启用」）

// benchClient 构造最小可用 Client，指向 mock 服务。
//
// sm 必须非 nil：fetchCirclePageJSON 等路径经 doBizAndDecode → ActivateSession
// → sm.Activate，sm 为 nil 会 panic。
//
// logger 用 DiscardHandler（Enabled 恒返回 false），精确模拟默认 LevelWarn 下
// Info 日志被过滤的真实场景——P1-1 的分配浪费正是在这个条件下发生。
func benchClient(tb testing.TB, bizURL string) *Client {
	tb.Helper()
	c := &Client{
		ssoBaseURL:        bizURL,
		baseURL:           bizURL,
		uploadURL:         bizURL,
		http:              newHTTPClient(),
		logger:            slog.New(slog.DiscardHandler),
		sm:                &sessionManager{},
		submittedPageSize: defaultSubmittedPageSize,
	}
	if parsed, err := url.Parse(bizURL); err == nil {
		c.baseURLParsed.Store(parsed)
	}
	tb.Cleanup(func() { _ = c.Close() })
	return c
}

// benchBizServer 启动只回固定响应体的 mock 服务。
// body 在服务启动前构造一次，b.N 次请求复用同一份字节，服务端零分配干扰。
func benchBizServer(tb testing.TB, body []byte) *httptest.Server {
	tb.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	tb.Cleanup(srv.Close)
	return srv
}

// benchWarmupBizServer 响应 ActivateSession 的 4 步预热，其余路径回 circleBody。
//
// 白盒版 warmupBizHandler——external 包的同名 helper 在 client_test.go，
// 白盒测试无法引用。预热响应与业务响应分开，保证 4 步能正常完成。
//
// getMyInfo 返回的 UserInfo 必须带 studentNumber 且 SchoolID/SchoolName 齐全：
// 否则 ActivateSession 出口会走 postProcessSchoolFallback（user.go:115），
// 对 mock 服务多发一次 GetSchoolID 请求，污染「缓存命中」路径的测量。
func benchWarmupBizServer(tb testing.TB, circleBody []byte) *httptest.Server {
	tb.Helper()
	menuBody := []byte(`{"code":1,"msg":"成功"}`)
	myInfoBody := []byte(`{"code":1,"msg":"成功","returnData":{"name":"bench","studentNumber":"BENCH001","className":"八班","seat":1,"schoolId":173,"schoolName":"bench学校"}}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			_, _ = w.Write(menuBody)
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write(myInfoBody)
		default:
			_, _ = w.Write(circleBody)
		}
	}))
	tb.Cleanup(srv.Close)
	return srv
}

// benchContentText 是记录正文模板，长度贴近前端 200 字上限的常见填写量。
const benchContentText = "参加了本次benchmark活动，在准备与实施过程中完成了资料查阅、方案设计与现场执行，最终取得预期成果，特此记录以便归档。"

// benchCircleRecord 生成单条 CircleRecord 形状的 JSON 对象。
//
// 字段名与平台真实 JSON 对齐（camelCase 与 snake_case 混用，见 CLAUDE.md
// 「响应信封与 JSON 类型」节），保证解码路径测到的是真实形状而非简化形状。
func benchCircleRecord(i int) string {
	var sb strings.Builder
	sb.Grow(640)
	sb.WriteString(`{"id":`)
	sb.WriteString(strconv.Itoa(1000000 + i))
	sb.WriteString(`,"name":"bench活动","content":"`)
	sb.WriteString(benchContentText)
	sb.WriteString(`","type_name":"活动项目","status":0,"type":1`)
	sb.WriteString(`,"host_name":"bench主办单位","rank":"一等奖","level":3`)
	sb.WriteString(`,"hours":2.5,"check_result":1,"play_role":1,"approved":1`)
	sb.WriteString(`,"operator_name":"张三","creationTimeStr":"2026-09-01 10:00:00"`)
	sb.WriteString(`,"circle_task_name":"bench任务名称","showName":"活动项目 bench活动"`)
	sb.WriteString(`,"creator_name":"张三","class_name":"八年级(2)班","grade_name":"八年级"`)
	sb.WriteString(`,"imgList":[`)
	for j := 0; j < 4; j++ {
		if j > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(`{"id":`)
		sb.WriteString(strconv.Itoa(j))
		sb.WriteString(`,"url":"https://doc.nazhisoft.com/common/attachment/getImg?id=`)
		sb.WriteString(strconv.Itoa(i*10 + j))
		sb.WriteString(`&type=1&groupName=other"}`)
	}
	sb.WriteString(`],"imgPreViewList":[],"commentList":[]}`)
	return sb.String()
}

// benchDataListJSON 生成含 n 条记录的 dataList JSON 数组。
// n=500 时约 300KB，n=2000 时约 1.2MB——后者对齐真实公示页
// （500 条 + 图片 URL 实测超 1MB，见 request.go maxResponseBodySize 注释）。
func benchDataListJSON(n int) []byte {
	var sb strings.Builder
	sb.Grow(n * 640)
	sb.WriteByte('[')
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(benchCircleRecord(i))
	}
	sb.WriteByte(']')
	return []byte(sb.String())
}

// benchUnifiedBody 把 dataList 包进平台统一响应信封（含 pageBean）。
// 平台业务信封键为 code / msg / dataList / pageBean（非 CLI envelope 的
// status/code/message/data，见 CLAUDE.md 契约 A 节）。
func benchUnifiedBody(dataList []byte, totalNum, totalPage int) []byte {
	var sb strings.Builder
	sb.Grow(len(dataList) + 160)
	sb.WriteString(`{"code":1,"msg":"成功","dataList":`)
	sb.Write(dataList)
	sb.WriteString(`,"pageBean":{"pageNo":1,"pageSize":500,"totalNum":`)
	sb.WriteString(strconv.Itoa(totalNum))
	sb.WriteString(`,"totalPage":`)
	sb.WriteString(strconv.Itoa(totalPage))
	sb.WriteString(`}}`)
	return []byte(sb.String())
}

// benchPageBean 返回分页元数据，供纯内存路径（assembleCirclesJSON）直接调用。
func benchPageBean() *types.PageBean {
	return &types.PageBean{PageNo: 1, PageSize: 500, TotalNum: 2000, TotalPage: 4}
}

// TestBenchFixture_GeneratesValidJSON 是 fixture 自身的自检。
// fixture 若产出非法 JSON，所有 benchmark 都在测错误路径，
// 数字再好看也无意义——这个测试是测量有效性的前提。
func TestBenchFixture_GeneratesValidJSON(t *testing.T) {
	list := benchDataListJSON(3)
	if !json.Valid(list) {
		t.Fatalf("benchDataListJSON 产出非法 JSON: %s", string(list[:min(len(list), 200)]))
	}

	var recs []types.CircleRecord
	if err := json.Unmarshal(list, &recs); err != nil {
		t.Fatalf("benchDataListJSON 无法解码为 []CircleRecord: %v", err)
	}
	if len(recs) != 3 {
		t.Fatalf("期望 3 条记录，实际 %d", len(recs))
	}
	// 抽查字段真的解出来了，防止 JSON 键名拼错导致全零结构体
	if recs[0].Name != "bench活动" {
		t.Errorf("recs[0].Name = %q，期望 %q", recs[0].Name, "bench活动")
	}
	if recs[0].Hours != 2.5 {
		t.Errorf("recs[0].Hours = %v，期望 2.5", recs[0].Hours)
	}
	if len(recs[0].ImgList) != 4 {
		t.Errorf("recs[0].ImgList 长度 = %d，期望 4", len(recs[0].ImgList))
	}

	body := benchUnifiedBody(list, 2000, 4)
	if !json.Valid(body) {
		t.Fatalf("benchUnifiedBody 产出非法 JSON")
	}
	var resp types.UnifiedResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("benchUnifiedBody 无法解码为 UnifiedResponse: %v", err)
	}
	if resp.Code != 1 {
		t.Errorf("resp.Code = %d，期望 1", resp.Code)
	}
	if resp.DataList == nil {
		t.Fatal("resp.DataList 为 nil，信封键名可能拼错")
	}
	if resp.PageBean == nil {
		t.Fatal("resp.PageBean 为 nil，分页键名可能拼错")
	}

	// PageBean 字段必须能解出来（分页逻辑依赖它）
	pb, err := types.DecodePageBean(resp)
	if err != nil {
		t.Fatalf("DecodePageBean 失败: %v", err)
	}
	if pb == nil || pb.TotalNum != 2000 || pb.TotalPage != 4 {
		t.Fatalf("PageBean 解码异常: %+v", pb)
	}
}
