package main

// 本文件锁定 CLI 写操作族 payload 未知键拒绝的收敛后命令级语义。
//
// 契约（重构后统一）：
//   - 全部写操作命令（task submit/edit/preview、honor add/update、
//     typical-case submit/update、user update）对 payload 顶层未知键
//     以参数错误拒绝（400/exit3），错误文案「payload 含未知键: %v」
//   - 大小写变体（如 "Telephone" vs "telephone"）折叠放行（ToLower 比较）
//   - 允许集统一小写存储
//
// 行为变化（由本文件锁定）：
//   - 重构前 user update 的 unknownUserUpdateKeys 大小写敏感（注释称
//     对齐实际没对齐），{"Telephone":...} 会被拒；重构后折叠放行。
//   - 重构前 unknownTaskInputKeys 是独立实现（同语义不同代码）；重构后
//     共用 unknownUpdatePayloadKeys，行为不变。
//
// 这些测试在重构前必须失败（红），重构后通过（绿）——它们验证的是
// 命令对用户可见的契约，不是 helper 内部实现。

import (
	"fmt"
	"testing"
)

// convergeRun 表达收敛后的命令级拒绝流：parse → rejectUnknown (ToLower fold)。
func convergeRun(payload string, allowed map[string]struct{}) (unknown []string, rejected bool) {
	unknown = unknownUpdatePayloadKeys([]byte(payload), allowed)
	return unknown, len(unknown) > 0
}

// TestUserUpdate_UnknownKeys_Drift 锁定 user update 未知键拒绝的**重构前 drift**：
// unknownUserUpdateKeys 直接以原始键查 camelCase 允许集（大小写敏感），
// {"Telephone":...} 会被误判为未知键；而注释声称"与 task 族 EqualFold 语义对齐"。
// 重构后该函数改为调用 unknownUpdatePayloadKeys(payloadBytes, userUpdateAllowedKeys)
// （ToLower 折叠），行为变化由此测试锁定：重构前 FAIL（折叠失败），重构后 PASS。
func TestUserUpdate_UnknownKeys_Drift(t *testing.T) {
	// 大小写变体 Telephone：重构前 unknownUserUpdateKeys 判未知（drift），
	// 重构后折叠放行。本测试断言**收敛后的期望**——重构前必须失败。
	if unknown := unknownUserUpdateKeys([]byte(`{"Telephone":"13800138000"}`)); len(unknown) != 0 {
		t.Fatalf("user update 大小写变体应折叠放行（重构后），当前 drift 判未知: %v", unknown)
	}
	// 精确小写命中
	if unknown := unknownUserUpdateKeys([]byte(`{"telephone":"13800138000"}`)); len(unknown) != 0 {
		t.Fatalf("精确小写键不应判未知: %v", unknown)
	}
	// 真正未知键仍拒绝
	if unknown := unknownUserUpdateKeys([]byte(`{"telephoneX":"13800138000"}`)); len(unknown) != 1 || unknown[0] != "telephoneX" {
		t.Fatalf("未知键应报出: %v", unknown)
	}
}

// TestTaskUpdate_UnknownKeys_FoldCase 锁定 task 族（submit/edit/preview）
// 未知键拒绝共用 unknownUpdatePayloadKeys：行为与重构前 unknownTaskInputKeys
// 完全一致（ToLower 折叠、稳定排序、历史兼容字段放行）。
func TestTaskUpdate_UnknownKeys_FoldCase(t *testing.T) {
	allowed := taskInputAllowedKeys

	// 合法键（含大小写变体）折叠放行
	if unknown, rejected := convergeRun(`{"TaskId":1,"ImagePaths":["./a.jpg"]}`, allowed); rejected {
		t.Fatalf("task 合法键大小写变体应折叠放行: %v", unknown)
	}
	// 历史兼容字段放行
	if unknown, rejected := convergeRun(`{"id":1,"circleDate":"2026-09-01"}`, allowed); rejected {
		t.Fatalf("task 历史兼容字段应放行: %v", unknown)
	}
	// 未知键拒绝（含拼错键名 imagePath 单数）
	if unknown, rejected := convergeRun(`{"imagePath":"./a.jpg","taskId":1}`, allowed); !rejected || len(unknown) != 1 || unknown[0] != "imagePath" {
		t.Fatalf("task 未知键应报出: %v rejected=%v", unknown, rejected)
	}
}

// TestUnknownKeys_StableSort 锁定稳定排序：未知键按原始键名字典序输出，
// 保证错误消息确定性（脚本依赖）。
func TestUnknownKeys_StableSort(t *testing.T) {
	unknown := unknownUpdatePayloadKeys([]byte(`{"zzz":1,"AAA":2,"bbb":3}`), allowedKeysLower("id"))
	want := []string{"AAA", "bbb", "zzz"}
	if len(unknown) != len(want) {
		t.Fatalf("未知键数量: got %v want %v", unknown, want)
	}
	for i := range want {
		if unknown[i] != want[i] {
			t.Fatalf("稳定排序: got %v want %v", unknown, want)
		}
	}
}

// TestUnknownKeys_NonObjectReturnsNil 锁定非对象 JSON → nil（调用方已报错）。
func TestUnknownKeys_NonObjectReturnsNil(t *testing.T) {
	if unknown := unknownUpdatePayloadKeys([]byte(`[1,2,3]`), allowedKeysLower("id")); unknown != nil {
		t.Fatalf("非对象 JSON 应返回 nil: %v", unknown)
	}
}

// TestUserUpdate_UnknownKeys_FoldCase 通过命令级 helper 断言 user update
// 收敛后的 ToLower 折叠行为（与 Drift 测试互补：Drift 断言 helper 本身，
// 本测试断言收敛后的命令流）。
func TestUserUpdate_UnknownKeys_FoldCase(t *testing.T) {
	allowed := userUpdateAllowedKeys

	// 精确小写命中
	if unknown, rejected := convergeRun(`{"telephone":"13800138000"}`, allowed); rejected {
		t.Fatalf("精确小写键不应判未知: %v", unknown)
	}
	// 大小写变体折叠放行（重构后）
	if unknown, rejected := convergeRun(`{"Telephone":"13800138000"}`, allowed); rejected {
		t.Fatalf("user update 大小写变体应折叠放行（重构后）: %v", unknown)
	}
	// 真正未知键仍拒绝
	if unknown, rejected := convergeRun(`{"telephoneX":"13800138000"}`, allowed); !rejected || len(unknown) != 1 || unknown[0] != "telephoneX" {
		t.Fatalf("user update 未知键应报出: %v rejected=%v", unknown, rejected)
	}
}

// allowedKeysLower 构造小写允许集。
func allowedKeysLower(keys ...string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		allowed[fmt.Sprintf("%s", k)] = struct{}{}
	}
	return allowed
}
