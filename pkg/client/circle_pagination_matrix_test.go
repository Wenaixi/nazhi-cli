package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// matrixPageSize 是分页矩阵测试使用的 pageSize，与生产默认 500 解耦，
// 让少量记录即可跨多页，稳定触发翻页路径。
const matrixPageSize = 2

// matrixProbe 记录每个 type 实际收到的请求页序列。
type matrixProbe struct {
	mu          sync.Mutex
	pagesByType map[string][]int
}

func (p *matrixProbe) pagesFor(circleType string) []int {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]int, len(p.pagesByType[circleType]))
	copy(out, p.pagesByType[circleType])
	return out
}

// newMatrixServer 构造按 pageNo 分页、按 type 区分记录前缀的 mock。
func newMatrixServer(t *testing.T, totalRecords int) (*httptest.Server, *matrixProbe) {
	t.Helper()
	probe := &matrixProbe{pagesByType: make(map[string][]int)}

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/getStudentCircle" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		q := r.URL.Query()
		circleType := q.Get("type")
		pageNo, _ := strconv.Atoi(q.Get("pageNo"))
		if pageNo < 1 {
			pageNo = 1
		}

		probe.mu.Lock()
		probe.pagesByType[circleType] = append(probe.pagesByType[circleType], pageNo)
		probe.mu.Unlock()

		totalPages := (totalRecords + matrixPageSize - 1) / matrixPageSize
		start := (pageNo-1)*matrixPageSize + 1
		end := start + matrixPageSize - 1
		if end > totalRecords {
			end = totalRecords
		}

		records := make([]map[string]any, 0)
		for i := start; i <= end; i++ {
			// content 带上 type 与页内序号，使「拿到别的 tab 或错序的数据」可被断言发现。
			// id 必须是数字：CircleRecord.ID 为 int64，结构化路径按平台真实形态
			// 用数字解码，字符串 id 会让整页 DecodeDataList 失败。
			records = append(records, map[string]any{
				"id":      i,
				"content": "type=" + circleType + " seq=" + strconv.Itoa(i),
				"status":  0,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 1, "msg": "成功",
			"dataList": records,
			"pageBean": map[string]any{
				"pageNo": pageNo, "pageSize": matrixPageSize,
				"totalPage": totalPages, "totalNum": totalRecords,
			},
		})
	}

	srv := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, handler)))
	t.Cleanup(srv.Close)
	return srv, probe
}

// circleTypeCase 描述一种 circleType 在两条路径上的取数方式。
type circleTypeCase struct {
	name       string
	wantType   string
	structured func(c *client.Client) ([]json.RawMessage, error)
	raw        func(c *client.Client) (json.RawMessage, error)
}

func matrixCases() []circleTypeCase {
	mk := func(fetch func() ([]json.RawMessage, error)) func(c *client.Client) ([]json.RawMessage, error) {
		return func(c *client.Client) ([]json.RawMessage, error) {
			return fetch()
		}
	}
	_ = mk
	return []circleTypeCase{
		{
			name:     "public",
			wantType: "1",
			structured: func(c *client.Client) ([]json.RawMessage, error) {
				recs, err := c.GetPublicCircles(context.Background(), "test-token", "")
				if err != nil {
					return nil, err
				}
				out := make([]json.RawMessage, 0, len(recs))
				for _, r := range recs {
					b, mErr := json.Marshal(r)
					if mErr != nil {
						return nil, mErr
					}
					out = append(out, b)
				}
				return out, nil
			},
			raw: func(c *client.Client) (json.RawMessage, error) {
				return c.GetPublicCirclesJSON(context.Background(), "test-token", "")
			},
		},
		{
			name:     "teacher",
			wantType: "2",
			structured: func(c *client.Client) ([]json.RawMessage, error) {
				recs, err := c.GetTeacherCircles(context.Background(), "test-token", "")
				if err != nil {
					return nil, err
				}
				out := make([]json.RawMessage, 0, len(recs))
				for _, r := range recs {
					b, mErr := json.Marshal(r)
					if mErr != nil {
						return nil, mErr
					}
					out = append(out, b)
				}
				return out, nil
			},
			raw: func(c *client.Client) (json.RawMessage, error) {
				return c.GetTeacherCirclesJSON(context.Background(), "test-token", "")
			},
		},
		{
			name:     "submitted",
			wantType: "3",
			structured: func(c *client.Client) ([]json.RawMessage, error) {
				recs, err := c.GetSubmittedCircles(context.Background(), "test-token", "")
				if err != nil {
					return nil, err
				}
				out := make([]json.RawMessage, 0, len(recs))
				for _, r := range recs {
					b, mErr := json.Marshal(r)
					if mErr != nil {
						return nil, mErr
					}
					out = append(out, b)
				}
				return out, nil
			},
			raw: func(c *client.Client) (json.RawMessage, error) {
				return c.GetSubmittedCirclesJSON(context.Background(), "test-token", "")
			},
		},
		{
			name:     "withdrawn",
			wantType: "4",
			structured: func(c *client.Client) ([]json.RawMessage, error) {
				recs, err := c.GetWithdrawnCircles(context.Background(), "test-token", "")
				if err != nil {
					return nil, err
				}
				out := make([]json.RawMessage, 0, len(recs))
				for _, r := range recs {
					b, mErr := json.Marshal(r)
					if mErr != nil {
						return nil, mErr
					}
					out = append(out, b)
				}
				return out, nil
			},
			raw: func(c *client.Client) (json.RawMessage, error) {
				return c.GetWithdrawnCirclesJSON(context.Background(), "test-token", "")
			},
		},
	}
}

