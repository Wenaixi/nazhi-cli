package types

import (
	"encoding/json"
	"testing"
)

// TestCircleRecord_FrontendRealJSON 验证 getStudentCircle 真实响应字段名能正确解码。
//
// 平台字段命名混用：大量 snake_case（host_name/type_name）与若干 camelCase
// （imgList/commentList/likeStatus/ifMySelf/creationTimeStr/showName/imgPath/studentId）
// 并存。本测试用前端模板 + docs/sdk 样例中的真实键名，禁止测试夹具自创命名。
func TestCircleRecord_FrontendRealJSON(t *testing.T) {
	raw := `{
		"id": 5400001,
		"name": "主题班会",
		"content": "写实内容",
		"type": 1,
		"type_name": "主题班会",
		"host_name": "学校",
		"circle_task_name": "感恩教育",
		"circle_task_id": 18001,
		"hours": 0.5,
		"circle_date": "2026-07-12",
		"studentId": 380001,
		"operator_name": "赵明轩",
		"creationTimeStr": "2026-07-12 21:15",
		"showName": "展示标题",
		"ifMySelf": 1,
		"likeStatus": true,
			"status": 2,
			"auditRemark": "内容不符合要求",
		"imgList": [
			{
				"id": 5000001,
				"circle_id": 5400001,
				"class_id": 162647,
				"task_id": 18001,
				"attachment_id": 5000001,
				"imgPath": ".jpg"
			}
		],
		"imgPreViewList": [
			"http://www.nazhisoft.com/common/attachment/getImg?id=5000001"
		],
		"commentList": [
			{
				"id": 99,
				"content": "写得不错",
				"commentator": 1,
				"commentator_name": "老师",
				"commentator_type": 2,
				"comment_time": "2026-07-12 22:00"
			}
		]
	}`

	var rec CircleRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		t.Fatalf("Unmarshal CircleRecord 失败: %v", err)
	}

	if rec.ID != 5400001 {
		t.Errorf("ID: got %d", rec.ID)
	}
	if rec.TypeName != "主题班会" {
		t.Errorf("TypeName: got %q", rec.TypeName)
	}
	if rec.HostName != "学校" {
		t.Errorf("HostName: got %q", rec.HostName)
	}
	if rec.StudentId != 380001 {
		t.Errorf("StudentId: got %d，期望解码 studentId", rec.StudentId)
	}
	if rec.CreationTimeStr != "2026-07-12 21:15" {
		t.Errorf("CreationTimeStr: got %q，期望解码 creationTimeStr", rec.CreationTimeStr)
	}
	if rec.ShowName != "展示标题" {
		t.Errorf("ShowName: got %q，期望解码 showName", rec.ShowName)
	}
	if rec.IfMySelf != 1 {
		t.Errorf("IfMySelf: got %d，期望 1（前端 ifMySelf==1）", rec.IfMySelf)
	}
	if !rec.LikeStatus {
		t.Error("LikeStatus: 期望 true（解码 likeStatus）")
	}
	if rec.Status != 2 {
		t.Errorf("Status: got %d，期望 2（被撤回）", rec.Status)
	}
	if rec.AuditRemark != "内容不符合要求" {
		t.Errorf("AuditRemark: got %q，期望解码 auditRemark（camelCase）", rec.AuditRemark)
	}
	if len(rec.ImgList) != 1 {
		t.Fatalf("ImgList: got %d，期望 1（解码 imgList）", len(rec.ImgList))
	}
	if rec.ImgList[0].AttachmentID != 5000001 {
		t.Errorf("ImgList[0].AttachmentID: got %d", rec.ImgList[0].AttachmentID)
	}
	if rec.ImgList[0].ImgPath != ".jpg" {
		t.Errorf("ImgList[0].ImgPath: got %q，期望解码 imgPath", rec.ImgList[0].ImgPath)
	}
	if len(rec.ImgPreViewList) != 1 {
		t.Fatalf("ImgPreViewList: got %d，期望 1（解码 imgPreViewList）", len(rec.ImgPreViewList))
	}
	if len(rec.CommentList) != 1 {
		t.Fatalf("CommentList: got %d，期望 1（解码 commentList）", len(rec.CommentList))
	}
	if rec.CommentList[0].CommentatorName != "老师" {
		t.Errorf("CommentList[0].CommentatorName: got %q", rec.CommentList[0].CommentatorName)
	}
}

