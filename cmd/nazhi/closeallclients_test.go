// closeallclients_test.go 锚定 closeAllClients 的关闭契约。
//
// 背景：OCR 迁移前，Client.Close() 会调用注入识别器的 Close()，
// 测试用失败注入驱动 closeAllClients 的错误路径。移除 OCR 后 Client.Close()
// 只有两个不可出错的动作（Transport.CloseIdleConnections / sm.Reset），
// 失败路径已不可达。本文件保留仍可验证的核心契约：
//   - closeAllClients 正常关闭已登记的 Client，不崩溃
//   - 空列表 / 正常 Client 列表下返回 nil error
//   - 关闭后登记列表被清空（二次调用是 no-op）
//
// 2026-09-26：资源所有权已收口到 ProcessScope，本文件不再直接读写
// legacy 包级表（pendingClients / pendingLogFiles 已删除）。
// Scope 自身的去重、LIFO 与错误聚合契约见 process_scope_close_test.go。
package main

import (
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// resetDefaultScope 清空默认 Scope 的登记列表，让测试从干净状态开始。
func resetDefaultScope(t *testing.T) {
	t.Helper()
	_ = closeAllClients()
	_ = closeLogFiles()
	t.Cleanup(func() {
		_ = closeAllClients()
		_ = closeLogFiles()
	})
}

// TestCloseAllClients_Empty_NoError 验证无登记资源时 closeAllClients
// 返回 nil error。
func TestCloseAllClients_Empty_NoError(t *testing.T) {
	resetDefaultScope(t)

	if err := closeAllClients(); err != nil {
		t.Fatalf("空列表 closeAllClients 应返回 nil error，实际: %v", err)
	}
}

// TestCloseAllClients_NormalClients_NoError 验证多个正常 Client 能依次关闭
// （LIFO 遍历无 panic），返回 nil error。
func TestCloseAllClients_NormalClients_NoError(t *testing.T) {
	resetDefaultScope(t)

	for range 3 {
		c, err := client.New(client.WithTimeout(5 * 1e9))
		if err != nil {
			t.Fatalf("client.New: %v", err)
		}
		trackClient(c)
	}

	if got := defaultScope.TrackedClientCount(); got != 3 {
		t.Fatalf("应登记 3 个 Client，实际 %d", got)
	}
	if err := closeAllClients(); err != nil {
		t.Fatalf("正常 Client 列表 closeAllClients 应返回 nil error，实际: %v", err)
	}
}

// TestCloseAllClients_ClearsRegistry 锁定关闭后登记列表被清空：
// main 的 defer 与 os.Exit 前会各调用一次清理，二次调用必须是 no-op。
func TestCloseAllClients_ClearsRegistry(t *testing.T) {
	resetDefaultScope(t)

	c, err := client.New(client.WithTimeout(5 * 1e9))
	if err != nil {
		t.Fatalf("client.New: %v", err)
	}
	trackClient(c)

	if err := closeAllClients(); err != nil {
		t.Fatalf("首次 closeAllClients 失败: %v", err)
	}
	if got := defaultScope.TrackedClientCount(); got != 0 {
		t.Errorf("关闭后登记列表应清空，实际仍有 %d 个", got)
	}
	if err := closeAllClients(); err != nil {
		t.Fatalf("二次 closeAllClients 应为 no-op，实际: %v", err)
	}
}

// TestCloseLogFiles_ClearsRegistry 同理锁定日志文件登记列表被清空。
func TestCloseLogFiles_ClearsRegistry(t *testing.T) {
	resetDefaultScope(t)

	trackLogFile(nopCloser{})
	if got := defaultScope.TrackedLogFileCount(); got != 1 {
		t.Fatalf("应登记 1 个日志文件，实际 %d", got)
	}
	if err := closeLogFiles(); err != nil {
		t.Fatalf("首次 closeLogFiles 失败: %v", err)
	}
	if got := defaultScope.TrackedLogFileCount(); got != 0 {
		t.Errorf("关闭后登记列表应清空，实际仍有 %d 个", got)
	}
}

// nopCloser 是无副作用的 io.Closer，用于登记/关闭流程测试。
type nopCloser struct{}

func (nopCloser) Close() error { return nil }
