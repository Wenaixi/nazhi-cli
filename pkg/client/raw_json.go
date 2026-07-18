// pkg/client 包内 1:1 透传业务 JSON 的方法族（*JSON 后缀）。
//
// 主人诉求：CLI 输出 envelope.data 必须跟 SDK 方法返回值 byte-for-byte 一致。
// 原方法（FetchTasks / GetMyInfo / GetSubmittedCircles / QuerySelfEvaluation /
// GetHonorTypes / GetHonorList / ActivateSession）反序列化进强类型 struct，
// 存在字段裁剪 / 命名转换 / 字段顺序稳定性的差异；本文件新增的方法直接返回
// 服务端原始 JSON（json.RawMessage），让调用方（CLI、第三方 SDK 消费者）
// 拿到与平台一致的数据。
//
// 设计要点：
//   - 自动分页/跨维度合并仍在 SDK 内部完成，调用方仍只需传 token
//   - 自动 fallback：dataList → returnData / dataMap（按方法实际通道）
//   - 不做 postProcess（如 user.go 的学校/班级名清理），由调用方负责
//   - 失败/取消语义与原方法一致，错误链不变

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// rawListBytes 返回 dataList 的原始字节。dataList 缺失时返回 nil。
// 返回 []byte 而非 RawMessage 让 bytes.Buffer 直接 append，避免反复拷贝。
func rawListBytes(resp types.UnifiedResponse) []byte {
	if resp.DataList == nil {
		return nil
	}
	return *resp.DataList
}

// rawObjectBytes 返回对象的原始字节（returnData 优先，dataMap 兜底）。
func rawObjectBytes(resp types.UnifiedResponse) []byte {
	switch {
	case resp.ReturnData != nil:
		return *resp.ReturnData
	case resp.DataMap != nil:
		return *resp.DataMap
	}
	return nil
}

// rawSingleObjectBytes 返回单个对象的原始 JSON。
// 用于 QuerySelfEvaluation 这种"returnData 优先，dataList[0] 兜底"的接口。
//
// 优先 returnData；若为字符串型 token/或为 null，尝试从 dataList 拿第一项；
// 否则尝试 dataMap（object 风格）。
//
// 返回非 nil 时一定是合法 JSON object；若都为 nil/字符串，返回 nil。
func rawSingleObjectBytes(resp types.UnifiedResponse) []byte {
	if resp.ReturnData != nil && len(*resp.ReturnData) > 0 && (*resp.ReturnData)[0] == '{' {
		return *resp.ReturnData
	}
	if resp.DataList != nil && len(*resp.DataList) > 0 {
		// dataList 解析为数组，取首项（数组非空判定）
		trimmed := bytes.TrimSpace(*resp.DataList)
		if len(trimmed) >= 2 && trimmed[0] == '[' && trimmed[len(trimmed)-1] == ']' {
			// 解析数组到 []json.RawMessage 然后取首项 —— 比正则简单
			var arr []json.RawMessage
			if err := json.Unmarshal(trimmed, &arr); err == nil && len(arr) > 0 {
				return arr[0]
			}
		}
	}
	if resp.DataMap != nil && len(*resp.DataMap) > 0 && (*resp.DataMap)[0] == '{' {
		return *resp.DataMap
	}
	return nil
}

