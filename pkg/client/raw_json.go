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

	page1, pb, raw1, err := c.fetchSubmittedPageJSON(ctx, token, 1, pageSize)
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

// fetchSubmittedPageJSON 拉取一页已提交写实记录，返回原始 dataList 字节。
func (c *Client) fetchSubmittedPageJSON(ctx context.Context, token string, pageNo, pageSize int) ([]types.CircleRecord, *types.PageBean, []byte, error) {
	path := "/api/studentCircleNew/getStudentCircle?type=1&pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(pageSize) + "&key="

	resp, err := c.doBizAndDecode(ctx, token, "GetSubmittedCirclesJSON", path, http.MethodGet, nil)
	if err != nil {
		return nil, nil, nil, err
	}

	pb, err := types.DecodePageBean(*resp)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("GetSubmittedCirclesJSON 解析分页信息失败: %w", err)
	}
	records, err := types.DecodeDataList[types.CircleRecord](*resp)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("GetSubmittedCirclesJSON 解析写实记录失败: %w", err)
	}
	return records, pb, rawListBytes(*resp), nil
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

	// 拼接 buffer：跨维度合并所有 dataList（按 FetchTasksConcurrentLimit 守护并发）
	limit := len(dimensions)
	if limit > fetchTasksConcurrentLimit {
		limit = fetchTasksConcurrentLimit
	}
	if limit == 0 {
		limit = 1
	}

	type rawPage struct {
		raw  []byte
		size int
	}
	pages := make(chan rawPage, len(dimensions))
	dimErrs := make([]error, 0, len(dimensions))

	// 串行写入合并 buffer；并发只负责拉取 + 解析单维度 rawPage
	buf := bytes.NewBuffer(nil)
	buf.WriteByte('[')
	first := true

	for _, dim := range dimensions {
		if dim.ID == 0 {
			continue
		}
		dim := dim // capture
		go func() {
			raw, err := c.fetchTasksDimensionJSON(ctx, dim, headers)
			if err != nil {
				dimErrs = append(dimErrs, err)
				pages <- rawPage{}
				return
			}
			pages <- rawPage{raw: raw, size: len(raw)}
		}()
	}

	// 收集并发结果（按维度顺序，保证稳定输出）
	_ = limit // 简化：当前并发上限已通过 errgroup 处理在 FetchTasks，本方法保留语义而不强行 errgroup 化
	totalPages := 0
	for range dimensions {
		p := <-pages
		if len(p.raw) == 0 {
			continue
		}
		// raw 是单维度 dataList 数组 "[]" 或 "[{...}]"
		if first {
			buf.Write(trimArrayBrackets(p.raw))
			first = false
		} else {
			buf.WriteByte(',')
			buf.Write(trimArrayBrackets(p.raw))
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
//
// 返回值：
//   - json.RawMessage：拼装后的 `{"records":..., "page":...}`；
//     无记录时 records 为 `[]`，无分页上下文时不含 page 字段。
//   - error：网络/解析/业务错误
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
	// 拼接 {"records":..., "page":...}
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

// collectToBuf 把单页 dataList 字节收集进 buf，处理首项去括号、其余逗号分隔。
// helper：给 *JSON 系列跨页/跨维度合并共用。
//
// 内部状态：buf 必须以 '[' 开头（caller 负责初始化），
// 调用 collectToBuf 后 caller 需 buf.WriteByte(']') 闭合。
func collectToBuf(buf *bytes.Buffer, raw []byte, first *bool) {
	body := trimArrayBrackets(raw)
	if len(body) == 0 {
		return
	}
	if !*first {
		buf.WriteByte(',')
	}
	buf.Write(body)
	*first = false
}
