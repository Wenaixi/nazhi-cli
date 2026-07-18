package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"golang.org/x/sync/errgroup"
)

// ─── 通用辅助函数（带 type 参数）───

// fetchCirclePage 拉取一页写实记录，同时返回分页信息。
// circleType 对应 getStudentCircle 接口的 type 参数：
//
//	1=公示/全部, 2=教师写实, 3=我发布的, 4=被撤回
func (c *Client) fetchCirclePage(ctx context.Context, token string, pageNo, pageSize int, circleType int) ([]types.CircleRecord, *types.PageBean, error) {
	path := "/api/studentCircleNew/getStudentCircle?type=" + strconv.Itoa(circleType) + "&pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(pageSize) + "&key="

	resp, err := c.doBizAndDecode(ctx, token, "fetchCirclePage", path, http.MethodGet, nil)
	if err != nil {
		return nil, nil, err
	}

	// 解码分页信息
	pb, err := types.DecodePageBean(*resp)
	if err != nil {
		return nil, nil, fmt.Errorf("fetchCirclePage 解析分页信息失败: %w", err)
	}

	// 解码记录列表
	records, err := types.DecodeDataList[types.CircleRecord](*resp)
	if err != nil {
		return nil, nil, fmt.Errorf("fetchCirclePage 解析写实记录失败: %w", err)
	}

	return records, pb, nil
}

// fetchCirclePageJSON 拉取一页写实记录，返回原始 dataList 字节。
func (c *Client) fetchCirclePageJSON(ctx context.Context, token string, pageNo, pageSize int, circleType int) ([]types.CircleRecord, *types.PageBean, []byte, error) {
	path := "/api/studentCircleNew/getStudentCircle?type=" + strconv.Itoa(circleType) + "&pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(pageSize) + "&key="

	resp, err := c.doBizAndDecode(ctx, token, "fetchCirclePageJSON", path, http.MethodGet, nil)
	if err != nil {
		return nil, nil, nil, err
	}

	pb, err := types.DecodePageBean(*resp)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fetchCirclePageJSON 解析分页信息失败: %w", err)
	}
	records, err := types.DecodeDataList[types.CircleRecord](*resp)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fetchCirclePageJSON 解析写实记录失败: %w", err)
	}
	return records, pb, rawListBytes(*resp), nil
}

// concurrentPageLimit 是翻页并发的上限，避免打爆服务端。
const concurrentPageLimit = 4

// fetchAllCirclePages 通用翻页合并，≤pageSize 条时直接返回单页数据，
// 超过时并发拉取剩余页并保持顺序合并。
//
// 约定：pageSize 默认 500，服务端上限也是 500，所以 ≤500 的场景零额外请求。
// 2533 条时约 5 个剩余页，4 路并发一次完成（约 1 RTT），而非串行 5 次。
//
// pageResult 是内部收集器，避免 errgroup goroutine 中 map 竞态写入。
type pageResult struct {
	records []types.CircleRecord
}

func (c *Client) fetchAllCirclePages(ctx context.Context, token string, circleType int) ([]types.CircleRecord, error) {
	pageSize := c.submittedPageSize
	if pageSize <= 0 {
		pageSize = defaultSubmittedPageSize
	}

	page1, pb, err := c.fetchCirclePage(ctx, token, 1, pageSize, circleType)
	if err != nil {
		return nil, fmt.Errorf("获取写实记录失败: %w", err)
	}

	// 单页覆盖（≤500 条），直接返回
	if pb == nil || pb.TotalPage <= 1 || pb.TotalNum <= pageSize {
		return page1, nil
	}

	// 多页：预分配容量后并发翻页
	all := make([]types.CircleRecord, len(page1), pb.TotalNum)
	copy(all, page1)

	// results 按页号索引，预分配避免竞态
	results := make([]pageResult, pb.TotalPage+1)
	results[1] = pageResult{records: page1}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrentPageLimit)

	for pageNo := 2; pageNo <= pb.TotalPage; pageNo++ {
		pn := pageNo
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			records, _, err := c.fetchCirclePage(gctx, token, pn, pageSize, circleType)
			if err != nil {
				return fmt.Errorf("第 %d 页失败: %w", pn, err)
			}
			results[pn] = pageResult{records: records}
			return nil
		})
	}

	// 等待全部完成
	if err := g.Wait(); err != nil {
		// 有部分失败，但已成功拉取的页仍然有效
		// 收集已有数据
		for pn := 2; pn <= pb.TotalPage; pn++ {
			all = append(all, results[pn].records...)
		}
		return all, err
	}

	// 全部成功，按页号顺序合并
	for pn := 2; pn <= pb.TotalPage; pn++ {
		all = append(all, results[pn].records...)
	}

	return all, nil
}

