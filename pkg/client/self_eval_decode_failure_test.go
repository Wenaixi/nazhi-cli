package client

import (
	"errors"
	"fmt"
	"testing"
)

// TestIsEmptyDecodeFailure_Fragility_StringTrap 复现字符串误判为解码失败的脆弱性：
// 旧实现用 strings.Contains("所有解码器均失败") 判断，任何包含该短语的业务错误
// 都会被误判为“空成功”而被吞掉。修复后应基于哨兵 ErrAllDecodersFailed，
// 此类字符串陷阱必须返回 false。
func TestIsEmptyDecodeFailure_Fragility_StringTrap(t *testing.T) {
	fake := errors.New("业务提示：所有解码器均失败，请重试")
	if isEmptyDecodeFailure(fake) {
		t.Fatalf("脆弱性复现：包含短语但非哨兵的错误被误判为 true，fake=%v", fake)
	}
	// 包装一层也应 false
	wrapped := fmt.Errorf("wrap: %w", fake)
	if isEmptyDecodeFailure(wrapped) {
		t.Fatalf("包装后仍误判为 true，wrapped=%v", wrapped)
	}
	// Join 形式但无哨兵也应 false
	joined := errors.Join(fmt.Errorf("QuerySelfEvaluation: 所有解码器均失败"), errors.New("some decode error"))
	if isEmptyDecodeFailure(joined) {
		t.Fatalf("非哨兵 Join 被误判，joined=%v", joined)
	}
}

// TestIsEmptyDecodeFailure_SentinelPureEmpty 真正的纯空应为 true。
func TestIsEmptyDecodeFailure_SentinelPureEmpty(t *testing.T) {
	// doBizGetDecode 纯空路径：Join( fmt.Errorf("%s: %w", op, ErrAllDecodersFailed), nil)
	empty := errors.Join(fmt.Errorf("QuerySelfEvaluation: %w", ErrAllDecodersFailed), nil)
	if !isEmptyDecodeFailure(empty) {
		t.Fatalf("纯空哨兵应为 true，empty=%v", empty)
	}
	// 单哨兵包装也应 true
	single := fmt.Errorf("QuerySelfEvaluation: %w", ErrAllDecodersFailed)
	if !isEmptyDecodeFailure(single) {
		t.Fatalf("单哨兵应为 true，single=%v", single)
	}
}

// TestIsEmptyDecodeFailure_SentinelWithRealError 有真实解码子错误时应为 false，保留错误排错。
func TestIsEmptyDecodeFailure_SentinelWithRealError(t *testing.T) {
	withSub := errors.Join(fmt.Errorf("QuerySelfEvaluation: %w", ErrAllDecodersFailed), errors.New("json: cannot unmarshal"))
	if isEmptyDecodeFailure(withSub) {
		t.Fatalf("带真实子错误的哨兵应为 false，withSub=%v", withSub)
	}
}

// TestNormalizeSelfEvalStatus_AliasNarrowing 验证别名收窄：
//   - 前端仅读 student_comment / studentComment（mainLeft.vue:90、selfgaintloss.vue:107），
//     snake 为主路径、camel 兼容
//   - 投机键 content / teacherRemark 无任何前端读取点，与 selfEvaluation /
//     evaluationContent 同批收窄，不再生效
func TestNormalizeSelfEvalStatus_AliasNarrowing(t *testing.T) {
	// 投机键不再被消费：记录 ID 保留，注释字段为空
	m1 := map[string]any{"id": 1, "content": "投机键内容", "teacherRemark": "投机教师评语"}
	s1 := normalizeSelfEvalStatus(m1)
	if s1 == nil || s1.ID != 1 || s1.StudentComment != "" || s1.TeacherComment != "" {
		t.Fatalf("投机键 content/teacherRemark 应已收窄失效（注释为空），got %+v", s1)
	}
	// 已移除别名不再生效：仅用旧过宽 key 应归一为 nil
	m2 := map[string]any{"id": 0, "selfEvaluation": "过宽别名1", "evaluationContent": "过宽别名2"}
	s2 := normalizeSelfEvalStatus(m2)
	if s2 != nil {
		t.Fatalf("已收窄别名应不再生效，期望 nil，got %+v", s2)
	}
	// snake 主路径 + camel 兼容
	m3 := map[string]any{"id": 2, "student_comment": "snake 主路径", "teacher_comment": "教师 snake"}
	s3 := normalizeSelfEvalStatus(m3)
	if s3 == nil || s3.StudentComment != "snake 主路径" {
		t.Fatalf("snake 主路径应生效，got %+v", s3)
	}
}

// TestParseStudentComment 覆盖二次解析 helper。
func TestParseStudentComment(t *testing.T) {
	// 空串
	m, err := ParseStudentComment("")
	if err != nil || m != nil {
		t.Fatalf("空串应 (nil,nil)，got m=%v err=%v", m, err)
	}
	// 普通文本非 JSON
	m, err = ParseStudentComment("本学期表现良好")
	if err != nil || m != nil {
		t.Fatalf("普通文本应 (nil,nil)，got m=%v err=%v", m, err)
	}
	// 结构化 JSON
	m, err = ParseStudentComment(`{"bxqhzr":"会做人目标","bxqhqz":"会求知","bxqbx":"表现"}`)
	if err != nil {
		t.Fatalf("结构化 JSON 解析失败: %v", err)
	}
	if m["bxqhzr"] != "会做人目标" || m["bxqbx"] != "表现" {
		t.Fatalf("结构化解析结果异常，got %v", m)
	}
	// 非法 JSON（以 { 开头但非合法）
	_, err = ParseStudentComment("{not json")
	if err == nil {
		t.Fatal("非法 JSON 应返回错误")
	}
	// 带前导空格的 JSON
	m, err = ParseStudentComment("  {\"bxqhzr\":\"x\"}  ")
	if err != nil || m["bxqhzr"] != "x" {
		t.Fatalf("带空格 JSON 应解析成功，got m=%v err=%v", m, err)
	}
}