// GetSubmittedCirclesJSON 获取当前用户已提交的全部写实记录，返回平台原始 JSON 数组。
//
// 自动翻页合并多页 dataList，与 GetSubmittedCircles 等价但保留平台原始字段。
//
// 返回值：
//   - json.RawMessage：dataList 风格的 JSON 数组（可能为 null 当服务端确实无记录）
//   - error：网络/解析/业务错误
//
// 取消语义：ctx 取消时返回 (已有合并数据, ctx.Err())，调用方按 partial envelope 处理。
func (c *Client) GetSubmittedCirclesJSON(ctx context.Context, token string) (json.RawMessage, error) {
	pageSize := c.submittedPageSize
	if pageSize <= 0 {
		pageSize = defaultSubmittedPageSize
	}

	page1, pb, raw1, err := c.fetchCirclePageJSON(ctx, token, 1, pageSize, 1)
	if err != nil {
		return nil, fmt.Errorf("GetSubmittedCirclesJSON 失败: %w", err)
	}

	// 单页：直接返回第一页原始字节，避免重新序列化
	if pb == nil || pb.TotalPage <= 1 || pb.TotalNum <= pageSize {
		if len(raw1) == 0 {
			return []byte("[]"), nil
		}
		return raw1, nil
	}

	// 多页：用 bytes.Buffer 拼接，"[" + (page1 去括号) + "," + (page2 去括号) + ... + "]"
	// 单页已经返回 page1 字节的原始外层方括号，多页拼接时需要先去掉避免嵌套。
	buf := bytes.NewBuffer(make([]byte, 0, len(page1)*pb.TotalPage))
	buf.WriteByte('[')
	buf.Write(trimArrayBrackets(raw1))
	for pageNo := 2; pageNo <= pb.TotalPage; pageNo++ {
		if cerr := ctx.Err(); cerr != nil {
			return json.RawMessage(trimArrayToCurrent(buf.Bytes())), cerr
		}
		_, _, raw, err := c.fetchSubmittedPageJSON(ctx, token, pageNo, pageSize)
		if err != nil {
			return json.RawMessage(trimArrayToCurrent(buf.Bytes())),
				fmt.Errorf("GetSubmittedCirclesJSON 第 %d 页失败: %w", pageNo, err)
		}
		if len(raw) == 0 {
			continue
		}
		// raw 是单页数组："[{...},{...}]"，去掉首尾方括号再逗号拼接
		buf.WriteByte(',')
		buf.Write(trimArrayBrackets(raw))
	}
	buf.WriteByte(']')
	return buf.Bytes(), nil
}

// GetSubmittedCirclesLimitJSON 按偏移和条数限制拉取已提交写实记录（原始 JSON）。
//
// offset=0, limit=0 时全量（等于 GetSubmittedCirclesJSON）。
// offset/limit 超出实际数据量时返回空数组，不报错。
//
// 返回值：
//   - dataList 原始 JSON 数组（可能为 []）
//   - 分页信息（含 TotalNum）
//   - error
func (c *Client) GetSubmittedCirclesLimitJSON(ctx context.Context, token string, offset, limit int) (json.RawMessage, *types.PageBean, error) {
	pageSize := c.submittedPageSize
	if pageSize <= 0 {
		pageSize = defaultSubmittedPageSize
	}

	if limit <= 0 {
		raw, err := c.GetSubmittedCirclesJSON(ctx, token)
		return raw, nil, err
	}

	_, pb, raw1, err := c.fetchCirclePageJSON(ctx, token, 1, pageSize, 1)
	if err != nil {
		return nil, nil, fmt.Errorf("GetSubmittedCirclesLimitJSON 失败: %w", err)
	}
	if pb == nil || pb.TotalNum == 0 {
		return []byte("[]"), pb, nil
	}
	if offset >= pb.TotalNum {
		return []byte("[]"), pb, nil
	}

	buf := bytes.NewBuffer(make([]byte, 0, 2048))
	buf.WriteByte('[')
	first := true
	taken := 0
	skipped := 0

	skipped, taken = appendPageRange(buf, raw1, &first, taken, offset, limit, skipped)
	if taken >= limit {
		buf.WriteByte(']')
		return buf.Bytes(), pb, nil
	}

	if pb.TotalPage > 1 {
		for pageNo := 2; pageNo <= pb.TotalPage; pageNo++ {
			if cerr := ctx.Err(); cerr != nil {
				return json.RawMessage(trimArrayToCurrent(buf.Bytes())), pb, cerr
			}
			_, _, raw, err := c.fetchCirclePageJSON(ctx, token, pageNo, pageSize, 1)
			if err != nil {
				return json.RawMessage(trimArrayToCurrent(buf.Bytes())), pb,
					fmt.Errorf("GetSubmittedCirclesLimitJSON 第 %d 页失败: %w", pageNo, err)
			}
			if len(raw) == 0 {
				continue
			}
			skipped, taken = appendPageRange(buf, raw, &first, taken, offset, limit, skipped)
			if taken >= limit {
				break
			}
		}
	}

	buf.WriteByte(']')
	return buf.Bytes(), pb, nil
}

