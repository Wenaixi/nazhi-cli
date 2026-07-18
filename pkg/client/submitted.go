package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ─── 通用辅助函数（带 type 参数）───

// fetchCirclePage 拉取一页写实记录，同时返回分页信息。
// circleType 对应 getStudentCircle 接口的 type 参数：
//
//	1=已发布, 2=教师写实, 3=被撤回, 4=公示
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

// ─── type=1: 已发布的写实（原有方法，保持向后兼容）───

// GetSubmittedCircles 获取当前用户已提交的全部写实记录。
//
// 内部流程：ActivateSession → 循环 getStudentCircle(pageNo) 直到 totalPage。
// 自动合并多页数据，调用方拿到的是全量已提交记录。
//
// 单页就能覆盖绝大多数场景（pageSize 默认 100，服务端上限 500），
// 只有记录超过每页条数时才翻页。每页条数可用 WithSubmittedPageSize 配置。
func (c *Client) GetSubmittedCircles(ctx context.Context, token string) ([]types.CircleRecord, error) {
	pageSize := c.submittedPageSize
	if pageSize <= 0 {
		pageSize = defaultSubmittedPageSize
	}

	// 第一页请求，同时获取分页信息
	page1, pb, err := c.fetchCirclePage(ctx, token, 1, pageSize, 1)
	if err != nil {
		return nil, fmt.Errorf("GetSubmittedCircles 失败: %w", err)
	}

	// totalPage ≤ 1 时直接返回第一页数据
	if pb == nil || pb.TotalPage <= 1 || pb.TotalNum <= pageSize {
		return page1, nil
	}

	// 多页：预分配容量后翻页合并
	all := make([]types.CircleRecord, len(page1), pb.TotalNum)
	copy(all, page1)

	for pageNo := 2; pageNo <= pb.TotalPage; pageNo++ {
		// context 取消时返回已有数据 + error，让调用方感知截断
		if err := ctx.Err(); err != nil {
			return all, err
		}

		records, _, err := c.fetchCirclePage(ctx, token, pageNo, pageSize, 1)
		if err != nil {
			// 翻页失败时返回已有数据 + 错误信号
			return all, fmt.Errorf("GetSubmittedCircles 第 %d 页失败: %w", pageNo, err)
		}
		all = append(all, records...)
	}

	return all, nil
}

// PeekSubmittedTotal 快速获取已提交写实记录总数。
//
// 内部调用 getStudentCircle?type=1&pageNo=1&pageSize=1，只提取 PageBean.TotalNum。
// 只拉 1 条记录获取分页信息，不拉全量列表，比 GetSubmittedCirclesLimitJSON(0,1) 更轻量。
//
// 返回总记录数；出错时返回 0 和 error。
func (c *Client) PeekSubmittedTotal(ctx context.Context, token string) (int, error) {
	_, pb, err := c.fetchCirclePage(ctx, token, 1, 1, 1)
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
	return c.fetchCirclePageJSON(ctx, token, pageNo, pageSize, 1)
}

// ─── type=2: 教师写实 ───

// GetTeacherCircles 获取教师代写的全部写实记录。
func (c *Client) GetTeacherCircles(ctx context.Context, token string) ([]types.CircleRecord, error) {
	pageSize := c.submittedPageSize
	if pageSize <= 0 {
		pageSize = defaultSubmittedPageSize
	}

	page1, pb, err := c.fetchCirclePage(ctx, token, 1, pageSize, 2)
	if err != nil {
		return nil, fmt.Errorf("GetTeacherCircles 失败: %w", err)
	}

	if pb == nil || pb.TotalPage <= 1 || pb.TotalNum <= pageSize {
		return page1, nil
	}

	all := make([]types.CircleRecord, len(page1), pb.TotalNum)
	copy(all, page1)

	for pageNo := 2; pageNo <= pb.TotalPage; pageNo++ {
		if err := ctx.Err(); err != nil {
			return all, err
		}
		records, _, err := c.fetchCirclePage(ctx, token, pageNo, pageSize, 2)
		if err != nil {
			return all, fmt.Errorf("GetTeacherCircles 第 %d 页失败: %w", pageNo, err)
		}
		all = append(all, records...)
	}

	return all, nil
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

// ─── type=3: 被撤回的写实 ───

// GetWithdrawnCircles 获取被撤回的全部写实记录。
func (c *Client) GetWithdrawnCircles(ctx context.Context, token string) ([]types.CircleRecord, error) {
	pageSize := c.submittedPageSize
	if pageSize <= 0 {
		pageSize = defaultSubmittedPageSize
	}

	page1, pb, err := c.fetchCirclePage(ctx, token, 1, pageSize, 3)
	if err != nil {
		return nil, fmt.Errorf("GetWithdrawnCircles 失败: %w", err)
	}

	if pb == nil || pb.TotalPage <= 1 || pb.TotalNum <= pageSize {
		return page1, nil
	}

	all := make([]types.CircleRecord, len(page1), pb.TotalNum)
	copy(all, page1)

	for pageNo := 2; pageNo <= pb.TotalPage; pageNo++ {
		if err := ctx.Err(); err != nil {
			return all, err
		}
		records, _, err := c.fetchCirclePage(ctx, token, pageNo, pageSize, 3)
		if err != nil {
			return all, fmt.Errorf("GetWithdrawnCircles 第 %d 页失败: %w", pageNo, err)
		}
		all = append(all, records...)
	}

	return all, nil
}

// PeekWithdrawnTotal 快速获取被撤回写实记录总数。
func (c *Client) PeekWithdrawnTotal(ctx context.Context, token string) (int, error) {
	_, pb, err := c.fetchCirclePage(ctx, token, 1, 1, 3)
	if err != nil {
		return 0, fmt.Errorf("PeekWithdrawnTotal 失败: %w", err)
	}
	if pb == nil {
		return 0, nil
	}
	return pb.TotalNum, nil
}

// ─── type=4: 公示查看（全班）───

// GetPublicCircles 获取公示的全部写实记录（全班）。
func (c *Client) GetPublicCircles(ctx context.Context, token string) ([]types.CircleRecord, error) {
	pageSize := c.submittedPageSize
	if pageSize <= 0 {
		pageSize = defaultSubmittedPageSize
	}

	page1, pb, err := c.fetchCirclePage(ctx, token, 1, pageSize, 4)
	if err != nil {
		return nil, fmt.Errorf("GetPublicCircles 失败: %w", err)
	}

	if pb == nil || pb.TotalPage <= 1 || pb.TotalNum <= pageSize {
		return page1, nil
	}

	all := make([]types.CircleRecord, len(page1), pb.TotalNum)
	copy(all, page1)

	for pageNo := 2; pageNo <= pb.TotalPage; pageNo++ {
		if err := ctx.Err(); err != nil {
			return all, err
		}
		records, _, err := c.fetchCirclePage(ctx, token, pageNo, pageSize, 4)
		if err != nil {
			return all, fmt.Errorf("GetPublicCircles 第 %d 页失败: %w", pageNo, err)
		}
		all = append(all, records...)
	}

	return all, nil
}

// PeekPublicTotal 快速获取公示写实记录总数。
func (c *Client) PeekPublicTotal(ctx context.Context, token string) (int, error) {
	_, pb, err := c.fetchCirclePage(ctx, token, 1, 1, 4)
	if err != nil {
		return 0, fmt.Errorf("PeekPublicTotal 失败: %w", err)
	}
	if pb == nil {
		return 0, nil
	}
	return pb.TotalNum, nil
}
