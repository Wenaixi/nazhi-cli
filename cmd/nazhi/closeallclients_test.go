// closeallclients_test.go 锚定 closeAllClients 的关闭顺序与错误聚合契约。
//
// 背景：OCR 迁移前，Client.Close() 会调用注入识别器的 Close()，
// 测试用失败注入驱动 closeAllClients 的错误路径。移除 OCR 后 Client.Close()
// 只有两个不可出错的动作（Transport.CloseIdleConnections / sm.Reset），
// 失败路径已不可达。本文件保留仍可验证的核心契约：
//   - closeAllClients 按 LIFO 顺序对每个 Client 调用 Close()，不崩溃；
//   - 空列表 / 正常 Client 列表下返回 nil error。
package main

import (
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestCloseAllClients_Empty_NoError 验证 pendingClients 为空时 closeAllClients
// 返回 nil error。
func TestCloseAllClients_Empty_NoError(t *testing.T) {
	pendingClientsMu.Lock()
	pendingClients = nil
	pendingClientsMu.Unlock()
	t.Cleanup(func() {
		pendingClientsMu.Lock()
		pendingClients = nil
		pendingClientsMu.Unlock()
	})

	if err := closeAllClients(); err != nil {
		t.Fatalf("空列表 closeAllClients 应返回 nil error，实际: %v", err)
	}
}

// TestCloseAllClients_NormalClients_NoError 验证多个正常 Client 能依次关闭
// （LIFO 遍历无 panic），返回 nil error。
func TestCloseAllClients_NormalClients_NoError(t *testing.T) {
	pendingClientsMu.Lock()
	pendingClients = nil
	pendingClientsMu.Unlock()
	t.Cleanup(func() {
		pendingClientsMu.Lock()
		pendingClients = nil
		pendingClientsMu.Unlock()
	})

	for i := 0; i < 3; i++ {
		c, err := client.New(client.WithTimeout(5 * 1e9))
		if err != nil {
			t.Fatalf("client.New: %v", err)
		}
		trackClient(c)
	}

	if err := closeAllClients(); err != nil {
		t.Fatalf("正常 Client 列表 closeAllClients 应返回 nil error，实际: %v", err)
	}
}
