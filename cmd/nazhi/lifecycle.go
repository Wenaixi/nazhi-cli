package main

import (
	"errors"
	"sync"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// pendingClients 跟踪本次进程内构造的所有 Client，main 退出前统一 Close()。
// 解决 "Client 包装了 *ocr.Pool 但不暴露 Close() → 临时目录泄漏" 的问题
// 。
var (
	pendingClientsMu  sync.Mutex
	pendingClients    []*client.Client
	pendingLogFilesMu sync.Mutex
	pendingLogFiles   []interface{ Close() error }
)

// trackClient 把 Client 加入待清理列表。
// 由 buildClient / buildBizClient 内部调用，业务侧无需感知。
func trackClient(c *client.Client) {
	pendingClientsMu.Lock()
	pendingClients = append(pendingClients, c)
	pendingClientsMu.Unlock()
}

func trackLogFile(f interface{ Close() error }) {
	pendingLogFilesMu.Lock()
	pendingLogFiles = append(pendingLogFiles, f)
	pendingLogFilesMu.Unlock()
}

func closeLogFiles() error {
	pendingLogFilesMu.Lock()
	files := pendingLogFiles
	pendingLogFiles = nil
	pendingLogFilesMu.Unlock()
	var firstErr error
	for i := len(files) - 1; i >= 0; i-- {
		if err := files[i].Close(); err != nil {
			firstErr = errors.Join(firstErr, err)
		}
	}
	return firstErr
}

// closeAllClients 关闭所有待清理 Client，返回聚合错误。
// 在 main 函数退出前调用一次 (defer)，保证 ONNX session + 临时目录 + keep-alive 连接全部释放。
func closeAllClients() error {
	pendingClientsMu.Lock()
	clients := pendingClients
	pendingClients = nil
	pendingClientsMu.Unlock()

	// 收集所有 Close 错误而非只保留第一个。
	var firstErr error
	for i := len(clients) - 1; i >= 0; i-- {
		if err := clients[i].Close(); err != nil {
			firstErr = errors.Join(firstErr, err)
		}
	}
	return firstErr
}
