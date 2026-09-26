package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// 这组测试锁定「写实列表类型」具名接口的行为，并证明它与旧的按类型各设
// 一份的入口逐字等价。
//
// 背景：写实列表类型在 CONTEXT.md 里有正式名字（公示 / 教师写实 / 我发布的 /
// 被撤回的），但代码里只是一串裸 int，名字只活在 raw_json.go 的注释块里。
// 后果是同一个浅模块被复制了两遍：SDK 侧 8 个入口方法各 1-3 行转发，
// CLI 侧 circle_list_mode.go 又是一张 52 行的模式表，每个 mode 三个函数
// 指针全是转发。调用方要学 12 个入口，实际需要的知识只有一份。

// TestCircleListTypeConstants 四个具名常量必须映射到平台既有的 type 参数值。
// 这些值是 getStudentCircle 的线协议约定，改动即破坏兼容。
func TestCircleListTypeConstants(t *testing.T) {
	cases := []struct {
		listType client.CircleListType
		want     int
	}{
		{client.CircleListPublic, 1},
		{client.CircleListTeacher, 2},
		{client.CircleListSubmitted, 3},
		{client.CircleListWithdrawn, 4},
	}
	for _, tc := range cases {
		if got := int(tc.listType); got != tc.want {
			t.Errorf("写实列表类型 %v 的线协议值 = %d，期望 %d", tc.listType, got, tc.want)
		}
	}
}

// TestCircleListTypeLabels 每个类型必须有中文领域名，供 CLI 文案与错误信息使用。
func TestCircleListTypeLabels(t *testing.T) {
	cases := map[client.CircleListType]string{
		client.CircleListPublic:    "公示",
		client.CircleListTeacher:   "教师写实",
		client.CircleListSubmitted: "我发布的",
		client.CircleListWithdrawn: "被撤回",
	}
	for listType, want := range cases {
		if got := listType.Label(); got != want {
			t.Errorf("写实列表类型 %d 的领域名 = %q，期望 %q", int(listType), got, want)
		}
	}
}

// TestCircleListTypeValid 非法类型必须可判定，供 CLI 把用户输入挡在发请求之前。
func TestCircleListTypeValid(t *testing.T) {
	for _, listType := range []client.CircleListType{client.CircleListPublic, client.CircleListTeacher, client.CircleListSubmitted, client.CircleListWithdrawn} {
		if !listType.Valid() {
			t.Errorf("类型 %d 应为合法", int(listType))
		}
	}
	// 穷举：只有线协议约定的 1-4 合法。这条能杀掉「Valid 的 switch 误加
	// 一个值」这类变异——逐个列举非法值时很容易漏掉刚被加进去的那个。
	for v := -128; v <= 127; v++ {
		got := client.CircleListType(v).Valid()
		want := v >= int(client.CircleListPublic) && v <= int(client.CircleListWithdrawn)
		if got != want {
			t.Errorf("类型 %d 的 Valid() = %v，期望 %v（合法区间 1-4）", v, got, want)
		}
	}
	for _, bad := range []client.CircleListType{0, 5, -1, 99} {
		if bad.Valid() {
			t.Errorf("类型 %d 不应被判定为合法", int(bad))
		}
	}
}

// TestCircleListTypeFromValue 支持从平台 type 参数反查列表类型，
// 让 CLI 的 --type 解析与 SDK 共享同一份映射。
func TestCircleListTypeFromValue(t *testing.T) {
	for want := client.CircleListPublic; want <= client.CircleListWithdrawn; want++ {
		got, ok := client.CircleListTypeFromValue(int(want))
		if !ok {
			t.Errorf("值 %d 应能反查为写实列表类型", int(want))
			continue
		}
		if got != want {
			t.Errorf("值 %d 反查得 %v，期望 %v", int(want), got, want)
		}
	}
	if _, ok := client.CircleListTypeFromValue(5); ok {
		t.Errorf("值 5 不应反查成功")
	}
}

// newCircleListTypeServer 起一个按 type 参数应答的写实列表服务端。
//
// 关键：响应内容随 type 参数变化。这是让「薄壳转发到错误的写实列表类型」
// 可被发现的必要条件——若服务端对所有 type 返回相同数据，薄壳与统一入口
// 即使绑错同一类型也比不出差异。
func newCircleListTypeServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/getStudentCircle" {
			http.NotFound(w, r)
			return
		}
		listType := r.URL.Query().Get("type")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 1,
			"dataList": []map[string]any{
				{"id": 1, "name": "type-" + listType, "listType": listType},
			},
			"pageBean": map[string]any{
				"pageNo": 1, "pageSize": 2, "totalNum": 2, "totalPage": 1,
			},
		})
	})))
}

