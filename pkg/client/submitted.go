package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// submittedPageSize 是 GetSubmittedCircles 每页请求条数。
// HAR 实测 pageSize=3 安全，pageSize=100 服务端返回 400。
// 保守用 20，绝大多数场景单页即可覆盖，>20 条时自动翻页。
const submittedPageSize = 20

// HAR 实测 pageSize=3 安全，pageSize=100 服务端返回 400。
// 去掉 &key= 参数也会 400，页面实际请求会带 &key= 空值。
// 保守用 20，绝大多数场景单页即可覆盖，>20 条时自动翻页。

// GetSubmittedCircles 获取当前用户已提交的全部写实记录。
//
// 内部流程：ActivateSession → 循环 getStudentCircle(pageNo) 直到 totalPage。
// 自动合并多页数据，调用方拿到的是全量已提交记录。
//
// 单页就能覆盖绝大多数场景（pageSize=100），只有记录 >100 条时才翻页。
func (c *Client) GetSubmittedCircles(ctx context.Context, token string) ([]types.CircleRecord, error) {
	// 第一页请求，同时获取分页信息
	page1, pb, err := c.fetchSubmittedPage(ctx, token, 1)
	if err != nil {
		return nil, fmt.Errorf("GetSubmittedCircles 失败: %w", err)
	}

	// totalPage ≤ 1 时直接返回第一页数据
	if pb == nil || pb.TotalPage <= 1 || pb.TotalNum <= submittedPageSize {
		return page1, nil
	}

	// 多页：预分配容量后翻页合并
	all := make([]types.CircleRecord, len(page1), pb.TotalNum)
	copy(all, page1)

	for pageNo := 2; pageNo <= pb.TotalPage; pageNo++ {
		// context 取消时提前返回已有数据
		if err := ctx.Err(); err != nil {
			return all, nil
		}

		records, _, err := c.fetchSubmittedPage(ctx, token, pageNo)
		if err != nil {
			// 翻页失败时返回已有数据 + 错误信号
			return all, fmt.Errorf("GetSubmittedCircles 第 %d 页失败: %w", pageNo, err)
		}
		all = append(all, records...)
	}

	return all, nil
}

// fetchSubmittedPage 拉取一页已提交写实记录，同时返回分页信息。
func (c *Client) fetchSubmittedPage(ctx context.Context, token string, pageNo int) ([]types.CircleRecord, *types.PageBean, error) {
	path := "/api/studentCircleNew/getStudentCircle?type=1&pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(submittedPageSize) + "&key="

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