// appendPageRange 从一页原始 JSON 数组中按 offset/limit 规则逐条取出写入 buf。
//
// pageRaw 是 "[{...},{...}]" 格式。用深度扫描分割 JSON 对象，不反序列化为 Go struct。
func appendPageRange(buf *bytes.Buffer, pageRaw []byte, first *bool, taken, offset, limit, skipped int) (int, int) {
	trimmed := trimArrayBrackets(pageRaw)
	if len(trimmed) == 0 {
		return skipped, taken
	}
	start := 0
	depth := 0
	for i := 0; i <= len(trimmed) && taken < limit; i++ {
		if i < len(trimmed) {
			switch trimmed[i] {
			case '{':
				depth++
			case '}':
				depth--
			case ',':
				if depth == 0 {
					obj := bytes.TrimSpace(trimmed[start:i])
					if len(obj) > 0 {
						skipped, taken = emitJSONObject(buf, first, taken, offset, limit, skipped, obj)
					}
					start = i + 1
				}
			}
		} else {
			obj := bytes.TrimSpace(trimmed[start:])
			if len(obj) > 0 {
				skipped, taken = emitJSONObject(buf, first, taken, offset, limit, skipped, obj)
			}
		}
	}
	return skipped, taken
}

// emitJSONObject 决定是否将一段 JSON 对象写入 buf。
func emitJSONObject(buf *bytes.Buffer, first *bool, taken, offset, limit, skipped int, obj []byte) (int, int) {
	if skipped < offset {
		return skipped + 1, taken
	}
	if *first {
		buf.Write(obj)
		*first = false
	} else {
		buf.WriteByte(',')
		buf.Write(obj)
	}
	return skipped + 1, taken + 1
}

// trimArrayBrackets 去掉 JSON 数组的首尾方括号（用于多页拼接）。
// 输入必须是合法的 JSON 数组；空数组返回空字节。
func trimArrayBrackets(raw []byte) []byte {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) >= 2 && trimmed[0] == '[' && trimmed[len(trimmed)-1] == ']' {
		return trimmed[1 : len(trimmed)-1]
	}
	return nil
}

// trimArrayToCurrent 关闭已写入的 JSON 数组（补齐末尾 ']'），用于 ctx 取消 / 翻页失败
// 时让已合并的部分仍是合法 JSON，便于调用方 partial envelope 渲染。
func trimArrayToCurrent(buf []byte) []byte {
	trimmed := bytes.TrimRight(buf, ",")
	if len(trimmed) > 0 && trimmed[len(trimmed)-1] != ']' {
		return append(trimmed, ']')
	}
	return trimmed
}

// GetTeacherCirclesJSON 获取教师代写的全部写实记录，返回平台原始 JSON 数组。
func (c *Client) GetTeacherCirclesJSON(ctx context.Context, token string) (json.RawMessage, error) {
	return c.getCirclesJSON(ctx, token, 2, "GetTeacherCirclesJSON")
}

// GetTeacherCirclesLimitJSON 按偏移和条数限制拉取教师写实记录（原始 JSON）。
func (c *Client) GetTeacherCirclesLimitJSON(ctx context.Context, token string, offset, limit int) (json.RawMessage, *types.PageBean, error) {
	return c.getCirclesLimitJSON(ctx, token, offset, limit, 2, "GetTeacherCirclesLimitJSON")
}

// GetWithdrawnCirclesJSON 获取被撤回的全部写实记录，返回平台原始 JSON 数组。
func (c *Client) GetWithdrawnCirclesJSON(ctx context.Context, token string) (json.RawMessage, error) {
	return c.getCirclesJSON(ctx, token, 3, "GetWithdrawnCirclesJSON")
}