// TestOldEntrypoints_BindToCorrectListType 逐一锁定旧入口薄壳各自绑定的写实
// 列表类型。薄壳唯一的职责就是把「这个名字」翻译成「这个类型」——翻译错了
// 调用方会静默拿到另一个列表的数据，正是本项目反复误读的那个概念。
func TestOldEntrypoints_BindToCorrectListType(t *testing.T) {
	cases := []struct {
		name string
		want client.CircleListType
		call func(c *client.Client) (json.RawMessage, error)
	}{
		{"GetPublicCirclesJSON", client.CircleListPublic, func(c *client.Client) (json.RawMessage, error) {
			return c.GetPublicCirclesJSON(context.Background(), "test-token", "")
		}},
		{"GetTeacherCirclesJSON", client.CircleListTeacher, func(c *client.Client) (json.RawMessage, error) {
			return c.GetTeacherCirclesJSON(context.Background(), "test-token", "")
		}},
		{"GetSubmittedCirclesJSON", client.CircleListSubmitted, func(c *client.Client) (json.RawMessage, error) {
			return c.GetSubmittedCirclesJSON(context.Background(), "test-token", "")
		}},
		{"GetWithdrawnCirclesJSON", client.CircleListWithdrawn, func(c *client.Client) (json.RawMessage, error) {
			return c.GetWithdrawnCirclesJSON(context.Background(), "test-token", "")
		}},
		// 分页入口薄壳
		{"GetPublicCirclesLimitJSON", client.CircleListPublic, func(c *client.Client) (json.RawMessage, error) {
			raw, _, err := c.GetPublicCirclesLimitJSON(context.Background(), "test-token", 0, 10, "")
			return raw, err
		}},
		{"GetTeacherCirclesLimitJSON", client.CircleListTeacher, func(c *client.Client) (json.RawMessage, error) {
			raw, _, err := c.GetTeacherCirclesLimitJSON(context.Background(), "test-token", 0, 10, "")
			return raw, err
		}},
		{"GetSubmittedCirclesLimitJSON", client.CircleListSubmitted, func(c *client.Client) (json.RawMessage, error) {
			raw, _, err := c.GetSubmittedCirclesLimitJSON(context.Background(), "test-token", 0, 10, "")
			return raw, err
		}},
		{"GetWithdrawnCirclesLimitJSON", client.CircleListWithdrawn, func(c *client.Client) (json.RawMessage, error) {
			raw, _, err := c.GetWithdrawnCirclesLimitJSON(context.Background(), "test-token", 0, 10, "")
			return raw, err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newCircleListTypeServer(t)
			defer srv.Close()
			c, err := client.New(client.WithBaseURL(srv.URL), client.WithHTTPClient(srv.Client()))
			if err != nil {
				t.Fatalf("建客户端失败: %v", err)
			}
			raw, err := tc.call(c)
			if err != nil {
				t.Fatalf("%s 调用失败: %v", tc.name, err)
			}
			wantMarker := `"listType":"` + strconv.Itoa(int(tc.want)) + `"`
			if !strings.Contains(string(raw), wantMarker) {
				t.Errorf("%s 应请求写实列表类型 %d（%s），实际响应: %s",
					tc.name, int(tc.want), tc.want.Label(), raw)
			}
		})
	}
}

