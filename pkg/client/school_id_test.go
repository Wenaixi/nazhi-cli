package client_test

import (
	"encoding/json"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// --- E1: schoolId JSON 三态分裂 RED 测试 ——

// TestSchoolID_UserInfo_ParsesSchoolId 验证 UserInfo.SchoolID 从 schoolId
// （驼峰）JSON key 正确解析。
// getMyInfo API 使用 schoolId（驼峰）作为 JSON key，
// 此测试确保标准 json.Unmarshal 能正确将 schoolId 反序列化到 int64 字段。
func TestSchoolID_UserInfo_ParsesSchoolId(t *testing.T) {
	data := `{"schoolId": 173, "name": "测试用户"}`
	var info types.UserInfo
	if err := json.Unmarshal([]byte(data), &info); err != nil {
		t.Fatalf("UserInfo JSON 解析失败: %v", err)
	}
	if info.SchoolID != 173 {
		t.Errorf("期望 SchoolID=173, 得到 %d", info.SchoolID)
	}
}

// TestSchoolID_SelfEvalStatus_ParsesSchoolId 验证 SelfEvalStatus.SchoolID
// 从 school_id（蛇形）JSON key 正确解析。
// querySelfEvaluation API 使用 school_id（蛇形）作为 JSON key，
// 与此对比 getMyInfo 的 UserInfo.SchoolID 使用 schoolId（驼峰）。
func TestSchoolID_SelfEvalStatus_ParsesSchoolId(t *testing.T) {
	data := `{"school_id": 173, "student_name": "测试用户"}`
	var status types.SelfEvalStatus
	if err := json.Unmarshal([]byte(data), &status); err != nil {
		t.Fatalf("SelfEvalStatus JSON 解析失败: %v", err)
	}
	if status.SchoolID != 173 {
		t.Errorf("期望 SchoolID=173, 得到 %d", status.SchoolID)
	}
}
