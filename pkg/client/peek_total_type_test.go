package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// peekTypeProbeServer 记录 getStudentCircle 请求的 type 参数与 pageSize，
// 用于锁定四个 Peek*Total 各自的路由与「只取首页一条」的意图。
type peekTypeProbe struct {
	types    []string
	pageSize []string
	pageNo   []string
}

func newPeekTypeProbeServer(t *testing.T, totalNum int) (*httptest.Server, *peekTypeProbe) {
	t.Helper()
	probe := &peekTypeProbe{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","studentNumber":"TEST2025001","schoolId":173,"schoolName":"示例中学","className":"高一(8)班","seat":45}}`))
		case "/api/studentCircleNew/getStudentCircle":
			probe.types = append(probe.types, r.URL.Query().Get("type"))
			probe.pageSize = append(probe.pageSize, r.URL.Query().Get("pageSize"))
			probe.pageNo = append(probe.pageNo, r.URL.Query().Get("pageNo"))
			body, _ := json.Marshal(map[string]any{
				"code": 1, "msg": "成功",
				"dataList": []map[string]any{{"id": 1}},
				"pageBean": map[string]any{
					"pageNo": 1, "pageSize": 1, "totalPage": 1, "totalNum": totalNum,
				},
			})
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		default:
			t.Errorf("未预期请求: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, probe
}

// TestPeekTotalMethods_RequestCorrectCircleType 锁定四个 Peek*Total 各自的
// getStudentCircle type，并断言它们返回服务端声明的 totalNum。
//
// 这四个方法此前只有 PeekSubmittedTotal 有测试覆盖，另外三个零断言；
// type 是「配错了不报错、只静默返回别的 tab 数据」的维度。
func TestPeekTotalMethods_RequestCorrectCircleType(t *testing.T) {
	cases := []struct {
		name     string
		peek     func(c *client.Client, ctx context.Context, token, key string) (int, error)
		wantType string
	}{
		{"PeekPublicTotal", func(c *client.Client, ctx context.Context, token, key string) (int, error) {
			return c.PeekPublicTotal(ctx, token, key)
		}, "1"},
		{"PeekTeacherTotal", func(c *client.Client, ctx context.Context, token, key string) (int, error) {
			return c.PeekTeacherTotal(ctx, token, key)
		}, "2"},
		{"PeekSubmittedTotal", func(c *client.Client, ctx context.Context, token, key string) (int, error) {
			return c.PeekSubmittedTotal(ctx, token, key)
		}, "3"},
		{"PeekWithdrawnTotal", func(c *client.Client, ctx context.Context, token, key string) (int, error) {
			return c.PeekWithdrawnTotal(ctx, token, key)
		}, "4"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, probe := newPeekTypeProbeServer(t, 4321)
			c, err := client.New(
				client.WithTimeout(5*time.Second),
				client.WithBaseURL(srv.URL),
				client.WithSSOBase(srv.URL),
			)
			if err != nil {
				t.Fatalf("构造 Client 失败: %v", err)
			}
			defer c.Close()

			total, err := tc.peek(c, context.Background(), "test-token", "")
			if err != nil {
				t.Fatalf("%s 失败: %v", tc.name, err)
			}
			if total != 4321 {
				t.Errorf("%s 应返回 totalNum=4321，实际 %d", tc.name, total)
			}
			if len(probe.types) == 0 {
				t.Fatalf("%s 未发出 getStudentCircle 请求", tc.name)
			}
			for i, typ := range probe.types {
				if typ != tc.wantType {
					t.Errorf("%s 应请求 type=%s，实际 type=%s（全部: %v）",
						tc.name, tc.wantType, typ, probe.types)
				}
				// Peek 的意图是「只看总数」，不应拉取整页数据
				if probe.pageSize[i] != "1" {
					t.Errorf("%s 应请求 pageSize=1，实际 %q", tc.name, probe.pageSize[i])
				}
				if probe.pageNo[i] != "1" {
					t.Errorf("%s 应请求 pageNo=1，实际 %q", tc.name, probe.pageNo[i])
				}
			}
		})
	}
}

// TestPeekTotalMethods_ZeroTotalNotError 锁定空数据契约：
// 服务端返回 totalNum=0 时应返回 (0, nil)，不是错误。
func TestPeekTotalMethods_ZeroTotalNotError(t *testing.T) {
	srv, _ := newPeekTypeProbeServer(t, 0)
	c, err := client.New(
		client.WithTimeout(5*time.Second),
		client.WithBaseURL(srv.URL),
		client.WithSSOBase(srv.URL),
	)
	if err != nil {
		t.Fatalf("构造 Client 失败: %v", err)
	}
	defer c.Close()

	total, err := c.PeekSubmittedTotal(context.Background(), "test-token", "")
	if err != nil {
		t.Fatalf("totalNum=0 时不应报错，实际: %v", err)
	}
	if total != 0 {
		t.Errorf("期望 total=0，实际 %d", total)
	}
}