// TestCircleRecord_PlayRoleNumber 前端 managementRightBottom 列表用 switch(map.play_role) case 1/2/3，
// 证明 getStudentCircle 返回数字；结构化解码须得到 "1"/"2"/"3" 字符串（与提交表单 label 一致）。
func TestCircleRecord_PlayRoleNumber(t *testing.T) {
	raw := `{"id":1,"name":"班会","play_role":1,"content":"x"}`
	var rec CircleRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if string(rec.PlayRole) != "1" {
		t.Errorf("PlayRole: got %q，期望 \"1\"（平台数字 1 解码为角色码）", rec.PlayRole)
	}
}

// TestCircleRecord_PlayRoleString 提交路径/兼容样例可能仍为字符串 "2"。
func TestCircleRecord_PlayRoleString(t *testing.T) {
	raw := `{"id":1,"play_role":"2"}`
	var rec CircleRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if string(rec.PlayRole) != "2" {
		t.Errorf("PlayRole: got %q，期望 \"2\"", rec.PlayRole)
	}
}

// TestSelfEvalStatus_FrontendSnakeJSON 前端 mainLeft/selfgaintloss 查询读 data.student_comment。
func TestSelfEvalStatus_FrontendSnakeJSON(t *testing.T) {
	raw := `{
		"id": 372235,
		"student_comment": "HAR 里的学生自评",
		"teacher_comment": "HAR 里的教师评语"
	}`
	var st SelfEvalStatus
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		t.Fatalf("Unmarshal SelfEvalStatus: %v", err)
	}
	if st.ID != 372235 {
		t.Errorf("ID: got %d", st.ID)
	}
	if st.StudentComment != "HAR 里的学生自评" {
		t.Errorf("StudentComment: got %q，期望解码 student_comment", st.StudentComment)
	}
	if st.TeacherComment != "HAR 里的教师评语" {
		t.Errorf("TeacherComment: got %q，期望解码 teacher_comment", st.TeacherComment)
	}
}

// TestSelfEvalStatus_CamelJSON 兼容 camelCase 样例（部分 mock / returnData）。
func TestSelfEvalStatus_CamelJSON(t *testing.T) {
	raw := `{"id":1,"studentComment":"好","teacherComment":"继续"}`
	var st SelfEvalStatus
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if st.StudentComment != "好" || st.TeacherComment != "继续" {
		t.Errorf("got student=%q teacher=%q", st.StudentComment, st.TeacherComment)
	}
}

// TestHonorRecord_FrontendStatus 验证荣誉列表 status 整型字段（前端 scope.row.status）。
func TestHonorRecord_FrontendStatus(t *testing.T) {
	raw := `{
		"id": 1,
		"type_name": "校三好学生",
		"level_name": "校",
		"level": 5,
		"dimension_name": "思想品德",
		"status": 0,
		"statusName": "未审核",
		"get_date": "2026-06-30",
		"evaluation_agency": "示例中学",
		"score": 5,
		"ifshow": "是",
		"student_name": "张三",
		"class_name": "八班"
	}`
	var rec HonorRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		t.Fatalf("Unmarshal HonorRecord 失败: %v", err)
	}
	if rec.Status != 0 {
		t.Errorf("Status: got %d，期望 0", rec.Status)
	}
	if rec.ApprovedName != "未审核" {
		t.Errorf("ApprovedName: got %q", rec.ApprovedName)
	}
	if rec.Score != 5.0 {
		t.Errorf("Score: got %v", rec.Score)
	}
}

// TestHonorType_FrontendSnakeJSON 前端德育说明表列 dimension_name/level_name/score。
func TestHonorType_FrontendSnakeJSON(t *testing.T) {
	raw := `{
		"id": 10,
		"name": "校三好学生",
		"dimension_name": "思想品德",
		"level_name": "校",
		"level": 5,
		"score": "分数：+5.0"
	}`
	var ht HonorType
	if err := json.Unmarshal([]byte(raw), &ht); err != nil {
		t.Fatalf("Unmarshal HonorType 失败: %v", err)
	}
	if ht.DimensionName != "思想品德" {
		t.Errorf("DimensionName: got %q", ht.DimensionName)
	}
	if ht.LevelName != "校" {
		t.Errorf("LevelName: got %q", ht.LevelName)
	}
	if ht.Score != "分数：+5.0" {
		t.Errorf("Score: got %q", ht.Score)
	}
}
