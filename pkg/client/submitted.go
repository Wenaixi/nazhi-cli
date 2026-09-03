package client

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"golang.org/x/sync/errgroup"
)

// ─── 通用辅助函数（带 type 参数）───

func buildCirclePath(circleType, pageNo, pageSize int, key string) string {
	return "/api/studentCircleNew/getStudentCircle?type=" + strconv.Itoa(circleType) + "&pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(pageSize) + "&key=" + url.QueryEscape(key)
}

func (c *Client) effectivePageSize() int {
	if c.submittedPageSize > 0 {
		return c.submittedPageSize
	}
	return defaultSubmittedPageSize
}

// fetchCirclePage 拉取一页写实记录，同时返回分页信息。
// circleType 对应 getStudentCircle 接口的 type 参数：
//
//	1=公示/全部, 2=教师写实, 3=我发布的, 4=被撤回
func (c *Client) fetchCirclePage(ctx context.Context, token string, pageNo, pageSize int, circleType int, key string) ([]types.CircleRecord, *types.PageBean, error) {
	path := buildCirclePath(circleType, pageNo, pageSize, key)

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
func (c *Client) fetchCirclePageJSON(ctx context.Context, token string, pageNo, pageSize int, circleType int, key string) (*types.PageBean, []byte, error) {
	path := buildCirclePath(circleType, pageNo, pageSize, key)

	resp, err := c.doBizAndDecode(ctx, token, "fetchCirclePageJSON", path, http.MethodGet, nil)
	if err != nil {
		return nil, nil, err
	}

	pb, err := types.DecodePageBean(*resp)
	if err != nil {
		return nil, nil, fmt.Errorf("fetchCirclePageJSON 解析分页信息失败: %w", err)
	}
	return pb, rawListBytes(*resp), nil
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

func (c *Client) fetchAllCirclePages(ctx context.Context, token string, circleType int, key string) ([]types.CircleRecord, error) {
	pageSize := c.effectivePageSize()

	page1, pb, err := c.fetchCirclePage(ctx, token, 1, pageSize, circleType, key)
	if err != nil {
		return nil, fmt.Errorf("获取写实记录失败: %w", err)
	}

	// 单页覆盖（≤500 条），直接返回。
	// ponytail: 短路谓词看 TotalNum，翻页上界却只用 TotalPage——二者均信任服务端自洽。
	// 若服务端同一响应内 totalPage 虚低于 ceil(totalNum/pageSize)（双重违约，真实抓包未现），
	// 会静默截断尾部数据；需要防御时把循环上界改为 max(TotalPage, ceil(TotalNum/pageSize))。
	if pb == nil || pb.TotalPage <= 1 || pb.TotalNum <= pageSize {
		return page1, nil
	}

	// 多页：预分配容量后并发翻页。
	// ponytail: cap 以 max(len(page1), totalNum) 钳制——totalNum 取自服务端声明、
	// len(page1) 取自实际解码条数，二者独立来源；服务端 count/list 短暂不一致时
	// totalNum < len(page1) 会让 make 直接 makeslice panic，钳制为防御纵深。
	capacity := pb.TotalNum
	if capacity < len(page1) {
		capacity = len(page1)
	}
	// C-F 修复（扩展）：capacity 也钳制——totalNum 同样来自服务端单字段声明，
	// 恶意值（如 1e9）会让 make 预分配巨大容量直接 OOM。上限 = 钳制页数 × pageSize。
	if capacity > maxTotalPage*pageSize {
		capacity = maxTotalPage * pageSize
	}
	all := make([]types.CircleRecord, len(page1), capacity)
	copy(all, page1)

	// results 按页号索引，预分配避免竞态
	// C-F 修复：TotalPage 来自服务端单字段声明，超钳制值时直接截断（只返回
	// 首页），不翻页——防服务端异常值驱动 make 分配 OOM。
	if pb.TotalPage > maxTotalPage {
		slog.Warn("submitted: totalPage 超过钳制上限，截断到首页", "total_page", pb.TotalPage, "max", maxTotalPage)
		return all, nil
	}
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
			records, _, err := c.fetchCirclePage(gctx, token, pn, pageSize, circleType, key)
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

// ─── type=1: 公示/全部（全班所有记录）───

// GetPublicCircles 获取公示的全部写实记录（全班）。
func (c *Client) GetPublicCircles(ctx context.Context, token string, key string) ([]types.CircleRecord, error) {
	return c.fetchAllCirclePages(ctx, token, 1, key)
}

// PeekPublicTotal 快速获取公示写实记录总数。
func (c *Client) PeekPublicTotal(ctx context.Context, token string, key string) (int, error) {
	_, pb, err := c.fetchCirclePage(ctx, token, 1, 1, 1, key)
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
func (c *Client) GetTeacherCircles(ctx context.Context, token string, key string) ([]types.CircleRecord, error) {
	return c.fetchAllCirclePages(ctx, token, 2, key)
}

// PeekTeacherTotal 快速获取教师写实记录总数。
func (c *Client) PeekTeacherTotal(ctx context.Context, token string, key string) (int, error) {
	_, pb, err := c.fetchCirclePage(ctx, token, 1, 1, 2, key)
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
func (c *Client) GetSubmittedCircles(ctx context.Context, token string, key string) ([]types.CircleRecord, error) {
	return c.fetchAllCirclePages(ctx, token, 3, key)
}

// PeekSubmittedTotal 快速获取已提交写实记录总数（type=3，我发布的）。
func (c *Client) PeekSubmittedTotal(ctx context.Context, token string, key string) (int, error) {
	_, pb, err := c.fetchCirclePage(ctx, token, 1, 1, 3, key)
	if err != nil {
		return 0, fmt.Errorf("PeekSubmittedTotal 失败: %w", err)
	}
	if pb == nil {
		return 0, nil
	}
	return pb.TotalNum, nil
}

// ─── type=4: 被撤回的写实 ───

// GetWithdrawnCircles 获取被撤回的全部写实记录。
func (c *Client) GetWithdrawnCircles(ctx context.Context, token string, key string) ([]types.CircleRecord, error) {
	return c.fetchAllCirclePages(ctx, token, 4, key)
}

// PeekWithdrawnTotal 快速获取被撤回写实记录总数。
func (c *Client) PeekWithdrawnTotal(ctx context.Context, token string, key string) (int, error) {
	_, pb, err := c.fetchCirclePage(ctx, token, 1, 1, 4, key)
	if err != nil {
		return 0, fmt.Errorf("PeekWithdrawnTotal 失败: %w", err)
	}
	if pb == nil {
		return 0, nil
	}
	return pb.TotalNum, nil
}