// GetWithdrawnCirclesLimitJSON 按偏移和条数限制拉取被撤回写实记录（原始 JSON）。
func (c *Client) GetWithdrawnCirclesLimitJSON(ctx context.Context, token string, offset, limit int) (json.RawMessage, *types.PageBean, error) {
	return c.getCirclesLimitJSON(ctx, token, offset, limit, 3, "GetWithdrawnCirclesLimitJSON")
}

// GetPublicCirclesJSON 获取公示的全部写实记录（全班），返回平台原始 JSON 数组。
func (c *Client) GetPublicCirclesJSON(ctx context.Context, token string) (json.RawMessage, error) {
	return c.getCirclesJSON(ctx, token, 4, "GetPublicCirclesJSON")
}

// GetPublicCirclesLimitJSON 按偏移和条数限制拉取公示写实记录（原始 JSON）。
func (c *Client) GetPublicCirclesLimitJSON(ctx context.Context, token string, offset, limit int) (json.RawMessage, *types.PageBean, error) {
	return c.getCirclesLimitJSON(ctx, token, offset, limit, 4, "GetPublicCirclesLimitJSON")
}

// getCirclesJSON 是各类型写实记录全量拉取的通用实现。
func (c *Client) getCirclesJSON(ctx context.Context, token string, circleType int, methodName string) (json.RawMessage, error) {
	pageSize := c.submittedPageSize
	if pageSize <= 0 {
		pageSize = defaultSubmittedPageSize
	}

	page1, pb, raw1, err := c.fetchCirclePageJSON(ctx, token, 1, pageSize, circleType)
	if err != nil {
		return nil, fmt.Errorf("%s 失败: %w", methodName, err)
	}

	if pb == nil || pb.TotalPage <= 1 || pb.TotalNum <= pageSize {
		if len(raw1) == 0 {
			return []byte("[]"), nil
		}
		return raw1, nil
	}

	buf := bytes.NewBuffer(make([]byte, 0, len(page1)*pb.TotalPage))
	buf.WriteByte('[')
	buf.Write(trimArrayBrackets(raw1))
	for pageNo := 2; pageNo <= pb.TotalPage; pageNo++ {
		if cerr := ctx.Err(); cerr != nil {
			return json.RawMessage(trimArrayToCurrent(buf.Bytes())), cerr
		}
		_, _, raw, err := c.fetchCirclePageJSON(ctx, token, pageNo, pageSize, circleType)
		if err != nil {
			return json.RawMessage(trimArrayToCurrent(buf.Bytes())),
				fmt.Errorf("%s 第 %d 页失败: %w", methodName, pageNo, err)
		}
		if len(raw) == 0 {
			continue
		}
		buf.WriteByte(',')
		buf.Write(trimArrayBrackets(raw))
	}
	buf.WriteByte(']')
	return buf.Bytes(), nil
}

// getCirclesLimitJSON 是各类型写实记录按偏移/条数限制拉取的通用实现。
func (c *Client) getCirclesLimitJSON(ctx context.Context, token string, offset, limit int, circleType int, methodName string) (json.RawMessage, *types.PageBean, error) {
	pageSize := c.submittedPageSize
	if pageSize <= 0 {
		pageSize = defaultSubmittedPageSize
	}

	if limit <= 0 {
		raw, err := c.getCirclesJSON(ctx, token, circleType, methodName)
		return raw, nil, err
	}

	_, pb, raw1, err := c.fetchCirclePageJSON(ctx, token, 1, pageSize, circleType)
	if err != nil {
		return nil, nil, fmt.Errorf("%s 失败: %w", methodName, err)
	}
	if pb == nil || pb.TotalNum == 0 {
		return []byte("[]"), pb, nil
	}
	if offset >= pb.TotalNum {
		return []byte("[]"), pb, nil
	}

	buf := bytes.NewBuffer(make([]byte, 0, 2048))
	buf.WriteByte('[')
	first := true
	taken := 0
	skipped := 0

	skipped, taken = appendPageRange(buf, raw1, &first, taken, offset, limit, skipped)
	if taken >= limit {
		buf.WriteByte(']')
		return buf.Bytes(), pb, nil
	}

	if pb.TotalPage > 1 {
		for pageNo := 2; pageNo <= pb.TotalPage; pageNo++ {
			if cerr := ctx.Err(); cerr != nil {
				return json.RawMessage(trimArrayToCurrent(buf.Bytes())), pb, cerr
			}
			_, _, raw, err := c.fetchCirclePageJSON(ctx, token, pageNo, pageSize, circleType)
			if err != nil {
				return json.RawMessage(trimArrayToCurrent(buf.Bytes())), pb,
					fmt.Errorf("%s 第 %d 页失败: %w", methodName, pageNo, err)
			}
			if len(raw) == 0 {
				continue
			}
			skipped, taken = appendPageRange(buf, raw, &first, taken, offset, limit, skipped)
			if taken >= limit {
				break
			}
		}
	}

	buf.WriteByte(']')
	return buf.Bytes(), pb, nil
}