// TestCirclePaginationMatrix_AllTypesAcrossBothPaths 建立分页行为矩阵：
// 四种 circleType × {结构化, 原始 JSON}，验证
//   - 结果条数一致（分页合并完整）
//   - 记录内容属于自己的 type（不会静默拿到别的 tab 数据）
//   - 记录顺序为 seq 升序（页序确定）
//   - 实际请求页数与页序符合 totalNum 推导
//
// 背景：既有分页测试几乎全部围绕 type=3 建立，type=1/2/4 缺少直接证据。
// 本测试用同一组分页夹具分别喂给两条路径，避免为每种 type 复制相似测试。
func TestCirclePaginationMatrix_AllTypesAcrossBothPaths(t *testing.T) {
	const totalRecords = 7 // pageSize=2 → 4 页
	const wantPages = 4

	modes := []struct {
		name string
		run  func(c *client.Client, tc circleTypeCase) ([]json.RawMessage, error)
	}{
		{"structured", func(c *client.Client, tc circleTypeCase) ([]json.RawMessage, error) {
			return tc.structured(c)
		}},
		{"raw", func(c *client.Client, tc circleTypeCase) ([]json.RawMessage, error) {
			raw, err := tc.raw(c)
			if err != nil {
				return nil, err
			}
			var arr []json.RawMessage
			if uErr := json.Unmarshal(raw, &arr); uErr != nil {
				return nil, uErr
			}
			return arr, nil
		}},
	}

	for _, tc := range matrixCases() {
		for _, mode := range modes {
			t.Run(tc.name+"/"+mode.name, func(t *testing.T) {
				srv, probe := newMatrixServer(t, totalRecords)
				c, err := client.New(
					client.WithTimeout(5*time.Second),
					client.WithBaseURL(srv.URL),
					client.WithSSOBase(srv.URL),
					client.WithSubmittedPageSize(matrixPageSize),
				)
				if err != nil {
					t.Fatalf("构造 Client 失败: %v", err)
				}
				defer c.Close()

				items, err := mode.run(c, tc)
				if err != nil {
					t.Fatalf("%s/%s 取数失败: %v", tc.name, mode.name, err)
				}
				if len(items) != totalRecords {
					t.Fatalf("%s/%s 期望 %d 条，实际 %d", tc.name, mode.name, totalRecords, len(items))
				}

				// 逐条校验：type 归属正确 + 序号严格升序（页序确定）
				for i, item := range items {
					var rec struct {
						Content string `json:"content"`
					}
					if uErr := json.Unmarshal(item, &rec); uErr != nil {
						t.Fatalf("%s/%s 第 %d 条解析失败: %v", tc.name, mode.name, i, uErr)
					}
					wantContent := "type=" + tc.wantType + " seq=" + strconv.Itoa(i+1)
					if rec.Content != wantContent {
						t.Errorf("%s/%s 第 %d 条内容错位：期望 %q，实际 %q（页序可能未保持）",
							tc.name, mode.name, i, wantContent, rec.Content)
					}
				}

				// 请求页序：应恰好覆盖 1..wantPages，且只请求自己的 type
				pages := probe.pagesFor(tc.wantType)
				if len(pages) != wantPages {
					t.Errorf("%s/%s 期望请求 %d 页(type=%s)，实际 %d 页 %v",
						tc.name, mode.name, wantPages, tc.wantType, len(pages), pages)
				}
				seen := make(map[int]bool, len(pages))
				for _, p := range pages {
					seen[p] = true
				}
				for p := 1; p <= wantPages; p++ {
					if !seen[p] {
						t.Errorf("%s/%s 缺少第 %d 页（实际 %v）", tc.name, mode.name, p, pages)
					}
				}
			})
		}
	}
}
