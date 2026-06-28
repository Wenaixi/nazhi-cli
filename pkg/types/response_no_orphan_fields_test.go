// Package types 公共类型契约测试 — UnifiedResponse 孤儿字段删除守卫。
//
// pkg/types/response.go UnifiedResponse 历史带 7 个全仓 0 引用字段：
//   - DataString   *string          `json:"dataString"`
//   - PageBean     *json.RawMessage `json:"pageBean"`
//   - Note         *string          `json:"note"`
//   - InsertID     int64            `json:"insertID"`
//   - UpdateCount  int              `json:"updateCount"`
//   - IsAttendance int              `json:"isAttendance"`
//   - DataInt      int              `json:"dataInt"`
//
// 这些字段仅在 json.Unmarshal 时被动填充，序列化为零值/空对象，
// 增加结构体大小且对调用方零价值。修复后删除。
//
// 保留活跃字段：Code / Msg / ReturnData / DataList / DataMap
// （这些字段都有活跃的解码方法 DecodeReturnData/DecodeDataList/DecodeDataMap）。
//
// 验证策略：
//  1. JSON 序列化不再含 7 个孤儿字段键
//  2. 活跃字段仍然保留
package types

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestUnifiedResponse_NoOrphanFields 守护：序列化不再含 7 个孤儿字段。
//
// 7 个孤儿字段：dataString / pageBean / note / insertID / updateCount / isAttendance / dataInt
func TestUnifiedResponse_NoOrphanFields(t *testing.T) {
	resp := UnifiedResponse{
		Code: 1,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal 失败: %v", err)
	}

	// 7 个孤儿字段键都不应出现
	orphanKeys := []string{
		"dataString", "pageBean", "note",
		"insertID", "updateCount", "isAttendance", "dataInt",
	}
	for _, key := range orphanKeys {
		if strings.Contains(string(data), key) {
			t.Errorf("UnifiedResponse 序列化不应含 '%s' 孤儿字段键，实际: %s", key, data)
		}
	}

	// 反向断言：活跃字段仍保留
	if !strings.Contains(string(data), `"code":1`) {
		t.Errorf("UnifiedResponse 序列化应保留 code 字段，实际: %s", data)
	}
}

// TestUnifiedResponse_OrphanFieldsAreNotDeserializable 守护：
// 反序列化时不存在的孤儿字段键被忽略（兼容旧 API 响应体）。
func TestUnifiedResponse_OrphanFieldsAreNotDeserializable(t *testing.T) {
	// 模拟旧 API 返回的响应体（含孤儿字段）— 新结构应正常解析且不报错
	rawJSON := `{
		"code": 1,
		"msg": "ok",
		"returnData": null,
		"dataList": null,
		"dataMap": null,
		"dataString": "should-be-ignored",
		"pageBean": null,
		"note": "should-be-ignored",
		"insertID": 999,
		"updateCount": 5,
		"isAttendance": 1,
		"dataInt": 100
	}`

	var resp UnifiedResponse
	if err := json.Unmarshal([]byte(rawJSON), &resp); err != nil {
		t.Fatalf("UnifiedResponse 反序列化失败: %v", err)
	}

	if resp.Code != 1 {
		t.Errorf("Code 期望 1，实际 %d", resp.Code)
	}
}
