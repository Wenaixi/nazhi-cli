package main

import (
	"io"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// 本文件是进程级资源清理的包级入口，资源所有权由 ProcessScope 持有
// （见 assembly.go）。
//
// 历史沿革：此处原有一套 legacy 层，同时维护 defaultScope 与两张包级
// 表（pendingClients / pendingLogFiles），track 时双写、关闭时用 seen map
// 去重。ProcessScope 的 Close 方法因此长期零调用，被直接读写私有字段旁路。
// 2026-09-26 已删除双写层：去重改由 ProcessScope 自身承担
// （closeInLIFO，回归测试见 process_scope_close_test.go）。
//
// 现在这四个函数是 defaultScope 的薄转发，保留它们是因为：
//   - main 的退出链、30+ 处测试的 t.Cleanup 都按包级函数调用；
//   - package main 无外部兼容压力，保留转发比全量改调用点更小风险。
// 新代码应显式传递 ProcessScope。

func trackClient(c *client.Client) {
	defaultScope.TrackClient(c)
}

func trackLogFile(f io.Closer) {
	defaultScope.TrackLogFile(f)
}

func closeLogFiles() error {
	return defaultScope.CloseLogFiles()
}

func closeAllClients() error {
	return defaultScope.CloseAllClients()
}
