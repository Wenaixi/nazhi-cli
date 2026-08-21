package client

import (
	"context"
	"encoding/json"
	"errors"
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
//
// 空数据契约：服务端 code=1 但尚未提交评价（returnData/dataMap/dataList 全空，
// 或解码后归一化为 nil）时返回 (nil, nil)，与 QuerySelfEvaluationJSON 对齐。
// 不把「未提交」误判为 doBizGetDecode 的「所有解码器均失败」。
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
		// 业务成功但无评价内容：doBizGetDecode 在所有解码器均返回 (nil,nil) 时
		// 返回 ErrAllDecodersFailed。仅当无真实解码错误（纯空）时归一为 (nil,nil)，
		// 有解码错误则透传，便于排错。
		if isEmptyDecodeFailure(err) {
			return nil, nil
		}
		return nil, err
	}
	return v, nil
}

// isEmptyDecodeFailure 判断 err 是否为 doBizGetDecode 的纯空结果（无 lastErr）。
//
// 语义：
//
//	业务成功但未提交评价时，所有解码器返回 (nil,nil)，doBizGetDecode 返回
//	包含 ErrAllDecodersFailed 的 Join 错误且无真实解码子错误 → 本函数 true，
//	调用方归一为 (nil,nil) 空成功。
//
//	若解码器有真实错误（JSON 类型不匹配等），Join 会附带 lastErr → false，
//	保留错误给调用方排错。
//
// 实现：用 errors.Is 匹配哨兵，避免字符串匹配的脆弱性（旧实现曾用
// strings.Contains("所有解码器均失败")，任何包含该短语的业务错误都会被误判）。
func isEmptyDecodeFailure(err error) bool {
	if err == nil {
		return false
	}
	if !errors.Is(err, ErrAllDecodersFailed) {
		return false
	}
	// 区分纯空（无子错误）vs 真实解码失败（有子错误）。
	// errors.Join 的多错误用 Unwrap() []error 暴露。
	type multiUnwrapper interface{ Unwrap() []error }
	if m, ok := err.(multiUnwrapper); ok {
		for _, sub := range m.Unwrap() {
			if sub == nil {
				continue
			}
			if errors.Is(sub, ErrAllDecodersFailed) {
				continue
			}
			// 存在非哨兵子错误 → 真实解码失败，非空
			return false
		}
	}
	return true
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
	// 字段别名策略（已收窄，WARN-4）：
	//
	// - 主路径：types.SelfEvalStatus 的 JSON tag 为 studentComment/teacherComment，
	//   同时兼容平台真实返回的 snake_case（student_comment/teacher_comment），
	//   前端 mainLeft.vue / selfgaintloss.vue 均以 dataMap.student_comment 为准。
	// - 已验证别名：content / teacherRemark 为历史测试与旧平台字段，保留兼容。
	// - 已移除的过宽别名：selfEvaluation / evaluationContent 无前端或抓包依据，
	//   属过度猜测，已收窄移除；若未来服务端新增字段，应在 types 层显式处理，
	//   而非在此无限制扩张别名表。
	status := &types.SelfEvalStatus{
		ID:             firstInt64(m, "id", "platformId", "selfEvalId"),
		StudentComment: firstString(m, "studentComment", "student_comment", "content"),
		TeacherComment: firstString(m, "teacherComment", "teacher_comment", "teacherRemark"),
	}
	if status.ID == 0 && status.StudentComment == "" && status.TeacherComment == "" {
		return nil
	}
	return status
}

// ParseStudentComment 解析结构化自评的二次 JSON（WARN-1 显式 helper）。
//
// 对应前端 selfgaintloss.vue：querySelfEvaluation 返回的 dataMap.student_comment
// 本身是 JSON.stringify(form) 的字符串，需二次 JSON.parse 才能得到表单对象。
// 普通文本自评（mainLeft.vue）直接为字符串，无需二次解析。
//
// 入参 comment 为 QuerySelfEvaluation 返回的 SelfEvalStatus.StudentComment。
// 返回：
//   - 若 comment 为空或非 JSON 对象字符串，返回 (nil, nil) 或原错误；
//   - 若为结构化 JSON 字符串，返回解析后的 map。
//
// 调用方可通过返回值 nil 判断为普通文本自评。
func ParseStudentComment(comment string) (map[string]any, error) {
	trimmed := strings.TrimSpace(comment)
	if trimmed == "" {
		return nil, nil
	}
	// 结构化自评为 JSON 对象字符串；普通文本通常不以 "{" 开头
	if !strings.HasPrefix(trimmed, "{") {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(trimmed), &m); err != nil {
		return nil, fmt.Errorf("ParseStudentComment 解析失败: %w", err)
	}
	return m, nil
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
// CLI 通过 QuerySelfGradEvaluationJSON 透传原始 JSON（self-eval grad-status）。
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

// SubmitSelfGradEvaluation 提交毕业评价。
//
// 与 SubmitSelfEvaluation 对称，使用 {"studentComment": "<评语>"} 请求体。
func (c *Client) SubmitSelfGradEvaluation(ctx context.Context, token string, comment string) error {
	_, err := c.doBizAndDecode(ctx, token, "SubmitSelfGradEvaluation", "/api/studentMoralEduNew/addSelfGradEvaluation",
		http.MethodPost, map[string]string{"studentComment": comment})
	return err
}
