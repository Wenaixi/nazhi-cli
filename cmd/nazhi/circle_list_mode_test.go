package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
)

// circleListModeCase 描述一个写实列表命令的入口。
// 四个命令（public/teacher/submitted/withdrawn）共享同一套模式逻辑
// （count / limit / 全量），本表把「模式 → envelope 形状」写成可复用断言，
// 避免为每个命令复制一份相似测试。
type circleListModeCase struct {
	name string
	run  func(cmd *cobra.Command, args []string)
}

func listModeCases() []circleListModeCase {
	return []circleListModeCase{
		{"public", taskPublicCmd.Run},
		{"teacher", taskTeacherCmd.Run},
		{"submitted", taskSubmittedCmd.Run},
		{"withdrawn", taskWithdrawnCmd.Run},
	}
}

// setupListModeServer 构造覆盖激活四步与 getStudentCircle 的 mock。
// totalNum 决定 --count 与 --limit 的 total 输出。
func setupListModeServer(t *testing.T, totalNum, recordsPerPage int) *httptest.Server {
	t.Helper()
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
			records := make([]map[string]any, 0, recordsPerPage)
			for i := 1; i <= recordsPerPage; i++ {
				records = append(records, map[string]any{
					"id":      i,
					"content": "rec",
					"status":  0,
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 1, "msg": "成功",
				"dataList": records,
				"pageBean": map[string]any{
					"pageNo": 1, "pageSize": recordsPerPage,
					"totalPage": 1, "totalNum": totalNum,
				},
			})
		default:
			t.Errorf("未预期请求: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newListModeCmd(srvURL string) *cobra.Command {
	cmd := &cobra.Command{Use: "task-list-mode"}
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

// decodedEnvelope 是 CLI envelope 的可断言投影。
type decodedEnvelope struct {
	Status string          `json:"status"`
	Code   int             `json:"code"`
	Data   json.RawMessage `json:"data"`
}

// decodeEnvelope 解析 CLI 输出的 envelope。
// envelope 是缩进 JSON（printEnvelope 走 MarshalIndent），
// 因此断言必须解析后比较字段，不能用单行字符串匹配。
func decodeEnvelope(t *testing.T, out string) decodedEnvelope {
	t.Helper()
	var env decodedEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("envelope 解析失败: %v\n原始输出:\n%s", err, out)
	}
	return env
}

// TestCircleListCommands_CountModeEnvelopeShape 锁定四个命令在 --count
// 模式下一致的 envelope 形状：status=success，data={"total":N}。
//
// 背景：四个 Run 回调此前各写一份 count 分支，形状一致性无测试保护。
// 提取共享实现前必须先有该证据，否则无法判断重构是否改变了输出契约。
func TestCircleListCommands_CountModeEnvelopeShape(t *testing.T) {
	for _, tc := range listModeCases() {
		t.Run(tc.name, func(t *testing.T) {
			srv := setupListModeServer(t, 321, 5)
			cmd := newListModeCmd(srv.URL)
			_ = cmd.Flags().Set("count", "true")
			swapListGlobals(t)
			stdout, _, restore := captureStdio(t)
			tc.run(cmd, nil)
			restore()

			env := decodeEnvelope(t, stdout.String())
			if env.Status != "success" {
				t.Errorf("%s --count 应 status=success，实际 %q", tc.name, env.Status)
			}
			var payload struct {
				Total int `json:"total"`
			}
			if err := json.Unmarshal(env.Data, &payload); err != nil {
				t.Fatalf("%s --count data 应为 {total:N}，解析失败: %v", tc.name, err)
			}
			if payload.Total != 321 {
				t.Errorf("%s --count 应输出 total=321，实际 %d", tc.name, payload.Total)
			}
			if pendingExitCode.Load() != 0 {
				t.Errorf("%s --count 成功时不应设置退出码，实际 %d", tc.name, pendingExitCode.Load())
			}
		})
	}
}

// TestCircleListCommands_FullModeEnvelopeShape 锁定全量模式的 envelope：
// status=success，且 data 是裸记录数组（不是 {records,total} 包装）。
func TestCircleListCommands_FullModeEnvelopeShape(t *testing.T) {
	for _, tc := range listModeCases() {
		t.Run(tc.name, func(t *testing.T) {
			srv := setupListModeServer(t, 5, 5)
			cmd := newListModeCmd(srv.URL)
			swapListGlobals(t)
			stdout, _, restore := captureStdio(t)
			tc.run(cmd, nil)
			restore()

			env := decodeEnvelope(t, stdout.String())
			if env.Status != "success" {
				t.Errorf("%s 全量模式应 status=success，实际 %q", tc.name, env.Status)
			}
			var arr []json.RawMessage
			if err := json.Unmarshal(env.Data, &arr); err != nil {
				t.Fatalf("%s 全量模式 data 应为裸数组，解析失败: %v", tc.name, err)
			}
			if len(arr) != 5 {
				t.Errorf("%s 全量模式应返回 5 条，实际 %d", tc.name, len(arr))
			}
		})
	}
}

// TestCircleListCommands_LimitModeEnvelopeShape 锁定 limit 模式的 envelope：
// data 是 {records,total} 包装（与全量的裸数组形状不同）。
func TestCircleListCommands_LimitModeEnvelopeShape(t *testing.T) {
	for _, tc := range listModeCases() {
		t.Run(tc.name, func(t *testing.T) {
			srv := setupListModeServer(t, 42, 5)
			cmd := newListModeCmd(srv.URL)
			_ = cmd.Flags().Set("limit", "2")
			swapListGlobals(t)
			stdout, _, restore := captureStdio(t)
			tc.run(cmd, nil)
			restore()

			env := decodeEnvelope(t, stdout.String())
			if env.Status != "success" {
				t.Errorf("%s --limit 应 status=success，实际 %q", tc.name, env.Status)
			}
			var payload struct {
				Records json.RawMessage `json:"records"`
				Total   int             `json:"total"`
			}
			if err := json.Unmarshal(env.Data, &payload); err != nil {
				t.Fatalf("%s --limit data 应为 {records,total}，解析失败: %v", tc.name, err)
			}
			if payload.Total != 42 {
				t.Errorf("%s --limit 应输出 total=42，实际 %d", tc.name, payload.Total)
			}
			var arr []json.RawMessage
			if err := json.Unmarshal(payload.Records, &arr); err != nil {
				t.Fatalf("%s --limit records 应为数组，解析失败: %v", tc.name, err)
			}
			if len(arr) == 0 {
				t.Errorf("%s --limit records 不应为空", tc.name)
			}
		})
	}
}