// fetchAllCirclePagesBytes 通用翻页合并的原始字节版。
// 语义同 fetchAllCirclePages，返回字节而非 Go struct。
func (c *Client) fetchAllCirclePagesBytes(ctx context.Context, token string, circleType int) ([]byte, *types.PageBean, error) {
	pageSize := c.submittedPageSize
	if pageSize <= 0 {
		pageSize = defaultSubmittedPageSize
	}

	_, pb, raw1, err := c.fetchCirclePageJSON(ctx, token, 1, pageSize, circleType)
	if err != nil {
		return nil, nil, fmt.Errorf("获取写实记录失败: %w", err)
	}

	// 单页覆盖
	if pb == nil || pb.TotalPage <= 1 || pb.TotalNum <= pageSize {
		return raw1, pb, nil
	}

	// 多页：并发拉取，用 map 暂存原始字节（pageNo→json bytes）
	type pageBytes struct {
		raw []byte
	}
	rawPages := make([]pageBytes, pb.TotalPage+1)
	rawPages[1] = pageBytes{raw: raw1}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrentPageLimit)

	for pageNo := 2; pageNo <= pb.TotalPage; pageNo++ {
		pn := pageNo
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			_, _, raw, err := c.fetchCirclePageJSON(gctx, token, pn, pageSize, circleType)
			if err != nil {
				return fmt.Errorf("第 %d 页失败: %w", pn, err)
			}
			rawPages[pn] = pageBytes{raw: raw}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		// 部分失败，返回已有数据 + 错误
		buf := newByteBuffer(raw1, pb.TotalPage)
		buf.Write(trimArrayBrackets(raw1))
		for pn := 2; pn <= pb.TotalPage; pn++ {
			if len(rawPages[pn].raw) > 0 {
				buf.WriteByte(',')
				buf.Write(trimArrayBrackets(rawPages[pn].raw))
			}
		}
		buf.WriteByte(']')
		return buf.Bytes(), pb, err
	}

	// 全部成功，拼接
	buf := newByteBuffer(raw1, pb.TotalPage)
	buf.Write(trimArrayBrackets(raw1))
	for pn := 2; pn <= pb.TotalPage; pn++ {
		if len(rawPages[pn].raw) > 0 {
			buf.WriteByte(',')
			buf.Write(trimArrayBrackets(rawPages[pn].raw))
		}
	}
	buf.WriteByte(']')
	return buf.Bytes(), pb, nil
}

// newByteBuffer 创建合适初始容量的 bytes.Buffer。
// 用 sync.Pool 或 make([]byte, 0, cap) 预分配减少 reallocation。
func newByteBuffer(page1 []byte, totalPages int) *bytesBufferWriter {
	cap := len(page1) * totalPages
	if cap < 2048 {
		cap = 2048
	}
	return &bytesBufferWriter{
		buf: make([]byte, 0, cap),
	}
}

// bytesBufferWriter 简化 bytes.Buffer 的写入操作。
type bytesBufferWriter struct {
	buf []byte
}

func (w *bytesBufferWriter) Write(p []byte) {
	w.buf = append(w.buf, p...)
}

