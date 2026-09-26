// pkg/client 包内 1:1 透传业务 JSON 的方法族（*JSON 后缀）。
//
// 设计目标：CLI 输出 envelope.data 与平台原始 JSON byte-for-byte 一致。
// 原方法（FetchTasks / GetMyInfo / GetSubmittedCircles / QuerySelfEvaluation /
// GetHonorTypes / GetHonorList / ActivateSession）反序列化进强类型 struct，
// 存在字段裁剪 / 命名转换 / 字段顺序稳定性的差异；本文件的方法直接返回
// 服务端原始 JSON（json.RawMessage），让调用方（CLI、第三方 SDK 消费者）
// 拿到与平台一致的数据。
//
// 设计要点：
//   - 自动分页/跨维度合并仍在 SDK 内部完成，调用方仍只需传 token
//   - 自动 fallback：dataList → returnData / dataMap（按方法实际通道）
//   - GetMyInfoJSON 内部调用 GetMyInfo()；ActivateSessionJSON 走 sessionManager
//     激活后 Marshal 其返回的 UserInfo。两者均含学校信息 SSO 降级补全与班级名清理后处理
//   - 失败/取消语义与原方法一致，错误链不变

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"golang.org/x/sync/errgroup"
)

// rawListBytes 返回 dataList 的原始字节。dataList 缺失时返回 nil。
// 返回 []byte 而非 RawMessage 让 bytes.Buffer 直接 append，避免反复拷贝。
// maxTotalPage 翻页上界钳制（C-F 修复）——TotalPage 来自服务端单字段声明，
// 恶意/异常值直接驱动 make 分配会导致单请求 OOM。10000 页 × pageSize=500 ≈ 500 万条，
// 远超任何真实业务数据量。
const maxTotalPage = 10000

// maxAssembleBuffer assembleCirclesJSON 预分配容量上界（CC1 修复）——
// len(raw1)×totalPage 可达 4MB×10000=40GB 单次 make，攻陷服务端可借首页大响应
// + 虚高 totalNum 驱动单请求 OOM。64MB 足够覆盖任何真实拼接输出。
const maxAssembleBuffer = 64 << 20

// maxFetchTasksDims 任务维度数上界（CLI-1 修复）——维度数直接来自
// getDimensions 服务端声明，恶意值驱动全维度并发拉取 × 单页 4MB（maxResponseBodySize）
// 累积无预算。128 维远超任何真实学校维度集（通常 <10）。FetchTasksJSON 与
// FetchTasks 两条取数路径共用同一道钳制（后者在 task.go 截断）。
const maxFetchTasksDims = 128

// capAssembledSlice 对已累积的 rawResult 切片做总量预算截断（N-04）。
// getCirclesJSON/getCirclesLimitJSON 翻页时把每页原始字节累积进 results，
// 已由 estimatePagesBudgeted（页数×首页字节）预翻页预算 + 本函数合并前
// 复核截断双层覆盖——服务端报 10000 页×4MB≈40GB 渐进填充时在真实翻页
// 前即被预预算截断，残余超预算由本函数兜底。超出预算返回 true，
// 调用方截断到已合并合法前缀。
func capAssembledSlice(raw1 []byte, results []rawResult, untilPage int) bool {
	if untilPage < 1 {
		return false
	}
	return cumulativeSliceBytes(raw1, results, untilPage) > maxAssembleBuffer
}

// cumulativeSliceBytes 统计 raw1 到 untilPage 的原始字节累积总量。
func cumulativeSliceBytes(raw1 []byte, results []rawResult, untilPage int) int {
	total := len(raw1)
	for pn := 2; pn <= untilPage && pn < len(results); pn++ {
		total += len(results[pn].raw)
	}
	return total
}

// estimatePagesBudgeted 以「页数 × 首页字节」估算多页累积量上界（N-04）。
// 翻页前 results 里 pn≥2 尚未拉取，无法按实际字节统计；真实每页 ≤ 首页
// 字节（分页语义），故 endPage×len(raw1) 是安全上界。
func estimatePagesBudgeted(pageCount, firstPageLen int) int {
	if pageCount < 1 || firstPageLen < 1 {
		return 0
	}
	return pageCount * firstPageLen
}

// isNullJSON 判断一段原始 JSON 是否为 null 形态：字面 null 或字符串 "null"。
// 平台偶发把空列表序列化为 dataList:"null"（字符串），字面 null 在解码层已折叠为
// nil 指针，这里统一识别两种形态，归一为 nil 让调用方走空值契约（[] 或 fallback）。
func isNullJSON(raw []byte) bool {
	trimmed := bytes.TrimSpace(raw)
	return bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte(`"null"`))
}

func rawListBytes(resp types.UnifiedResponse) []byte {
	if resp.DataList == nil {
		return nil
	}
	raw := bytes.TrimSpace(*resp.DataList)
	// null 形态（字面/字符串）视为空列表，不当作非法 JSON 或原样透传
	if isNullJSON(raw) {
		return nil
	}
	if !json.Valid(raw) {
		return nil
	}
	return raw
}