// FetchTasksJSON 拉取全维度任务列表，返回平台原始 JSON 数组（跨维度合并）。
//
// 自动跨维度并发拉取并在 SDK 内部合并为单一 JSON 数组，保留所有平台字段。
// 错误聚合：context 取消 → 返回已有原始字节 + ErrBusinessRejected；
// 业务错误 → 仍返回已合并的字节 + 包装错误（cmd 层按 partial envelope 处理）。
func (c *Client) FetchTasksJSON(ctx context.Context, token string) (json.RawMessage, error) {
	if _, err := c.ActivateSession(ctx, token); err != nil {
		return nil, fmt.Errorf("FetchTasksJSON 预热 session 失败: %w", err)
	}

	dimensions, err := c.fetchDimensions(ctx, token, "FetchTasksJSON getDimensions")
	if err != nil {
		return nil, err
	}

	headers := c.bizHeaders(token)

	activeDims := make([]types.Dimension, 0, len(dimensions))
	for _, dim := range dimensions {
		if dim.ID == 0 {
			continue
		}
		activeDims = append(activeDims, dim)
	}
	if len(activeDims) == 0 {
		return []byte("[]"), nil
	}

	type rawPage struct {
		raw []byte
	}
	pages := make(chan rawPage, len(activeDims))
	dimErrs := make([]error, 0, len(activeDims))
	var mu sync.Mutex

	// ponytail: 这里只补 root cause，不重写整套并发模型；对齐旧版跳过 id=0 的语义即可。
	for _, dim := range activeDims {
		dim := dim
		go func() {
			raw, err := c.fetchTasksDimensionJSON(ctx, dim, headers)
			if err != nil {
				appendLocked(&mu, &dimErrs, err)
				pages <- rawPage{}
				return
			}
			pages <- rawPage{raw: raw}
		}()
	}

	buf := bytes.NewBuffer(nil)
	buf.WriteByte('[')
	first := true
	totalPages := 0
	for range activeDims {
		p := <-pages
		if len(p.raw) == 0 {
			continue
		}
		// raw 是单维度 dataList 数组 "[]" 或 "[{...}]"
		if first {
			buf.Write(trimArrayBrackets(p.raw))
			first = false
		} else {
			trimmed := trimArrayBrackets(p.raw)
			if len(trimmed) > 0 {
				buf.WriteByte(',')
				buf.Write(trimmed)
			}
		}
		totalPages++
	}
	buf.WriteByte(']')

	if totalPages == 0 {
		if len(dimErrs) > 0 {
			return nil, fmt.Errorf("%w: FetchTasksJSON 全部维度失败: %w", ErrBusinessRejected, errors.Join(dimErrs...))
		}
		return []byte("[]"), nil
	}

	if len(dimErrs) > 0 {
		// 部分维度失败，仍返回已有字节 + partial 错误
		return buf.Bytes(), fmt.Errorf("%w: FetchTasksJSON %d 个维度失败: %w",
			ErrBusinessRejected, len(dimErrs), errors.Join(dimErrs...))
	}
	return buf.Bytes(), nil
}