func (w *bytesBufferWriter) WriteByte(b byte) error {
	w.buf = append(w.buf, b)
	return nil
}

func (w *bytesBufferWriter) Bytes() []byte {
	return w.buf
}

// ─── type=1: 公示/全部（全班所有记录）───

// GetPublicCircles 获取公示的全部写实记录（全班）。
func (c *Client) GetPublicCircles(ctx context.Context, token string) ([]types.CircleRecord, error) {
	return c.fetchAllCirclePages(ctx, token, 1)
}

// PeekPublicTotal 快速获取公示写实记录总数。
func (c *Client) PeekPublicTotal(ctx context.Context, token string) (int, error) {
	_, pb, err := c.fetchCirclePage(ctx, token, 1, 1, 1)
	if err != nil {
		return 0, fmt.Errorf("PeekPublicTotal 失败: %w", err)
	}
	if pb == nil {
		return 0, nil
	}
	return pb.TotalNum, nil
}

// ─── type=2: 教师写实 ───

// GetTeacherCircles 获取教师代写的全部写实记录。
func (c *Client) GetTeacherCircles(ctx context.Context, token string) ([]types.CircleRecord, error) {
	return c.fetchAllCirclePages(ctx, token, 2)
}

// PeekTeacherTotal 快速获取教师写实记录总数。
func (c *Client) PeekTeacherTotal(ctx context.Context, token string) (int, error) {
	_, pb, err := c.fetchCirclePage(ctx, token, 1, 1, 2)
	if err != nil {
		return 0, fmt.Errorf("PeekTeacherTotal 失败: %w", err)
	}
	if pb == nil {
		return 0, nil
	}
	return pb.TotalNum, nil
}

// ─── type=3: 我发布的写实（仅当前用户自己的记录）───

// GetSubmittedCircles 获取当前用户自己发布的写实记录。
func (c *Client) GetSubmittedCircles(ctx context.Context, token string) ([]types.CircleRecord, error) {
	return c.fetchAllCirclePages(ctx, token, 3)
}

// PeekSubmittedTotal 快速获取已提交写实记录总数（type=3，我发布的）。
func (c *Client) PeekSubmittedTotal(ctx context.Context, token string) (int, error) {
	_, pb, err := c.fetchCirclePage(ctx, token, 1, 1, 3)
	if err != nil {
		return 0, fmt.Errorf("PeekSubmittedTotal 失败: %w", err)
	}
	if pb == nil {
		return 0, nil
	}
	return pb.TotalNum, nil
}

// fetchSubmittedPageJSON 拉取一页已提交写实记录，返回原始 dataList 字节。
//
// 已弃用：请使用 fetchCirclePageJSON。
func (c *Client) fetchSubmittedPageJSON(ctx context.Context, token string, pageNo, pageSize int) ([]types.CircleRecord, *types.PageBean, []byte, error) {
	return c.fetchCirclePageJSON(ctx, token, pageNo, pageSize, 3)
}

// ─── type=4: 被撤回的写实 ───

// GetWithdrawnCircles 获取被撤回的全部写实记录。
func (c *Client) GetWithdrawnCircles(ctx context.Context, token string) ([]types.CircleRecord, error) {
	return c.fetchAllCirclePages(ctx, token, 4)
}

// PeekWithdrawnTotal 快速获取被撤回写实记录总数。
func (c *Client) PeekWithdrawnTotal(ctx context.Context, token string) (int, error) {
	_, pb, err := c.fetchCirclePage(ctx, token, 1, 1, 4)
	if err != nil {
		return 0, fmt.Errorf("PeekWithdrawnTotal 失败: %w", err)
	}
	if pb == nil {
		return 0, nil
	}
	return pb.TotalNum, nil
}

func init() {
	// 确保 concurrentPageLimit 不超过合理的并发上限
	// 服务端 pageSize 上限 500，6 页以内 4 路并发即可一次完成
}