// TestOldStructuredEntrypoints_BindToCorrectListType 结构化路径的四个旧入口
// 同样要锁定各自绑定的写实列表类型。它们返回 []types.CircleRecord 而非
// 原始 JSON，与上面的透传路径分表断言。
func TestOldStructuredEntrypoints_BindToCorrectListType(t *testing.T) {
	cases := []struct {
		name string
		want client.CircleListType
		call func(c *client.Client) ([]types.CircleRecord, error)
	}{
		{"GetPublicCircles", client.CircleListPublic, func(c *client.Client) ([]types.CircleRecord, error) {
			return c.GetPublicCircles(context.Background(), "test-token", "")
		}},
		{"GetTeacherCircles", client.CircleListTeacher, func(c *client.Client) ([]types.CircleRecord, error) {
			return c.GetTeacherCircles(context.Background(), "test-token", "")
		}},
		{"GetSubmittedCircles", client.CircleListSubmitted, func(c *client.Client) ([]types.CircleRecord, error) {
			return c.GetSubmittedCircles(context.Background(), "test-token", "")
		}},
		{"GetWithdrawnCircles", client.CircleListWithdrawn, func(c *client.Client) ([]types.CircleRecord, error) {
			return c.GetWithdrawnCircles(context.Background(), "test-token", "")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newCircleListTypeServer(t)
			defer srv.Close()
			c, err := client.New(client.WithBaseURL(srv.URL), client.WithHTTPClient(srv.Client()))
			if err != nil {
				t.Fatalf("建客户端失败: %v", err)
			}
			records, err := tc.call(c)
			if err != nil {
				t.Fatalf("%s 调用失败: %v", tc.name, err)
			}
			if len(records) == 0 {
				t.Fatalf("%s 未取到记录", tc.name)
			}
			// 服务端把收到的 type 参数回显在记录的 name 字段里
			want := "type-" + strconv.Itoa(int(tc.want))
			if records[0].Name != want {
				t.Errorf("%s 应请求写实列表类型 %d（%s），实际取回 name=%q",
					tc.name, int(tc.want), tc.want.Label(), records[0].Name)
			}
		})
	}
}

// TestListCirclesJSON_EquivOldEntrypoints 新入口按写实列表类型取数，
// 必须与旧的按类型各设一份的入口返回完全相同的字节。
func TestListCirclesJSON_EquivOldEntrypoints(t *testing.T) {
	pairs := []struct {
		listType client.CircleListType
		oldAll   func(c *client.Client) (json.RawMessage, error)
	}{
		{client.CircleListPublic, func(c *client.Client) (json.RawMessage, error) {
			return c.GetPublicCirclesJSON(context.Background(), "test-token", "")
		}},
		{client.CircleListTeacher, func(c *client.Client) (json.RawMessage, error) {
			return c.GetTeacherCirclesJSON(context.Background(), "test-token", "")
		}},
		{client.CircleListSubmitted, func(c *client.Client) (json.RawMessage, error) {
			return c.GetSubmittedCirclesJSON(context.Background(), "test-token", "")
		}},
		{client.CircleListWithdrawn, func(c *client.Client) (json.RawMessage, error) {
			return c.GetWithdrawnCirclesJSON(context.Background(), "test-token", "")
		}},
	}
	for _, p := range pairs {
		t.Run(p.listType.Label(), func(t *testing.T) {
			// 两次各自起独立服务端，避免页数缓存之类的状态互相影响
			srvNew := newCircleListTypeServer(t)
			defer srvNew.Close()
			cNew, err := client.New(client.WithBaseURL(srvNew.URL), client.WithHTTPClient(srvNew.Client()))
			if err != nil {
				t.Fatalf("建客户端失败: %v", err)
			}
			gotNew, err := cNew.ListCirclesJSON(context.Background(), "test-token", p.listType, "")
			if err != nil {
				t.Fatalf("新入口失败: %v", err)
			}

			srvOld := newCircleListTypeServer(t)
			defer srvOld.Close()
			cOld, err := client.New(client.WithBaseURL(srvOld.URL), client.WithHTTPClient(srvOld.Client()))
			if err != nil {
				t.Fatalf("建客户端失败: %v", err)
			}
			gotOld, err := p.oldAll(cOld)
			if err != nil {
				t.Fatalf("旧入口失败: %v", err)
			}

			if string(gotNew) != string(gotOld) {
				t.Errorf("新旧入口字节不一致：\n新: %s\n旧: %s", gotNew, gotOld)
			}
		})
	}
}

