package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestTaskAndCircleMetadataCommandsRegistered(t *testing.T) {
	for _, command := range []*cobra.Command{
		taskDimensionsCmd,
		taskCircleTypeCmd,
		circleTypesCmd,
		circleTasksCmd,
		circleImagesCmd,
		circleDictCmd,
	} {
		if command.Parent() == nil {
			t.Fatalf("命令 %q 未挂载到父命令", command.Name())
		}
	}
	if taskCircleTypeCmd.Flags().Lookup("task-id") == nil {
		t.Fatal("task circle-type 应暴露 --task-id")
	}
	if circleTypesCmd.Flags().Lookup("dimension-id") == nil || circleTypesCmd.Flags().Lookup("pid") == nil {
		t.Fatal("circle types 应暴露 --dimension-id 和 --pid")
	}
	if circleTasksCmd.Flags().Lookup("type-id") == nil {
		t.Fatal("circle tasks 应暴露 --type-id")
	}
	if circleImagesCmd.Flags().Lookup("page") == nil || circleImagesCmd.Flags().Lookup("page-size") == nil {
		t.Fatal("circle images 应暴露分页参数")
	}
	if circleDictCmd.Flags().Lookup("cate-code") == nil {
		t.Fatal("circle dict 应暴露 --cate-code")
	}
}

func TestMetadataCommands_ForwardSDKQueries(t *testing.T) {
	queries := make(map[string]string)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"示例学生","studentNumber":"TEST2025001"}}`))
		case "/api/studentCircleNew/getDimensions":
			_, _ = w.Write([]byte(`{"code":1,"dataList":[{"id":9,"name":"思想品德"}]}`))
		case "/api/studentCircleNew/getCircleTypeByTaskId":
			queries["circle-type.taskId"] = r.URL.Query().Get("taskId")
			_, _ = w.Write([]byte(`{"code":1,"dataMap":{"task_name":"生产劳动","circle_type_id":9274,"hours":2,"type_name":"生产劳动","dimension_id":14,"dimension_name":"劳动素养","task_id":18154,"remark":"劳动","type":10}}`))
		case "/api/studentCircleNew/getCircleType":
			queries["types.dimensionId"] = r.URL.Query().Get("dimensionId")
			queries["types.pid"] = r.URL.Query().Get("pid")
			_, _ = w.Write([]byte(`{"code":1,"dataList":[{"id":9274,"name":"生产劳动"}]}`))
		case "/api/studentCircleNew/getCircleTask":
			queries["tasks.typeId"] = r.URL.Query().Get("typeId")
			_, _ = w.Write([]byte(`{"code":1,"dataList":[{"id":18154,"name":"3月生产劳动"}]}`))
		case "/api/studentCircleNew/getCircleImg":
			queries["images.pageNo"] = r.URL.Query().Get("pageNo")
			queries["images.pageSize"] = r.URL.Query().Get("pageSize")
			_, _ = w.Write([]byte(`{"code":1,"dataList":[{"id":8,"name":"photo.jpg"}]}`))
		case "/api/common/sys/dict/list":
			queries["dict.cateCode"] = r.URL.Query().Get("cateCode")
			_, _ = w.Write([]byte(`{"code":1,"dataList":[{"value":"5","label":"学校"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	tests := []struct {
		name      string
		command   *cobra.Command
		configure func(*cobra.Command)
		wantBody  string
	}{
		{"dimensions", taskDimensionsCmd, func(cmd *cobra.Command) {}, "思想品德"},
		{"circle-type", taskCircleTypeCmd, func(cmd *cobra.Command) { _ = cmd.Flags().Set("task-id", "18154") }, "9274"},
		{"types", circleTypesCmd, func(cmd *cobra.Command) {
			_ = cmd.Flags().Set("dimension-id", "14")
			_ = cmd.Flags().Set("pid", "root/child")
		}, "生产劳动"},
		{"tasks", circleTasksCmd, func(cmd *cobra.Command) { _ = cmd.Flags().Set("type-id", "9274") }, "3月生产劳动"},
		{"images", circleImagesCmd, func(cmd *cobra.Command) { _ = cmd.Flags().Set("page", "2"); _ = cmd.Flags().Set("page-size", "20") }, "photo.jpg"},
		{"dict", circleDictCmd, func(cmd *cobra.Command) { _ = cmd.Flags().Set("cate-code", "23") }, "学校"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := makeMetadataTestCmd(t, srv.URL)
			tc.configure(cmd)
			originalQuiet, originalVerbose := quiet, verbose
			quiet, verbose = false, false
			pendingExitCode.Store(0)
			t.Cleanup(func() {
				quiet, verbose = originalQuiet, originalVerbose
				pendingExitCode.Store(0)
				_ = closeAllClients()
			})
			stdout, stderr, restore := captureStdio(t)
			tc.command.Run(cmd, nil)
			restore()
			if pendingExitCode.Load() != 0 {
				t.Fatalf("成功查询不应设置退出码=%d，stderr=%s", pendingExitCode.Load(), stderr.String())
			}
			if !strings.Contains(stdout.String(), tc.wantBody) {
				t.Fatalf("返回值未透传 %q: %s", tc.wantBody, stdout.String())
			}
		})
	}
	if queries["circle-type.taskId"] != "18154" || queries["types.dimensionId"] != "14" || queries["types.pid"] != "root/child" || queries["tasks.typeId"] != "9274" || queries["images.pageNo"] != "2" || queries["images.pageSize"] != "20" || queries["dict.cateCode"] != "23" {
		t.Fatalf("元数据查询参数未完整透传: %#v", queries)
	}
}

func TestTaskCircleTypeCmd_RejectsInvalidIDWithoutRequest(t *testing.T) {
	cmd := makeMetadataTestCmd(t, "http://127.0.0.1:1")
	_ = cmd.Flags().Set("task-id", "0")
	originalQuiet, originalVerbose := quiet, verbose
	quiet, verbose = false, false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		quiet, verbose = originalQuiet, originalVerbose
		pendingExitCode.Store(0)
	})
	_, stderr, restore := captureStdio(t)
	taskCircleTypeCmd.Run(cmd, nil)
	restore()
	if !strings.Contains(stderr.String(), "--task-id") {
		t.Fatalf("非法 task-id 应输出参数错误: %s", stderr.String())
	}
	if pendingExitCode.Load() != 3 {
		t.Fatalf("非法 task-id 应设置退出码 3，实际 %d", pendingExitCode.Load())
	}
}

func makeMetadataTestCmd(t *testing.T, baseURL string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "metadata-test"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", baseURL)
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().Int64("task-id", 0, "")
	cmd.Flags().Int64("dimension-id", 0, "")
	cmd.Flags().String("pid", "", "")
	cmd.Flags().Int64("type-id", 0, "")
	cmd.Flags().Int("page", 1, "")
	cmd.Flags().Int("page-size", 10, "")
	cmd.Flags().Int("cate-code", 0, "")
	return cmd
}