// fetchTasksDimensionJSON 拉取单个维度的任务 dataList 原始字节。
func (c *Client) fetchTasksDimensionJSON(ctx context.Context, dim types.Dimension, headers map[string]string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	statURL := c.bizURL("/api/studentCircleNew/getCircleStatistics") + "?dimensionId=" + strconv.FormatInt(dim.ID, 10)
	bodyBytes, err := c.httpDo(ctx, http.MethodGet, statURL, nil, headers, "")
	if err != nil {
		return nil, fmt.Errorf("维度 %d(%s) 请求失败: %w", dim.ID, dim.Name, err)
	}
	resp, err := types.DecodeResponse(bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("维度 %d(%s) 响应解析失败: %w", dim.ID, dim.Name, err)
	}
	if err := types.CheckCode(resp); err != nil {
		return nil, fmt.Errorf("%w: 维度 %d(%s) 业务错误: %w", ErrBusinessRejected, dim.ID, dim.Name, err)
	}
	if resp.DataList == nil {
		return []byte("[]"), nil
	}
	return *resp.DataList, nil
}

// ActivateSessionJSON 激活业务 session，返回 /api/studentInfo/getMyInfo 的原始 JSON。
//
// 与 ActivateSession 等价但保留平台原始字段（如 ScoreRole / Photo 等未在 UserInfo 中的字段）。
// 同时附带 4 步 HAR 激活所需的 HTTP 请求（首页 → getMenu ×2 → getMyInfo），
// 即使下游不消费返回的 UserInfo 也必须执行完这 4 步。
//
// 返回值：
//   - json.RawMessage：getMyInfo 响应的 returnData / dataMap 原始字节；
//     当两者都为 null 时返回 nil。
//   - error：网络/解析/业务错误（含 ErrEmptyUserInfo / ErrSessionBackoff）
func (c *Client) ActivateSessionJSON(ctx context.Context, token string) (json.RawMessage, error) {
	info, err := c.sm.Activate(ctx, token, c.activateSessionLocked)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}
	return rawGetMyInfo(ctx, token, c)
}

// rawGetMyInfo 拉取 /api/studentInfo/getMyInfo 一次，返回原始对象字节。
// 与 user.go 的 getMyInfoRaw 语义相同，但跳过 struct 解码与 postProcess。
func rawGetMyInfo(ctx context.Context, token string, c *Client) (json.RawMessage, error) {
	if _, err := c.ActivateSession(ctx, token); err != nil {
		return nil, fmt.Errorf("ActivateSessionJSON 预热失败: %w", err)
	}
	headers := c.bizHeaders(token)
	headers["Referer"] = c.bizURL("/modify")

	bodyBytes, err := c.httpDo(ctx, http.MethodGet, c.bizURL("/api/studentInfo/getMyInfo"), nil, headers, "")
	if err != nil {
		return nil, fmt.Errorf("getMyInfo 请求失败: %w", err)
	}
	resp, err := types.DecodeResponse(bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("getMyInfo 响应解析失败: %w", err)
	}
	if err := types.CheckCode(resp); err != nil {
		return nil, fmt.Errorf("%w: 获取用户信息业务错误: %w", ErrBusinessRejected, err)
	}
	raw := rawObjectBytes(resp)
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: getMyInfo returnData 与 dataMap 都为空", ErrEmptyUserInfo)
	}
	return raw, nil
}

