package main

import (
	"errors"
	"io"
	"sync"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// 兼容层：旧测试直接读写 pendingClients/pendingLogFiles，需保留包级变量。
// 实现上委托给 defaultScope（assembly.go 定义），保持单一真相来源。
var (
	pendingClientsMu  sync.Mutex
	pendingClients    []*client.Client
	pendingLogFilesMu sync.Mutex
	pendingLogFiles   []io.Closer
)

func trackClient(c *client.Client) {
	defaultScope.TrackClient(c)
	// 同步到旧全局以兼容直接读 pendingClients 的测试
	pendingClientsMu.Lock()
	pendingClients = append(pendingClients, c)
	pendingClientsMu.Unlock()
}

func trackLogFile(f io.Closer) {
	defaultScope.TrackLogFile(f)
	pendingLogFilesMu.Lock()
	pendingLogFiles = append(pendingLogFiles, f)
	pendingLogFilesMu.Unlock()
}

func closeLogFiles() error {
	defaultScope.filesMu.Lock()
	scoped := defaultScope.files
	defaultScope.files = nil
	defaultScope.filesMu.Unlock()
	pendingLogFilesMu.Lock()
	legacys := pendingLogFiles
	pendingLogFiles = nil
	pendingLogFilesMu.Unlock()
	seen := make(map[io.Closer]struct{}, len(scoped)+len(legacys))
	ordered := make([]io.Closer, 0, len(scoped)+len(legacys))
	for i := len(scoped) - 1; i >= 0; i-- {
		if _, ok := seen[scoped[i]]; !ok {
			seen[scoped[i]] = struct{}{}
			ordered = append(ordered, scoped[i])
		}
	}
	for i := len(legacys) - 1; i >= 0; i-- {
		if _, ok := seen[legacys[i]]; !ok {
			seen[legacys[i]] = struct{}{}
			ordered = append(ordered, legacys[i])
		}
	}
	var firstErr error
	for _, f := range ordered {
		if err := f.Close(); err != nil {
			firstErr = errors.Join(firstErr, err)
		}
	}
	return firstErr
}

func closeAllClients() error {
	// 收集去重：defaultScope 与旧全局可能持有同一指针（trackClient 双写），去重后只关一次
	defaultScope.clientsMu.Lock()
	scoped := defaultScope.clients
	defaultScope.clients = nil
	defaultScope.clientsMu.Unlock()
	pendingClientsMu.Lock()
	legacys := pendingClients
	pendingClients = nil
	pendingClientsMu.Unlock()
	seen := make(map[*client.Client]struct{}, len(scoped)+len(legacys))
	ordered := make([]*client.Client, 0, len(scoped)+len(legacys))
	// 保持 LIFO：先 scoped 逆序，再 legacy 逆序，去重后仍 LIFO
	for i := len(scoped) - 1; i >= 0; i-- {
		if _, ok := seen[scoped[i]]; !ok {
			seen[scoped[i]] = struct{}{}
			ordered = append(ordered, scoped[i])
		}
	}
	for i := len(legacys) - 1; i >= 0; i-- {
		if _, ok := seen[legacys[i]]; !ok {
			seen[legacys[i]] = struct{}{}
			ordered = append(ordered, legacys[i])
		}
	}
	var firstErr error
	for _, c := range ordered {
		if err := c.Close(); err != nil {
			firstErr = errors.Join(firstErr, err)
		}
	}
	return firstErr
}
