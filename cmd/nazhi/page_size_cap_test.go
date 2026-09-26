package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
)

// honor list / typical-case list 的 --page-size 无上钳。
// 现状：两只命令仅校验 >0 无上界；pkg/client/request.go:73 实测服务端
// pageSize 上限 500。CLI 直接透传超限值让服务端静默截断为 500，分页脚本
// 以错误的 pageSize 计算页数，拿到截断数据却不自知。
// 修复：参数层钳制 pageSize<=500（对齐 SDK defaultSubmittedPageSize 纪律），
// 超限以参数错误拒绝（400/exit3），与现有 ≤0 拒绝语义同族。
// 常量定义已上提 honor.go（Cmd 包共享），本文件引用同源锁定。

// pageSizeListTestCmd 创建带通用业务参数与指定 page-size 的 list 测试命令。
func pageSizeListTestCmd(t *testing.T, baseURL, pageSize string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "list-page-size"}
	cmd.SetContext(context.Background())
	cmd.Flags().Int("page", 1, "")
	cmd.Flags().Int("page-size", 10, "")
	_ = cmd.Flags().Set("page-size", pageSize)
	cmd.Flags().String("key", "", "")
	cmd.Flags().Int("status", 3, "")
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", baseURL)
	cmd.Flags().Int("timeout", 5, "")
	return cmd
}

// pageSizeMockServer 返回可响应预热与业务 path 的 mock server，并记录业务是否到达。
func pageSizeMockServer(t *testing.T, bizPath string) (*httptest.Server, *bool) {
	t.Helper()
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"示例学生","studentNumber":"TEST2025001"}}`))
		default:
			if r.URL.Path == bizPath {
				hit = true
				_, _ = w.Write([]byte(`{"code":1,"msg":"成功","dataList":[],"pageBean":{"pageNo":1,"pageSize":500,"totalNum":0,"totalPage":0}}`))
				return
			}
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &hit
}

// swapListGlobals 统一保存/恢复 quiet/verbose 与退出码。
func swapListGlobals(t *testing.T) {
	t.Helper()
	originalQuiet, originalVerbose := quiet, verbose
	quiet, verbose = false, false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		quiet, verbose = originalQuiet, originalVerbose
		pendingExitCode.Store(0)
	})
}

// TestHonorList_PageSizeCappedAt500 锁定：honor list --page-size=501 拒绝，500 放行。
func TestHonorList_PageSizeCappedAt500(t *testing.T) {
	t.Run("page-size 501 拒绝", func(t *testing.T) {
		cmd := pageSizeListTestCmd(t, "http://127.0.0.1:1", "501")
		swapListGlobals(t)
		_, _, restore := captureStdio(t)
		honorListCmd.Run(cmd, nil)
		restore()
		if pendingExitCode.Load() != 3 {
			t.Errorf("page-size=501 应走参数错误退出码 3，实际 %d", pendingExitCode.Load())
		}
	})

	t.Run("page-size 500 放行并发出业务请求", func(t *testing.T) {
		srv, bizHit := pageSizeMockServer(t, "/api/studentMoralEduNew/getHonorByStudentId")
		cmd := pageSizeListTestCmd(t, srv.URL, "500")
		swapListGlobals(t)
		_, stderr, restore := captureStdio(t)
		honorListCmd.Run(cmd, nil)
		restore()
		if !*bizHit {
			t.Fatalf("page-size=500 应放行并真正发出业务请求；stderr=%s", stderr.String())
		}
		if pendingExitCode.Load() != 0 {
			t.Errorf("page-size=500 不应设置退出码，实际 %d", pendingExitCode.Load())
		}
	})
}

// TestTypicalCaseList_PageSizeCappedAt500 锁定：typical-case list 同款钳制。
func TestTypicalCaseList_PageSizeCappedAt500(t *testing.T) {
	t.Run("page-size 501 拒绝", func(t *testing.T) {
		cmd := pageSizeListTestCmd(t, "http://127.0.0.1:1", "501")
		swapListGlobals(t)
		_, _, restore := captureStdio(t)
		typicalCaseListCmd.Run(cmd, nil)
		restore()
		if pendingExitCode.Load() != 3 {
			t.Errorf("page-size=501 应走参数错误退出码 3，实际 %d", pendingExitCode.Load())
		}
	})

	t.Run("page-size 500 放行并发出业务请求", func(t *testing.T) {
		srv, bizHit := pageSizeMockServer(t, "/api/studentCircleNew/getTypicalCase")
		cmd := pageSizeListTestCmd(t, srv.URL, "500")
		swapListGlobals(t)
		_, stderr, restore := captureStdio(t)
		typicalCaseListCmd.Run(cmd, nil)
		restore()
		if !*bizHit {
			t.Fatalf("page-size=500 应放行并真正发出业务请求；stderr=%s", stderr.String())
		}
		if pendingExitCode.Load() != 0 {
			t.Errorf("page-size=500 不应设置退出码，实际 %d", pendingExitCode.Load())
		}
	})
}
