package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// SubmitSelfEvaluation 提交自我评价文本。
func (c *Client) SubmitSelfEvaluation(ctx context.Context, token string, comment string) error {
	_, err := c.doBizAndDecode(ctx, token, "SubmitSelfEvaluation", "/api/studentMoralEduNew/addSelfEvaluation",
		http.MethodPost, map[string]string{"studentComment": comment})
	return err
}

// SubmitSelfEvaluationStructured 提交结构化自我评价（v1.4.0 新增）。
//
// 对应前端 selfgaintloss.vue 的"诉得失"页面：会做人/会求知/会生活/会创造/表现/优势/劣势 + 下学期目标。
// SDK 内部将 form 对象 JSON 序列化后，再包装为 {"studentComment": "<json>"} 提交。
//
// 典型字段：
//
//	bxqhzr — 本学期会做人目标
//	bxqhqz — 本学期会求知目标
//	bxqhsh — 本学期会生活目标
//	bxqhcz — 本学期会创造目标
//	bxqbx  — 本学期表现
//	bxqys  — 本学期优势
//	bxqls  — 本学期劣势
//	sxqhzr — 下学期会做人目标
//	sxqhqz — 下学期会求知目标
//	sxqhsh — 下学期会生活目标
//	sxqhcz — 下学期会创造目标
func (c *Client) SubmitSelfEvaluationStructured(ctx context.Context, token string, form map[string]any) error {
	// JSON 序列化 form 对象
	formJSON, err := json.Marshal(form)
	if err != nil {
		return fmt.Errorf("SubmitSelfEvaluationStructured 序列化失败: %w", err)
	}

	// 双层包装：{studentComment: JSON.stringify(form)}
	payload := map[string]string{
		"studentComment": string(formJSON),
	}

	_, err = c.doBizAndDecode(ctx, token, "SubmitSelfEvaluationStructured",
		"/api/studentMoralEduNew/addSelfEvaluation", http.MethodPost, payload)
	return err
}

// QuerySelfEvaluation 查询自我评价状态 + 教师评语。
//
// 使用 doBizGetDecode 的 fallback 链（returnData → dataMap → dataList[0]），
// 替换原有的 selfEvalGet + tryDecodeFallback 模式。
func (c *Client) QuerySelfEvaluation(ctx context.Context, token string) (*types.SelfEvalStatus, error) {
	v, err := doBizGetDecode[types.SelfEvalStatus](c, ctx, token, "QuerySelfEvaluation",
		"/api/studentMoralEduNew/querySelfEvaluation",
		decodeSelfEvalStatusFromContainer(func(resp types.UnifiedResponse) *json.RawMessage { return resp.ReturnData }),
		decodeSelfEvalStatusFromContainer(func(resp types.UnifiedResponse) *json.RawMessage { return resp.DataMap }),
		func(resp types.UnifiedResponse) (*types.SelfEvalStatus, error) {
			if resp.DataList == nil {
				return nil, nil
			}
			statuses, err := types.DecodeDataList[types.SelfEvalStatus](resp)
			if err == nil && len(statuses) > 0 {
				if normalized := normalizeSelfEvalStatus(map[string]any{
					"id":             statuses[0].ID,
					"studentComment": statuses[0].StudentComment,
					"teacherComment": statuses[0].TeacherComment,
				}); normalized != nil {
					return normalized, nil
				}
			}
			var rows []map[string]any
			if err := json.Unmarshal(*resp.DataList, &rows); err != nil {
				return nil, err
			}
			if len(rows) == 0 {
				return nil, nil
			}
			return normalizeSelfEvalStatus(rows[0]), nil
		},
	)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func decodeSelfEvalStatusFromContainer(getRaw func(types.UnifiedResponse) *json.RawMessage) func(types.UnifiedResponse) (*types.SelfEvalStatus, error) {
	return func(resp types.UnifiedResponse) (*types.SelfEvalStatus, error) {
		return decodeSelfEvalStatusMap(getRaw(resp))
	}
}

func decodeSelfEvalStatusMap(raw *json.RawMessage) (*types.SelfEvalStatus, error) {
	if raw == nil {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal(*raw, &m); err != nil {
		return nil, err
	}
	return normalizeSelfEvalStatus(m), nil
}

func normalizeSelfEvalStatus(m map[string]any) *types.SelfEvalStatus {
	if len(m) == 0 {
		return nil
	}
	status := &types.SelfEvalStatus{
		ID:             firstInt64(m, "id", "platformId", "selfEvalId"),
		StudentComment: firstString(m, "studentComment", "student_comment", "comment", "content", "selfEvaluation", "evaluationContent", "studentEvaluation"),
		TeacherComment: firstString(m, "teacherComment", "teacher_comment", "teacherRemark", "remark", "teacherAdvice"),
	}
	if status.ID == 0 && status.StudentComment == "" && status.TeacherComment == "" {
		return nil
	}
	return status
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := m[key]
		if !ok || value == nil {
			continue
		}
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return v
			}
		case []byte:
			s := strings.TrimSpace(string(v))
			if s != "" {
				return s
			}
		}
	}
	return ""
}

func firstInt64(m map[string]any, keys ...string) int64 {
	for _, key := range keys {
		value, ok := m[key]
		if !ok || value == nil {
			continue
		}
		switch v := value.(type) {
		case float64:
			return int64(v)
		case float32:
			return int64(v)
		case int:
			return int64(v)
		case int64:
			return v
		case int32:
			return int64(v)
		case json.Number:
			i, err := v.Int64()
			if err == nil {
				return i
			}
		case string:
			i, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			if err == nil {
				return i
			}
		}
	}
	return 0
}

// QuerySelfGradEvaluation 查询毕业状态。
//
// SDK 高级用户使用，CLI 暂未暴露此命令。
// 使用 doBizGetDecode 的 fallback 链（returnData → dataMap），
// 替换原有的 selfEvalGet + tryDecodeFallback 模式。
func (c *Client) QuerySelfGradEvaluation(ctx context.Context, token string) (*map[string]any, error) {
	v, err := doBizGetDecode[map[string]any](c, ctx, token, "QuerySelfGradEvaluation",
		"/api/studentMoralEduNew/querySelfGradEvaluation",
		types.DecodeReturnData[map[string]any],
		types.DecodeDataMap[map[string]any],
	)
	if err != nil {
		return nil, err
	}
	return v, nil
}