// GetMyInfoJSON 获取当前用户完整个人资料的原始 JSON。
//
// 等价 GetMyInfo 的语义（ActivateSession 4 步 HAR + getMyInfo 复用缓存），
// 但直接返回平台原始 JSON，保留所有未在 UserInfo 中建模的字段。
//
// 返回值：
//   - json.RawMessage：getMyInfo 响应的 returnData / dataMap 原始字节；
//     当两者都为 null 时返回 (nil, ErrEmptyUserInfo)。
//   - error：网络/解析/业务错误
func (c *Client) GetMyInfoJSON(ctx context.Context, token string) (json.RawMessage, error) {
	if _, err := c.ActivateSession(ctx, token); err != nil {
		return nil, fmt.Errorf("GetMyInfoJSON 预热 session 失败: %w", err)
	}
	headers := c.bizHeaders(token)
	headers["Referer"] = c.bizURL("/modify")
	bodyBytes, err := c.httpDo(ctx, http.MethodGet, c.bizURL("/api/studentInfo/getMyInfo"), nil, headers, "")
	if err != nil {
		return nil, fmt.Errorf("GetMyInfoJSON 请求失败: %w", err)
	}
	resp, err := types.DecodeResponse(bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("GetMyInfoJSON 响应解析失败: %w", err)
	}
	if err := types.CheckCode(resp); err != nil {
		return nil, fmt.Errorf("%w: 获取用户信息业务错误: %w", ErrBusinessRejected, err)
	}
	raw := rawObjectBytes(resp)
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: returnData 和 dataMap 都为空", ErrEmptyUserInfo)
	}
	return raw, nil
}

// QuerySelfEvaluationJSON 查询自我评价状态的原始 JSON。
//
// 等价 QuerySelfEvaluation 但保留平台原始字段。
// Fallback 链：returnData → dataMap → dataList[0]（与原方法一致）。
//
// 返回值：
//   - json.RawMessage：单条状态对象的原始 JSON；空数据时返回 (nil, nil)。
//   - error：网络/解析/业务错误
func (c *Client) QuerySelfEvaluationJSON(ctx context.Context, token string) (json.RawMessage, error) {
	resp, err := c.doBizAndDecode(ctx, token, "QuerySelfEvaluationJSON",
		"/api/studentMoralEduNew/querySelfEvaluation", http.MethodGet, nil)
	if err != nil {
		return nil, err
	}
	raw := rawSingleObjectBytes(*resp)
	if len(raw) == 0 {
		return nil, nil
	}
	return raw, nil
}

// GetHonorTypesJSON 获取所有荣誉类型的原始 JSON 数组。
//
// 等价 GetHonorTypes 但保留平台原始字段（如备注 / 启用状态 / 上传附件要求等）。
// 自动 fallback：dataList（首选）→ returnData（兼容）。
func (c *Client) GetHonorTypesJSON(ctx context.Context, token string) (json.RawMessage, error) {
	resp, err := c.doBizAndDecode(ctx, token, "GetHonorTypesJSON",
		"/api/studentMoralEduNew/getHonorType", http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetHonorTypesJSON 失败: %w", err)
	}
	raw := rawListBytes(*resp)
	if len(raw) == 0 && resp.ReturnData != nil {
		raw = *resp.ReturnData
	}
	if len(raw) == 0 {
		return []byte("[]"), nil
	}
	return raw, nil
}

// GetHonorListJSON 获取指定页荣誉记录的原始 JSON（不含自动翻页）。
//
// 与 GetHonorList 1:1 等价（按页调用，不自动翻页），
// 返回拼装后的完整 JSON 对象 `{"records":..., "page":...}`，
// records 和 page 字段值都是平台原始字节。
func (c *Client) GetHonorListJSON(ctx context.Context, token string, pageNo, pageSize int) (json.RawMessage, error) {
	path := "/api/studentMoralEduNew/getHonorByStudentId?pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(pageSize) + "&key="
	resp, err := c.doBizAndDecode(ctx, token, "GetHonorListJSON", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetHonorListJSON 失败: %w", err)
	}

	var recordsRaw json.RawMessage
	if resp.DataList != nil {
		recordsRaw = *resp.DataList
	}
	var pageBeanRaw json.RawMessage
	if resp.PageBean != nil {
		pageBeanRaw = *resp.PageBean
	}

	if len(recordsRaw) == 0 {
		recordsRaw = json.RawMessage("[]")
	}
	var buf bytes.Buffer
	buf.WriteString(`{"records":`)
	buf.Write(recordsRaw)
	if len(pageBeanRaw) > 0 {
		buf.WriteString(`,"page":`)
		buf.Write(pageBeanRaw)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
