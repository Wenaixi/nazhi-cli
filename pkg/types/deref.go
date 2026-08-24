package types

// DerefOr 安全解引用指针，指针为 nil 时返回兜底值 def。
//
// 与 cmp.Or(*s, def) 的区别：
//   - cmp.Or 在 s == nil 时 panic（解引用 nil 指针）
//   - DerefOr 安全返回 def
//
// 调用方:
//   - pkg/client/auth.go Login() 中 UnifiedResponse.Msg 兜底
//   - pkg/client/task.go 中任务状态和提交结果的 Msg 兜底
//
// 泛型化实现，所有 *T 类型通用；默认值防 nil 解引用 panic。
func DerefOr[T any](s *T, def T) T {
	if s == nil {
		return def
	}
	return *s
}
