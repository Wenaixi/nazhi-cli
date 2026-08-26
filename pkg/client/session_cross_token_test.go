package client

import (
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestUpdateCachedUserInfo_CrossTokenIgnored 锁定 AUTH-2 契约：
// UpdateCachedUserInfo 注释承诺"token 不匹配时静默忽略"，但实现只按
// info != nil && LoadToken() != "" 判断——多 goroutine 场景下，用旧 token 的
// 迟到 info 会把新 token 的缓存顶掉（污染短暂可见，靠下一次失败自愈）。
// 修复后签名带 forToken 并比对：token 不一致时不得覆盖当前缓存。
func TestUpdateCachedUserInfo_CrossTokenIgnored(t *testing.T) {
	sm := &sessionManager{}

	// 前置：token-B 已激活成功，缓存是 token-B 的 info
	sm.token.Store("token-B")
	infoB := &types.UserInfo{Name: "李四", StudentNumber: "TEST2025002"}
	sm.mu.Lock()
	sm.cachedUserInfo = infoB
	sm.mu.Unlock()

	// 旧 token-A 的迟到 info 调 UpdateCachedUserInfo——AUTH-2 契约：必须被忽略
	infoA := &types.UserInfo{Name: "张三", StudentNumber: "TEST2025001"}
	sm.UpdateCachedUserInfo(infoA, "token-A")

	// 断言：缓存仍指向 token-B 的 info（未被 infoA 覆盖）
	sm.mu.Lock()
	got := sm.cachedUserInfo
	sm.mu.Unlock()
	if got != infoB {
		t.Errorf("跨 token 迟到写入污染缓存：期望指向 infoB(token-B)，实际 %p", got)
	}
}
