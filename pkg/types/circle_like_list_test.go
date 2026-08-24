package types

import (
	"encoding/json"
	"testing"
)

// TestCircleRecord_LikeListKey 锁定点赞列表键名：
// 真实 API 返回 camelCase likeList（HAR 27/27 条实证），tag 必须与之对齐，
// 否则字段恒为空、形同虚设。
func TestCircleRecord_LikeListKey(t *testing.T) {
	raw := `{"id":5690624,"likeStatus":true,"likeList":[{"id":1,"student_id":9}]}`
	var rec CircleRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if len(rec.LikeList) != 1 {
		t.Fatalf("likeList 键未解入 LikeList 字段（len=%d），检查 json tag", len(rec.LikeList))
	}
}