// rawSingleObjectBytes 返回单个对象的原始 JSON。
// 用于 QuerySelfEvaluation 这种"returnData 优先，dataList[0] 兜底"的接口。
//
// 优先 returnData；若为字符串型 token 或为 null，尝试从 dataList 拿第一项；
// 否则尝试 dataMap（object 风格）。
//
// 返回非 nil 时一定是合法 JSON object；若都为 nil/字符串，返回 nil。
func rawSingleObjectBytes(resp types.UnifiedResponse) []byte {
	if resp.ReturnData != nil && len(*resp.ReturnData) > 0 && (*resp.ReturnData)[0] == '{' {
		return *resp.ReturnData
	}
	if resp.DataList != nil && len(*resp.DataList) > 0 {
		trimmed := bytes.TrimSpace(*resp.DataList)
		if len(trimmed) >= 2 && trimmed[0] == '[' && trimmed[len(trimmed)-1] == ']' {
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

// type→type 映射说明（getStudentCircle）：
//   1 = 公示/全部     → GetPublicCircles
//   2 = 教师写实     → GetTeacherCircles
//   3 = 我发布的     → GetSubmittedCircles
//   4 = 被撤回       → GetWithdrawnCircles

// GetSubmittedCirclesJSON 获取当前用户自己发布的写实记录，返回平台原始 JSON 数组。
//
// type=3 只返回当前用户自己发布的内容，自动翻页合并多页 dataList。
// key 为搜索关键字（可空，对应 getStudentCircle 的 key 查询参数）。
//
// 返回值：
//   - json.RawMessage：dataList 风格的 JSON 数组——恒为合法数组（服务端无记录或 dataList:null 时归一为 []），
//     不存在成功路径返回 null 的情形；错误场景返回 (nil, err)
//   - error：网络/解析/业务错误
//
// 取消语义：ctx 取消时返回 (已有合并数据, ctx.Err())，调用方按 partial envelope 处理。
func (c *Client) GetSubmittedCirclesJSON(ctx context.Context, token string, key string) (json.RawMessage, error) {
	raw, _, err := c.getCirclesJSON(ctx, token, 3, key, "GetSubmittedCirclesJSON")
	return raw, err
}

// GetSubmittedCirclesLimitJSON 按偏移和条数限制拉取当前用户自己发布的写实记录（原始 JSON）。
//
// offset=0, limit=0 时全量（等于 GetSubmittedCirclesJSON）。
// offset/limit 超出实际数据量时返回空数组，不报错。
// key 为搜索关键字（可空）。
//
// 返回值：
//   - dataList 原始 JSON 数组（可能为 []）
//   - 分页信息（含 TotalNum）
//   - error
func (c *Client) GetSubmittedCirclesLimitJSON(ctx context.Context, token string, offset, limit int, key string) (json.RawMessage, *types.PageBean, error) {
	return c.getCirclesLimitJSON(ctx, token, offset, limit, 3, key, "GetSubmittedCirclesLimitJSON")
}

// GetTeacherCirclesJSON 获取教师代写的全部写实记录，返回平台原始 JSON 数组。
// key 为搜索关键字（可空）。
func (c *Client) GetTeacherCirclesJSON(ctx context.Context, token string, key string) (json.RawMessage, error) {
	raw, _, err := c.getCirclesJSON(ctx, token, 2, key, "GetTeacherCirclesJSON")
	return raw, err
}

// GetTeacherCirclesLimitJSON 按偏移和条数限制拉取教师写实记录（原始 JSON）。
// key 为搜索关键字（可空）。
func (c *Client) GetTeacherCirclesLimitJSON(ctx context.Context, token string, offset, limit int, key string) (json.RawMessage, *types.PageBean, error) {
	return c.getCirclesLimitJSON(ctx, token, offset, limit, 2, key, "GetTeacherCirclesLimitJSON")
}

// GetWithdrawnCirclesJSON 获取被撤回的全部写实记录，返回平台原始 JSON 数组。
// key 为搜索关键字（可空）。
func (c *Client) GetWithdrawnCirclesJSON(ctx context.Context, token string, key string) (json.RawMessage, error) {
	raw, _, err := c.getCirclesJSON(ctx, token, 4, key, "GetWithdrawnCirclesJSON")
	return raw, err
}

// GetWithdrawnCirclesLimitJSON 按偏移和条数限制拉取被撤回写实记录（原始 JSON）。
// key 为搜索关键字（可空）。
func (c *Client) GetWithdrawnCirclesLimitJSON(ctx context.Context, token string, offset, limit int, key string) (json.RawMessage, *types.PageBean, error) {
	return c.getCirclesLimitJSON(ctx, token, offset, limit, 4, key, "GetWithdrawnCirclesLimitJSON")
}

// GetPublicCirclesJSON 获取公示的全部写实记录（全班），返回平台原始 JSON 数组。
// key 为搜索关键字（可空）。
func (c *Client) GetPublicCirclesJSON(ctx context.Context, token string, key string) (json.RawMessage, error) {
	raw, _, err := c.getCirclesJSON(ctx, token, 1, key, "GetPublicCirclesJSON")
	return raw, err
}

// GetPublicCirclesLimitJSON 按偏移和条数限制拉取公示写实记录（原始 JSON）。
// key 为搜索关键字（可空）。
func (c *Client) GetPublicCirclesLimitJSON(ctx context.Context, token string, offset, limit int, key string) (json.RawMessage, *types.PageBean, error) {
	return c.getCirclesLimitJSON(ctx, token, offset, limit, 1, key, "GetPublicCirclesLimitJSON")
}

// rawResult 存储单页原始 JSON 数据。
type rawResult struct {
	raw []byte
}

// assembleCirclesJSON 将多页数据按页号顺序拼接为单个 JSON 数组。
//
// raw1 是第一页原始 JSON，results 是按页号索引的后续页数据。
// 成功路径返回完整 JSON 数组，失败路径通过 trimArrayToCurrent 截断为已有部分。
//
// 用 first 标志控制逗号，避免 page1 为空数组时产生 leading comma 非法 JSON（[,{...}]）。
// 对齐 assembleCirclesLimitJSON 的拼接策略。
func assembleCirclesJSON(raw1 []byte, results []rawResult, totalPage int, partialErr error) (json.RawMessage, error) {
	// C-F 修复：totalPage 参与预分配乘法，超钳制值会放大分配或整数溢出。
	if totalPage > maxTotalPage {
		totalPage = maxTotalPage
	}
	// CC1 修复：预分配容量钳制到固定上界——len(raw1)×totalPage 可达 40GB，
	// 攻陷服务端可借首页大响应+虚高 totalNum 驱动单请求 OOM。
	// P2-3：改为按已有页实际内容求和精确预分配。
	// 此前估算 capHint = 页数×首页字节，偏小则触发 bytes.Buffer 倍增扩容的
	// 多次整块拷贝。各页实际长度已知（results），求和即精确容量；上界仍由
	// maxAssembleBuffer 钳制（防攻陷服务端借大响应×虚高页数放大分配）。
	total := len(raw1) + 2
	for pn := 2; pn <= totalPage; pn++ {
		total += len(results[pn].raw) + 1
	}
	if total > maxAssembleBuffer {
		total = maxAssembleBuffer
	}
	buf := bytes.NewBuffer(make([]byte, 0, total))
	buf.WriteByte('[')
	first := true
	// page1 可能为空数组 "[]"，trim 后长度为 0，不得写逗号
	if trimmed := trimArrayBrackets(raw1); len(trimmed) > 0 {
		buf.Write(trimmed)
		first = false
	}
	for pn := 2; pn <= totalPage; pn++ {
		if len(results[pn].raw) == 0 {
			continue
		}
		trimmed := trimArrayBrackets(results[pn].raw)
		if len(trimmed) == 0 {
			continue
		}
		if first {
			buf.Write(trimmed)
			first = false
		} else {
			buf.WriteByte(',')
			buf.Write(trimmed)
		}
	}
	buf.WriteByte(']')
	if partialErr != nil {
		return json.RawMessage(trimArrayToCurrent(buf.Bytes())), partialErr
	}
	return buf.Bytes(), nil
}

// getCirclesJSON 是各类型写实记录全量拉取的通用实现。
//
// 多页时使用 errgroup 并发翻页（与 fetchAllCirclePages 对齐），
// 避免串行循环在数据量大时慢 2-5 倍。
// key 透传到 getStudentCircle 的 key 查询参数。
func (c *Client) getCirclesJSON(ctx context.Context, token string, circleType int, key string, methodName string) (json.RawMessage, *types.PageBean, error) {
	pageSize := c.effectivePageSize()

	pb, raw1, err := c.fetchCirclePageJSON(ctx, token, 1, pageSize, circleType, key)
	if err != nil {
		return nil, nil, fmt.Errorf("%s 失败: %w", methodName, err)
	}

	if pb == nil || pb.TotalNum <= pageSize {
		if len(raw1) == 0 {
			return []byte("[]"), pb, nil
		}
		return raw1, pb, nil
	}

	// 多页：预分配索引切片 + errgroup 并发翻页，保持页号顺序
	// C-F 修复：TotalPage 来自服务端单字段声明，恶意/异常值直接驱动 make 分配——
	// 服务端被攻陷时单请求 OOM 崩进程。超钳制值时直接截断（只返回首页），
	// 不翻页——钳制的意义是防放大而不是真翻 10000 页。
	// 页数下界与上界钳制统一由 derivePageBounds 负责（v1.6.4 / 84982af）：
	// 下界取 max(totalPage, ceil(totalNum/pageSize)) 防 totalPage 虚低漏页，
	// 上界钳到 maxTotalPage 防服务端声明值驱动 make 分配 OOM。
	declaredPages := derivePageBounds(pb.TotalNum, pb.TotalPage, pageSize)
	// CLI-124-01：全量路径在 make 前补「页数 × 首页字节」预算守卫，与
	// getCirclesLimitJSON 的 N-04 预估守卫同纪律。此前翻页后才由
	// capAssembledSlice 复核截断——最坏 10000 页 × 4MB 逐页填充至 40GB，
	// errgroup 已真实发出所有请求才在 g.Wait 后截断（内存放大 + 无谓翻页）。
	// 越界直接截断到首页快照，不翻页（防放大优先于全量完整性）。
	if declaredPages > 1 && estimatePagesBudgeted(declaredPages, len(raw1)) > maxAssembleBuffer {
		slog.Warn("raw_json: 全量翻页预估累积量超过合并预算，截断到首页",
			"estimated_bytes", estimatePagesBudgeted(declaredPages, len(raw1)), "max", maxAssembleBuffer)
		return raw1, pb, nil
	}
	results := make([]rawResult, declaredPages+1)
	results[1] = rawResult{raw: raw1}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrentPageLimit)

	for pageNo := 2; pageNo <= declaredPages; pageNo++ {
		pn := pageNo
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			_, raw, err := c.fetchCirclePageJSON(gctx, token, pn, pageSize, circleType, key)
			if err != nil {
				return fmt.Errorf("第 %d 页失败: %w", pn, err)
			}
			results[pn] = rawResult{raw: raw}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		// 部分失败时，已成功的页仍有效；按已有页顺序拼接
		raw, assembleErr := assembleCirclesJSON(raw1, results, declaredPages,
			fmt.Errorf("%s 部分页失败: %w", methodName, err))
		return raw, pb, assembleErr
	}

	// N-04：页面累积原始字节越过 maxAssembleBuffer 预算时截断到已合并
	// 前缀（对齐 submitted 条数闸的语义；这里按字节而非条数判）。
	// Cycle 105 修正：命中预算后必须传已钳制页数（而非声明页数）给
	// assembleCirclesJSON——此前传 declaredPages 让 Bytes.Buffer 仍无上限
	// 增长再做第二次全量拷贝，与「截断防放大」目标矛盾。
	if capAssembledSlice(raw1, results, declaredPages) {
		// 二分找「累积不超过预算」的最大页号：页量单调不降，从头线性也可
		// 接受但 O(n) 在 10000 页时无谓；线性页数已由 maxTotalPage 钳制。
		budgetPage := declaredPages
		for budgetPage > 1 && cumulativeSliceBytes(raw1, results, budgetPage) > maxAssembleBuffer {
			budgetPage--
		}
		slog.Warn("raw_json: 多页累积量超过合并预算，截断到已合并前缀",
			"total_bytes", cumulativeSliceBytes(raw1, results, declaredPages),
			"budget_page", budgetPage, "max", maxAssembleBuffer)
		raw, assembleErr := assembleCirclesJSON(raw1, results, budgetPage, nil)
		return raw, pb, assembleErr
	}

	raw, assembleErr := assembleCirclesJSON(raw1, results, declaredPages, nil)
	return raw, pb, assembleErr
}

// getCirclesLimitJSON 是各类型写实记录按偏移/条数限制拉取的通用实现。
//
// 多页时使用 errgroup 并发翻页，但只请求 offset/limit 覆盖到的页：
// endPage = min(TotalPage, ceil((offset+limit)/pageSize))，
// 避免全量翻页再截断造成的多余请求。
// key 透传到 getStudentCircle 的 key 查询参数。
func (c *Client) getCirclesLimitJSON(ctx context.Context, token string, offset, limit int, circleType int, key string, methodName string) (json.RawMessage, *types.PageBean, error) {
	pageSize := c.effectivePageSize()

	if limit <= 0 {
		raw, pb, err := c.getCirclesJSON(ctx, token, circleType, key, methodName)
		return raw, pb, err
	}

	pb, raw1, err := c.fetchCirclePageJSON(ctx, token, 1, pageSize, circleType, key)
	if err != nil {
		return nil, nil, fmt.Errorf("%s 失败: %w", methodName, err)
	}
	if pb == nil || pb.TotalNum == 0 {
		return []byte("[]"), pb, nil
	}
	if offset >= pb.TotalNum {
		return []byte("[]"), pb, nil
	}

	// 只拉到覆盖 offset+limit 的最后一页，不全量翻页。
	//
	// 注意：limitEndPage 的收敛必须用**未钳制**的声明页数。若传
	// derivePageBounds 的结果（已钳到 maxTotalPage），totalNum=1e9 场景会
	// 算出 endPage=10000 并真的翻 10000 页，违反 C86-CLI#2 锁定的
	// 「超限退回首页、不再翻页」。
	declaredPages := derivePageBoundsUnclamped(pb.TotalNum, pb.TotalPage, pageSize)
	endPage := limitEndPage(offset, limit, pageSize, declaredPages)

	// N-04（Cycle 105 修正）：getCirclesLimitJSON 的预翻页估算必须以
	// 「页数 × 首页字节」作上界——此前 capAssembledSlice 看 results 里
	// 已拉取字节（pn≥2 尚未拉全为 nil），concat 只有 len(raw1)≤4MB，
	// >64MB 预算永不命中，估算守卫形同虚设。用 endPage×len(raw1) 作为
	// 单页最坏上界（真实页面 ≤ 首页），越界直接截断到首页。
	results := make([]rawResult, endPage+1)
	results[1] = rawResult{raw: raw1}
	if endPage > 1 && estimatePagesBudgeted(endPage, len(raw1)) > maxAssembleBuffer {
		slog.Warn("raw_json: limit 翻页预估累积量超过合并预算，截断到首页",
			"estimated_bytes", estimatePagesBudgeted(endPage, len(raw1)), "max", maxAssembleBuffer)
		endPage = 1
	}

	if endPage > 1 {
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(concurrentPageLimit)

		for pageNo := 2; pageNo <= endPage; pageNo++ {
			pn := pageNo
			g.Go(func() error {
				if err := gctx.Err(); err != nil {
					return err
				}
				_, raw, err := c.fetchCirclePageJSON(gctx, token, pn, pageSize, circleType, key)
				if err != nil {
					return fmt.Errorf("第 %d 页失败: %w", pn, err)
				}
				results[pn] = rawResult{raw: raw}
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			// 部分失败时，已成功的页仍有效；按已有页顺序拼接
			return c.assembleCirclesLimitJSON(results, pb, offset, limit, endPage, methodName,
				fmt.Errorf("%s 部分页失败: %w", methodName, err))
		}

		// CLI-120-1：翻页完成后做合并前累积字节复核——estimatePagesBudgeted
		// 只以「每页 ≤ 首页字节」为假设（CLI-120-2），服务端分页异常（后续页
		// 实际字节超过首页）可绕过估算预算。与 getCirclesJSON 的 N-04 口径
		// 对齐：实际累积越过 maxAssembleBuffer 时二分截断到合法前缀，再交给
		// assembleCirclesLimitJSON 输出（保证 Bytes.Buffer 永不无上限增长）。
		if capAssembledSlice(raw1, results, endPage) {
			budgetPage := endPage
			for budgetPage > 1 && cumulativeSliceBytes(raw1, results, budgetPage) > maxAssembleBuffer {
				budgetPage--
			}
			slog.Warn("raw_json: limit 多页累积量超过合并预算，截断到已合并前缀",
				"total_bytes", cumulativeSliceBytes(raw1, results, endPage),
				"budget_page", budgetPage, "max", maxAssembleBuffer)
			return c.assembleCirclesLimitJSON(results, pb, offset, limit, budgetPage, methodName, nil)
		}
	}

	return c.assembleCirclesLimitJSON(results, pb, offset, limit, endPage, methodName, nil)
}

// assembleCirclesLimitJSON 将并发翻页结果按 offset/limit 规则拼接为最终 JSON 数组。
// endPage 是实际请求到的最大页号（<= TotalPage），results 长度对应 endPage+1。
func (c *Client) assembleCirclesLimitJSON(results []rawResult, pb *types.PageBean, offset, limit, endPage int, methodName string, partialErr error) (json.RawMessage, *types.PageBean, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 2048))
	buf.WriteByte('[')
	first := true
	taken := 0
	skipped := 0

	// 按页号顺序处理数据，应用 offset/limit 规则（只遍历已请求页）。
	// endPage 已由调用方按 totalNum/totalPage 推导并与 results 长度对齐，
	// 不能再次按原始 totalPage 截断，否则虚低声明会丢失已请求页。
	for pn := 1; pn <= endPage && pn < len(results); pn++ {
		if len(results[pn].raw) == 0 {
			continue
		}
		skipped, taken = appendPageRange(buf, results[pn].raw, &first, taken, offset, limit, skipped)
		if taken >= limit {
			break
		}
	}

	buf.WriteByte(']')
	if partialErr != nil {
		return json.RawMessage(trimArrayToCurrent(buf.Bytes())), pb, partialErr
	}
	return buf.Bytes(), pb, nil
}

// appendPageRange 从一页原始 JSON 数组中按 offset/limit 规则逐条取出写入 buf。
//
// pageRaw 是 "[{...},{...}]" 格式。用深度扫描分割 JSON 对象，不反序列化为 Go struct。
// 注意：深度扫描感知 JSON 字符串边界，避免字符串内的花括号被误计为对象边界。
func appendPageRange(buf *bytes.Buffer, pageRaw []byte, first *bool, taken, offset, limit, skipped int) (int, int) {
	trimmed := trimArrayBrackets(pageRaw)
	if len(trimmed) == 0 {
		return skipped, taken
	}
	start := 0
	depth := 0
	inString := false
	for i := 0; i <= len(trimmed) && taken < limit; i++ {
		if i < len(trimmed) {
			ch := trimmed[i]
			// JSON 字符串边界感知：在字符串内时，跳过转义字符，只在非转义的 " 处切换状态
			if inString {
				if ch == '\\' && i+1 < len(trimmed) {
					i++ // 跳过转义字符（如 \"、\\、\n 等）
				} else if ch == '"' {
					inString = false
				}
				continue
			}
			switch ch {
			case '"':
				inString = true
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

// FetchTasksJSON 拉取全维度任务列表，返回平台原始 JSON 数组（跨维度合并）。
//
// 自动跨维度并发拉取并在 SDK 内部合并为单一 JSON 数组，保留所有平台字段。
// 错误聚合与 FetchTasks 对齐：
//   - context 取消且无 partial → ErrRetryable
//   - context 取消且有 partial → ErrBusinessRejected + ErrRetryable + 已合并字节
//   - 业务错误 → 仍返回已合并的字节 + ErrBusinessRejected（cmd 层 partial envelope）
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
	// CLI-1：维度数上界钳制（对齐 maxTotalPage 纪律）——getDimensions 的
	// 维度数来自服务端声明，恶意值驱动全维度并发拉取×单页 4MB 累积无预算。
	// 128 远超任何真实学校维度集（通常 <10），截断保留前 128 维并 Warn。
	if len(activeDims) > maxFetchTasksDims {
		slog.Warn("FetchTasksJSON: 维度数超过钳制上限，截断到前 128 维",
			"dims", len(activeDims), "max", maxFetchTasksDims)
		activeDims = activeDims[:maxFetchTasksDims]
	}

	// 使用索引切片按维度顺序收集结果，保持维度顺序确定性。
	// channel-based 收集顺序取决于 goroutine 调度，结果顺序不可预测。
	results := make([][]byte, len(activeDims))
	dimErrs := make([]error, 0, len(activeDims))
	var mu sync.Mutex

	// 使用 errgroup 控制并发，限制最多 8 路并发，与 FetchTasks 保持一致
	g, gctx := errgroup.WithContext(ctx)
	limit := len(activeDims)
	if limit > fetchTasksConcurrentLimit {
		limit = fetchTasksConcurrentLimit
	}
	g.SetLimit(limit)

	for i, dim := range activeDims {
		dim := dim
		idx := i
		g.Go(func() error {
			// context 取消后直接 propagate，防止 cancel 被 dimErrs 吞掉后
			// 统一包装为 ErrBusinessRejected，丢失 ErrRetryable 可重试语义。
			// 对齐 FetchTasks 的 isContextError / ErrRetryable 分流。
			if err := gctx.Err(); err != nil {
				return err
			}
			raw, err := c.fetchTasksDimensionJSON(gctx, dim, headers)
			if err != nil {
				// 维度级 ctx 错误也要 propagate，让 g.Wait 走 cancel 分支
				if isContextError(err) {
					return err
				}
				appendLocked(&mu, &dimErrs, err)
				return nil // 业务错误记录到 dimErrs，不取消其他维度
			}
			results[idx] = raw
			return nil
		})
	}

	// 先按维度顺序拼装已有结果，供 cancel / partial 路径复用
	assemble := func() (json.RawMessage, int) {
		buf := bytes.NewBuffer(make([]byte, 0, 2048))
		buf.WriteByte('[')
		first := true
		totalPages := 0
		// CLI-1：累积字节预算（对齐 getCirclesJSON 的 maxAssembleBuffer 纪律）。
		// 各维度单页最大 maxResponseBodySize（4MB）×维度数可单请求累积上 GB；
		// 超出预算后停止追加，返回已合并的合法 JSON 前缀。
		assembledLen := 0
		for _, raw := range results {
			if len(raw) == 0 {
				continue
			}
			trimmed := trimArrayBrackets(raw)
			if len(trimmed) == 0 {
				continue
			}
			if assembledLen+len(trimmed)+2 > maxAssembleBuffer {
				slog.Warn("FetchTasksJSON: 累积字节超过预算，截断到已合并前缀",
					"assembled", assembledLen, "max", maxAssembleBuffer)
				break
			}
			if first {
				buf.Write(trimmed)
				first = false
			} else {
				buf.WriteByte(',')
				buf.Write(trimmed)
			}
			assembledLen += len(trimmed) + 1
			totalPages++
		}
		buf.WriteByte(']')
		return buf.Bytes(), totalPages
	}

	if err := g.Wait(); err != nil {
		if isContextError(err) {
			merged, n := assemble()
			if n > 0 {
				// partial + cancel：双包 ErrBusinessRejected + ErrRetryable
				return merged, fmt.Errorf("%w: FetchTasksJSON context 取消后部分维度成功: %w",
					ErrBusinessRejected,
					fmt.Errorf("%w: %w", ErrRetryable, err))
			}
			// 全 cancel：裸 ErrRetryable
			return nil, fmt.Errorf("%w: FetchTasksJSON 全部维度因 context 取消失败: %w", ErrRetryable, err)
		}
		return nil, fmt.Errorf("FetchTasksJSON 并发拉取失败: %w", err)
	}

	merged, totalPages := assemble()

	if totalPages == 0 {
		if len(dimErrs) > 0 {
			// 分离 dimErrs 中可能残留的 ctx 错误（防御性）
			var bizErrs []error
			var cancelledCount int
			for _, de := range dimErrs {
				if isContextError(de) {
					cancelledCount++
					continue
				}
				bizErrs = append(bizErrs, de)
			}
			if len(bizErrs) == 0 && cancelledCount > 0 {
				return nil, fmt.Errorf("%w: FetchTasksJSON 全部维度因 context 取消失败: %w",
					ErrRetryable, errors.Join(dimErrs...))
			}
			return nil, fmt.Errorf("%w: FetchTasksJSON 全部维度失败: %w", ErrBusinessRejected, errors.Join(dimErrs...))
		}
		return []byte("[]"), nil
	}

	if len(dimErrs) > 0 {
		var bizErrs []error
		var ctxErrs []error
		var cancelledCount int
		for _, de := range dimErrs {
			if isContextError(de) {
				cancelledCount++
				ctxErrs = append(ctxErrs, de)
				continue
			}
			bizErrs = append(bizErrs, de)
		}
		var cancelPlaceholder error
		if cancelledCount > 0 {
			cancelPlaceholder = fmt.Errorf("%w: %d 个维度因 context 取消而失败", ErrRetryable, cancelledCount)
		}
		if len(bizErrs) == 0 && cancelledCount > 0 {
			joined := errors.Join(append(ctxErrs, cancelPlaceholder)...)
			return merged, fmt.Errorf("%w: FetchTasksJSON context 取消后部分维度成功: %w",
				ErrBusinessRejected, joined)
		}
		joined := errors.Join(append(append(bizErrs, ctxErrs...), cancelPlaceholder)...)
		return merged, fmt.Errorf("%w: FetchTasksJSON %d 个维度失败: %w",
			ErrBusinessRejected, len(bizErrs), joined)
	}
	return merged, nil
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
	resp, err := decodeOrInvalidResponse(fmt.Sprintf("维度 %d(%s)", dim.ID, dim.Name), bodyBytes)
	if err != nil {
		return nil, err
	}
	if err := types.CheckCode(resp); err != nil {
		return nil, fmt.Errorf("%w: 维度 %d(%s) 业务错误: %w", ErrBusinessRejected, dim.ID, dim.Name, err)
	}
	if resp.DataList == nil {
		return []byte("[]"), nil
	}
	return *resp.DataList, nil
}

// marshalUserInfoJSON 将 UserInfo 序列化为 JSON，输出前剔除 StudentUuid 敏感值。
//
// P2-1（user-info 域 18 轮审计）：StudentUuid 是密码/学生 UUID 载体（写侧），
// 前端 modifyBox.vue:185 读取 getMyInfo 响应后显式清零佐证其只写不读的敏感属性。
// 结构化 GetMyInfo 返回的 info 保留该字段（Go 调用方自行裁决）；
// 但 JSON 透传路径（CLI whoami / GetMyInfoJSON / ActivateSessionJSON 消费）必须剔除，
// 防止敏感值经 envelope 原样透给脚本消费者与日志。
//
// 关键约束（P0-A7 教训）：info 与 sm.cachedUserInfo 共享同一指针，禁止原地置空；
// 必须浅拷贝副本后置空。拷贝只涉及 string 字段，成本 O(1) 级。
func marshalUserInfoJSON(info *types.UserInfo, caller string) (json.RawMessage, error) {
	if info == nil {
		return nil, nil
	}
	copyInfo := *info
	copyInfo.StudentUuid = ""
	raw, err := json.Marshal(&copyInfo)
	if err != nil {
		return nil, fmt.Errorf("%s 序列化失败: %w", caller, err)
	}
	return raw, nil
}

// ActivateSessionJSON 激活业务 session，返回 /api/studentInfo/getMyInfo 的 JSON（经 SDK 后处理）。
//
// 经 sessionManager 激活（含 4 步 HAR 请求），Marshal 其返回的后处理 UserInfo 为 JSON。
// 包含学校信息 SSO 降级补全、班级名年级前缀清理等后处理。
// 同时附带 4 步 HAR 激活所需的 HTTP 请求（首页 → getMenu ×2 → getMyInfo），
// 即使下游不消费返回的 UserInfo 也必须执行完这 4 步。
//
// 返回值：
//   - json.RawMessage：处理后的 UserInfo JSON 字节；数据为空时返回 nil。
//   - error：网络/解析/业务错误（含 ErrEmptyUserInfo / ErrSessionBackoff）
func (c *Client) ActivateSessionJSON(ctx context.Context, token string) (json.RawMessage, error) {
	info, err := c.GetMyInfo(ctx, token)
	if err != nil {
		// GetMyInfo 空数据返回 ErrEmptyUserInfo —— 保持 ActivateSessionJSON 原契约：
		// 数据为空时返回 (nil, nil)，不向调用方暴露哨兵。
		if errors.Is(err, ErrEmptyUserInfo) {
			return nil, nil
		}
		return nil, err
	}
	if info == nil {
		return nil, nil
	}
	return marshalUserInfoJSON(info, "ActivateSessionJSON")
}

// GetMyInfoJSON 获取当前用户完整个人资料的 JSON（经 SDK 后处理）。
//
// 内部调用 GetMyInfo() 获取已后处理的 UserInfo struct，再 Marshal 回 JSON。
// 包含 GetMyInfo 的全部后处理：学校信息 SSO 降级补全、班级名年级前缀清理。
//
// 返回值：
//   - json.RawMessage：处理后的 UserInfo JSON 字节；
//     当数据为空时返回 (nil, ErrEmptyUserInfo)。
//   - error：网络/解析/业务错误
func (c *Client) GetMyInfoJSON(ctx context.Context, token string) (json.RawMessage, error) {
	info, err := c.GetMyInfo(ctx, token)
	if err != nil {
		return nil, err
	}
	return marshalUserInfoJSON(info, "GetMyInfoJSON")
}

// QuerySelfEvaluationJSON 查询自我评价状态的原始 JSON。
//
// 等价 QuerySelfEvaluation 但保留平台原始字段。
// 结构化方法内部是 returnData → dataMap → dataList[0] 三解码器 fallback 链；
// 本方法走 rawSingleObjectBytes：returnData 对象优先，其次 dataList[0]，再 dataMap。
// 对账口径：前端唯一读取通道是 dataMap（mainLeft.vue）；若服务端异常地
// 双容器并存且内容不同，本方法透传的是 returnData 内容，与网页所见可能
// 不一致——对账请以结构化方法或 dataMap 字段为准。
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

// QuerySelfGradEvaluationJSON 查询毕业评价状态的原始 JSON。
//
// 等价 QuerySelfGradEvaluation 但保留平台原始字段。
// 前端 mainLeft.vue 主读 dataMap.student_comment / isGrad；
// 本方法走 rawSingleObjectBytes：returnData 对象优先，其次 dataList[0]，再 dataMap。
// 对账口径：若服务端异常地双容器并存且内容不同，透传的是 returnData 内容，
// 与网页所见的 dataMap 可能不一致——对账请以结构化方法或 dataMap 字段为准。
// 空数据时返回 (nil, nil)。
func (c *Client) QuerySelfGradEvaluationJSON(ctx context.Context, token string) (json.RawMessage, error) {
	resp, err := c.doBizAndDecode(ctx, token, "QuerySelfGradEvaluationJSON",
		"/api/studentMoralEduNew/querySelfGradEvaluation", http.MethodGet, nil)
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
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("[]")) {
		raw = nil
		if resp.ReturnData != nil {
			raw = *resp.ReturnData
		}
	}
	trimmed = bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte(`""`)) || bytes.Equal(trimmed, []byte("[]")) {
		return []byte("[]"), nil
	}
	return raw, nil
}

// assembleRecordsPageJSON 将 UnifiedResponse 拼装为 {"records":..., "page":...} 格式。
//
// records 取自 dataList（缺失时回退为 []），page 取自 pageBean（缺失时省略该字段）。
// 被 GetHonorListJSON 和 GetTypicalCaseListJSON 共用，避免重复拼装逻辑。
func assembleRecordsPageJSON(resp *types.UnifiedResponse) json.RawMessage {
	var recordsRaw json.RawMessage
	if resp.DataList != nil {
		recordsRaw = *resp.DataList
		// G2（Cycle 101）：dataList 的 null 形态（字面 null / 字符串 "null"）先归一，
		// 防脏数组元素 ["null"]。rawListBytes 同款归一（Cycle 94 F1 覆盖 getStudentCircle/
		// getHonorType，本函数是 GetHonorListJSON/GetTypicalCaseListJSON 共用拼装点）。
		if isNullJSON(recordsRaw) {
			recordsRaw = nil
		}
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
	return buf.Bytes()
}

// GetHonorListJSON 获取指定页荣誉记录的原始 JSON（不含自动翻页）。
//
// 与 GetHonorList 1:1 等价（按页调用，不自动翻页），
// 返回拼装后的完整 JSON 对象 `{"records":..., "page":...}`，
// records 和 page 字段值都是平台原始字节。
// key 为搜索关键字（可空，会做 URL 转义）。
func (c *Client) GetHonorListJSON(ctx context.Context, token string, pageNo, pageSize int, key string) (json.RawMessage, error) {
	path := "/api/studentMoralEduNew/getHonorByStudentId?pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(pageSize) + "&key=" + url.QueryEscape(key)
	resp, err := c.doBizAndDecode(ctx, token, "GetHonorListJSON", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetHonorListJSON 失败: %w", err)
	}
	return assembleRecordsPageJSON(resp), nil
}
