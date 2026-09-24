//go:build race

package client

// raceEnabled 标记当前构建是否启用 -race（由 build 约束注入）。
//
// race 模式会显著改变分配行为（检测桩 + 额外堆分配），allocs/op 门禁在 race
// 下失去意义。raceEnabled 在此为 true，让门禁断言在 `go test -race ./...`
// 全仓步骤（CI check job 的 race 检测）下优雅跳过；门禁只在非 race 的独立步骤
// （make test-perf / CI 性能门禁步骤）执行。
const raceEnabled = true
