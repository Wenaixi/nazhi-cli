//go:build !race

package client

// raceEnabled 标记当前构建是否启用 -race（由 build 约束注入）。
//
// race 模式会显著改变分配行为（race 检测桩 + 额外的堆分配），allocs/op
// 门禁在 race 下失去意义——门禁只在非 race 的独立步骤（make test-perf /
// CI 性能门禁步骤）中执行。该常量让门禁断言在 -race 下优雅跳过，避免标准
// `go test -race ./...` 全仓步骤（CI check job 的 race 检测）假阳性。
const raceEnabled = false
