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
// 历史: refactor/review-tdd 从中发现 stringPtrOr → derefOr 重命名，避免
// 与 cmp.Or 混淆后引发 nil panic。现在泛型化后所有 *T 类型都可使用。
func DerefOr[T any](s *T, def T) T {
	if s == nil {
		return def
	}
	return *s
}
