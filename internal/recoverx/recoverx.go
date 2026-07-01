// Package recoverx 提供 panic recover 统一工具。
//
// 3 个 panic-recover 路径（cmd/main.go, pkg/client/task.go, pkg/client/client.go）
// 此前各自手写 os.Stderr.Write(debug.Stack()) + fmt.Errorf(...)。
// RecoverPanic 收敛为单点，确保 debug.Stack() 策略在所有 recover 路径一致。
package recoverx

import (
	"fmt"
	"os"
	"runtime/debug"
)

// RecoverPanic 处理 recover() 拿到的值 r：
//   - r == nil：返回 nil（不做任何事，让正常流程继续）
//   - r != nil：输出 goroutine stack trace 到 stderr，然后用 sentinel + name 包装为 error
//
// sentinel 为 nil 时跳过 %w 直接构造 message。
func RecoverPanic(r any, sentinel error, name string) error {
	if r == nil {
		return nil
	}
	// ponytail: 所有 recover 路径统一通过此函数输出 stack，确保唯一的输出策略入口
	os.Stderr.Write(debug.Stack())
	if sentinel != nil {
		return fmt.Errorf("%s: %w: %v", name, sentinel, r)
	}
	return fmt.Errorf("%s: %v", name, r)
}
