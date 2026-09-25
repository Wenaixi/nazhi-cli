package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/spf13/cobra"
)

// circleTypeProbeServer 记录 getStudentCircle 请求的 type 参数，
// 并按 type 返回可区分的记录内容，使「命令请求了错误的 tab」成为可观察行为。
//
// 背景：四个写实列表命令（public/teacher/submitted/withdrawn）分别对应
// getStudentCircle 的 type=1/2/3/4。type 是本族唯一「配错了不报错、
// 只静默返回别的 tab 数据」的维度，而既有测试只比对 URL.Path，
// 对四个 type 返回完全相同的响应，因此无法发现 type 配错。
type circleTypeProbe struct {
	mu     sync.Mutex
	types  []string
	keys   []string
	byType map[string]int
}

func newCircleTypeProbeServer(t *testing.T) (*httptest.Server, *circleTypeProbe) {
	t.Helper()
	probe := &circleTypeProbe{byType: make(map[string]int)}

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
			circleType := r.URL.Query().Get("type")
			key := r.URL.Query().Get("key")

			probe.mu.Lock()
			probe.types = append(probe.types, circleType)
			probe.keys = append(probe.keys, key)
			probe.byType[circleType]++
			probe.mu.Unlock()

			// 每种 type 返回不同 id 前缀：命令拿到别的 type 的数据即可被断言发现
			body, _ := json.Marshal(map[string]any{
				"code": 1, "msg": "成功",
				"dataList": []map[string]any{
					{"id": 1, "content": "type=" + circleType, "circle_type_tag": circleType},
				},
				"pageBean": map[string]any{
					"pageNo": 1, "pageSize": 500, "totalPage": 1, "totalNum": 1,
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

func (p *circleTypeProbe) observedTypes() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, len(p.types))
	copy(out, p.types)
	return out
}

func (p *circleTypeProbe) observedKeys() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, len(p.keys))
	copy(out, p.keys)
	return out
}

// makeTypeProbeCmd 构造带业务参数的命令实例。
func makeTypeProbeCmd(srvURL string) *cobra.Command {
	cmd := &cobra.Command{Use: "task-type-probe"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", srvURL)
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().Bool("count", false, "")
	cmd.Flags().Int("offset", 0, "")
	cmd.Flags().Int("limit", 0, "")
	cmd.Flags().String("key", "", "")
	return cmd
}

// TestTaskCirclesCommands_RequestCorrectCircleType 锁定四个写实列表命令
// 各自请求正确的 getStudentCircle type：
//
//	public   → type=1（公示/全部）
//	teacher  → type=2（教师写实）
//	submitted→ type=3（我发布的）
//	withdrawn→ type=4（被撤回）
//
// 这是深化共享实现前必须先有的证据：type 配错不会报错，只会静默返回
// 别的 tab 数据，脚本拿到错误数据却以为成功。
func TestTaskCirclesCommands_RequestCorrectCircleType(t *testing.T) {
	cases := []struct {
		name     string
		run      func(cmd *cobra.Command, args []string)
		wantType string
	}{
		{"public", taskPublicCmd.Run, "1"},
		{"teacher", taskTeacherCmd.Run, "2"},
		{"submitted", taskSubmittedCmd.Run, "3"},
		{"withdrawn", taskWithdrawnCmd.Run, "4"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, probe := newCircleTypeProbeServer(t)
			cmd := makeTypeProbeCmd(srv.URL)
			swapListGlobals(t)
			_, _, restore := captureStdio(t)
			tc.run(cmd, nil)
			restore()

			got := probe.observedTypes()
			if len(got) == 0 {
				t.Fatalf("%s 命令未发出任何 getStudentCircle 请求", tc.name)
			}
			for _, typ := range got {
				if typ != tc.wantType {
					t.Errorf("%s 命令应请求 type=%s，实际请求 type=%s（全部观测: %v）",
						tc.name, tc.wantType, typ, got)
				}
			}
		})
	}
}

// TestTaskCirclesCommands_ForwardKeyToCircleQuery 锁定 --key 透传：
// 四个命令都必须把 key 传给 getStudentCircle 的 key 查询参数。
func TestTaskCirclesCommands_ForwardKeyToCircleQuery(t *testing.T) {
	cases := []struct {
		name string
		run  func(cmd *cobra.Command, args []string)
	}{
		{"public", taskPublicCmd.Run},
		{"teacher", taskTeacherCmd.Run},
		{"submitted", taskSubmittedCmd.Run},
		{"withdrawn", taskWithdrawnCmd.Run},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, probe := newCircleTypeProbeServer(t)
			cmd := makeTypeProbeCmd(srv.URL)
			_ = cmd.Flags().Set("key", "劳动")
			swapListGlobals(t)
			_, _, restore := captureStdio(t)
			tc.run(cmd, nil)
			restore()

			keys := probe.observedKeys()
			if len(keys) == 0 {
				t.Fatalf("%s 命令未发出任何 getStudentCircle 请求", tc.name)
			}
			for _, k := range keys {
				if k != "劳动" {
					t.Errorf("%s 命令应透传 key=劳动，实际 %q（全部观测: %v）",
						tc.name, k, keys)
				}
			}
		})
	}
}

// TestTaskCirclesCommands_CountModeRequestsCorrectType 锁定 --count 模式：
// count 走 Peek*Total，同样必须请求正确的 type。
func TestTaskCirclesCommands_CountModeRequestsCorrectType(t *testing.T) {
	cases := []struct {
		name     string
		run      func(cmd *cobra.Command, args []string)
		wantType string
	}{
		{"public", taskPublicCmd.Run, "1"},
		{"teacher", taskTeacherCmd.Run, "2"},
		{"submitted", taskSubmittedCmd.Run, "3"},
		{"withdrawn", taskWithdrawnCmd.Run, "4"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, probe := newCircleTypeProbeServer(t)
			cmd := makeTypeProbeCmd(srv.URL)
			_ = cmd.Flags().Set("count", "true")
			swapListGlobals(t)
			_, _, restore := captureStdio(t)
			tc.run(cmd, nil)
			restore()

			got := probe.observedTypes()
			if len(got) == 0 {
				t.Fatalf("%s --count 未发出任何 getStudentCircle 请求", tc.name)
			}
			for _, typ := range got {
				if typ != tc.wantType {
					t.Errorf("%s --count 应请求 type=%s，实际 type=%s（全部观测: %v）",
						tc.name, tc.wantType, typ, got)
				}
			}
		})
	}
}