// TestListCirclesLimitJSON_EquivOldEntrypoints 分页入口同样必须逐字等价，
// 且保留 PageBean——全量模式下丢弃分页元数据是已修复过的回归。
func TestListCirclesLimitJSON_EquivOldEntrypoints(t *testing.T) {
	pairs := []struct {
		listType client.CircleListType
		oldLimit func(c *client.Client) (json.RawMessage, *types.PageBean, error)
	}{
		{client.CircleListPublic, func(c *client.Client) (json.RawMessage, *types.PageBean, error) {
			return c.GetPublicCirclesLimitJSON(context.Background(), "test-token", 0, 10, "")
		}},
		{client.CircleListSubmitted, func(c *client.Client) (json.RawMessage, *types.PageBean, error) {
			return c.GetSubmittedCirclesLimitJSON(context.Background(), "test-token", 0, 10, "")
		}},
		{client.CircleListWithdrawn, func(c *client.Client) (json.RawMessage, *types.PageBean, error) {
			return c.GetWithdrawnCirclesLimitJSON(context.Background(), "test-token", 0, 10, "")
		}},
	}
	for _, p := range pairs {
		t.Run(p.listType.Label(), func(t *testing.T) {
			srvNew := newCircleListTypeServer(t)
			defer srvNew.Close()
			cNew, err := client.New(client.WithBaseURL(srvNew.URL), client.WithHTTPClient(srvNew.Client()))
			if err != nil {
				t.Fatalf("建客户端失败: %v", err)
			}
			rawNew, pbNew, err := cNew.ListCirclesLimitJSON(context.Background(), "test-token", p.listType, 0, 10, "")
			if err != nil {
				t.Fatalf("新分页入口失败: %v", err)
			}

			srvOld := newCircleListTypeServer(t)
			defer srvOld.Close()
			cOld, err := client.New(client.WithBaseURL(srvOld.URL), client.WithHTTPClient(srvOld.Client()))
			if err != nil {
				t.Fatalf("建客户端失败: %v", err)
			}
			rawOld, pbOld, err := p.oldLimit(cOld)
			if err != nil {
				t.Fatalf("旧分页入口失败: %v", err)
			}

			if string(rawNew) != string(rawOld) {
				t.Errorf("新旧分页入口字节不一致：\n新: %s\n旧: %s", rawNew, rawOld)
			}
			if pbNew == nil || pbOld == nil {
				t.Fatalf("分页元数据不得丢弃：新=%v 旧=%v", pbNew, pbOld)
			}
			if pbNew.TotalNum != pbOld.TotalNum || pbNew.TotalPage != pbOld.TotalPage {
				t.Errorf("分页元数据不一致：新=%+v 旧=%+v", pbNew, pbOld)
			}
		})
	}
}

// TestPeekCircleTotal_EquivOldEntrypoints 总数预取入口同样必须等价。
func TestPeekCircleTotal_EquivOldEntrypoints(t *testing.T) {
	pairs := []struct {
		listType client.CircleListType
		oldPeek  func(c *client.Client) (int, error)
	}{
		{client.CircleListPublic, func(c *client.Client) (int, error) { return c.PeekPublicTotal(context.Background(), "test-token", "") }},
		{client.CircleListTeacher, func(c *client.Client) (int, error) { return c.PeekTeacherTotal(context.Background(), "test-token", "") }},
		{client.CircleListSubmitted, func(c *client.Client) (int, error) {
			return c.PeekSubmittedTotal(context.Background(), "test-token", "")
		}},
		{client.CircleListWithdrawn, func(c *client.Client) (int, error) {
			return c.PeekWithdrawnTotal(context.Background(), "test-token", "")
		}},
	}
	for _, p := range pairs {
		t.Run(p.listType.Label(), func(t *testing.T) {
			srvNew := newCircleListTypeServer(t)
			defer srvNew.Close()
			cNew, err := client.New(client.WithBaseURL(srvNew.URL), client.WithHTTPClient(srvNew.Client()))
			if err != nil {
				t.Fatalf("建客户端失败: %v", err)
			}
			gotNew, err := cNew.PeekCircleTotal(context.Background(), "test-token", p.listType, "")
			if err != nil {
				t.Fatalf("新预取入口失败: %v", err)
			}

			srvOld := newCircleListTypeServer(t)
			defer srvOld.Close()
			cOld, err := client.New(client.WithBaseURL(srvOld.URL), client.WithHTTPClient(srvOld.Client()))
			if err != nil {
				t.Fatalf("建客户端失败: %v", err)
			}
			gotOld, err := p.oldPeek(cOld)
			if err != nil {
				t.Fatalf("旧预取入口失败: %v", err)
			}
			if gotNew != gotOld {
				t.Errorf("新旧预取入口结果不一致：新=%d 旧=%d", gotNew, gotOld)
			}
		})
	}
}

// TestListCirclesJSON_RejectsInvalidType 非法写实列表类型必须在发请求前被拒。
// 服务端声明值驱动路径选择，传错值会静默取到另一个列表的数据。
func TestListCirclesJSON_RejectsInvalidType(t *testing.T) {
	srv := newCircleListTypeServer(t)
	defer srv.Close()
	c, err := client.New(client.WithBaseURL(srv.URL), client.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("建客户端失败: %v", err)
	}
	for _, bad := range []client.CircleListType{0, 5, -1} {
		if _, err := c.ListCirclesJSON(context.Background(), "test-token", bad, ""); err == nil {
			t.Errorf("非法写实列表类型 %d 应被拒绝，不得发出请求", int(bad))
		}
	}
}
