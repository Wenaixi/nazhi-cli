package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

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
	page1, pb, err := c.fetchSubmittedPage(ctx, token, 1, pageSize)
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

		records, _, err := c.fetchSubmittedPage(ctx, token, pageNo, pageSize)
		if err != nil {
			// 翻页失败时返回已有数据 + 错误信号
			return all, fmt.Errorf("GetSubmittedCircles 第 %d 页失败: %w", pageNo, err)
		}
		all = append(all, records...)
	}

	return all, err
}

// fetchSubmittedPage 拉取一页已提交写实记录，同时返回分页信息。
func (c *Client) fetchSubmittedPage(ctx context.Context, token string, pageNo, pageSize int) ([]types.CircleRecord, *types.PageBean, error) {
	path := "/api/studentCircleNew/getStudentCircle?type=1&pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(pageSize) + "&key="

	resp, err := c.doBizAndDecode(ctx, token, "GetSubmittedCircles", path, http.MethodGet, nil)
	if err != nil {
		return nil, nil, err
	}

	// 解码分页信息
	pb, err := types.DecodePageBean(*resp)
	if err != nil {
		return nil, nil, fmt.Errorf("GetSubmittedCircles 解析分页信息失败: %w", err)
	}

	// 解码记录列表
	records, err := types.DecodeDataList[types.CircleRecord](*resp)
	if err != nil {
		return nil, nil, fmt.Errorf("GetSubmittedCircles 解析写实记录失败: %w", err)
	}

	return records, pb, nil
}
