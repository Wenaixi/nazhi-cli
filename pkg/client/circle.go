package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// DeleteCircle 删除一条写实记录。
// GET /api/studentCircleNew/deleteCircle?id=
func (c *Client) DeleteCircle(ctx context.Context, token string, circleID int64) error {
	path := "/api/studentCircleNew/deleteCircle?id=" + strconv.FormatInt(circleID, 10)
	_, err := c.doBizAndDecode(ctx, token, "DeleteCircle", path, http.MethodGet, nil)
	if err != nil {
		return fmt.Errorf("DeleteCircle 失败: %w", err)
	}
	return nil
}

// AddCircleComment 给写实记录添加评论。
// POST /api/studentCircleNew/addCircleComment
// 请求体: {"circleId": circleID, "content": content}
func (c *Client) AddCircleComment(ctx context.Context, token string, circleID int64, content string) error {
	payload := map[string]any{
		"circleId": circleID,
		"content":  content,
	}
	_, err := c.doBizAndDecode(ctx, token, "AddCircleComment",
		"/api/studentCircleNew/addCircleComment", http.MethodPost, payload)
	if err != nil {
		return fmt.Errorf("AddCircleComment 失败: %w", err)
	}
	return nil
}

// SetCircleLike 点赞或取消点赞写实记录。
// GET /api/studentCircleNew/setCircleLikeById?circleId=
// 服务端自动切换点赞/取消状态。
func (c *Client) SetCircleLike(ctx context.Context, token string, circleID int64) error {
	path := "/api/studentCircleNew/setCircleLikeById?circleId=" + strconv.FormatInt(circleID, 10)
	_, err := c.doBizAndDecode(ctx, token, "SetCircleLike", path, http.MethodGet, nil)
	if err != nil {
		return fmt.Errorf("SetCircleLike 失败: %w", err)
	}
	return nil
}

// GetCircleTypes 获取指定维度下的写实类别。
// GET /api/studentCircleNew/getCircleType?dimensionId=&pid=
// pid 可为空字符串。
func (c *Client) GetCircleTypes(ctx context.Context, token string, dimensionID int64, pid string) ([]map[string]any, error) {
	path := "/api/studentCircleNew/getCircleType?dimensionId=" + strconv.FormatInt(dimensionID, 10) + "&pid=" + pid
	v, err := doBizGetDecode[[]map[string]any](c, ctx, token, "GetCircleTypes", path,
		func(resp types.UnifiedResponse) (*[]map[string]any, error) {
			data, err := types.DecodeDataList[map[string]any](resp)
			if err != nil {
				return nil, err
			}
			return &data, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return *v, nil
}

// GetCircleTasks 获取指定类别下的写实任务。
// GET /api/studentCircleNew/getCircleTask?typeId=
func (c *Client) GetCircleTasks(ctx context.Context, token string, typeID int64) ([]map[string]any, error) {
	path := "/api/studentCircleNew/getCircleTask?typeId=" + strconv.FormatInt(typeID, 10)
	v, err := doBizGetDecode[[]map[string]any](c, ctx, token, "GetCircleTasks", path,
		func(resp types.UnifiedResponse) (*[]map[string]any, error) {
			data, err := types.DecodeDataList[map[string]any](resp)
			if err != nil {
				return nil, err
			}
			return &data, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return *v, nil
}

// GetCircleImages 获取当前用户上传的写实图片列表。
// GET /api/studentCircleNew/getCircleImg
func (c *Client) GetCircleImages(ctx context.Context, token string) ([]map[string]any, error) {
	v, err := doBizGetDecode[[]map[string]any](c, ctx, token, "GetCircleImages", "/api/studentCircleNew/getCircleImg",
		func(resp types.UnifiedResponse) (*[]map[string]any, error) {
			data, err := types.DecodeDataList[map[string]any](resp)
			if err != nil {
				return nil, err
			}
			return &data, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return *v, nil
}

// GetDictList 获取系统字典列表。
// GET /api/common/sys/dict/list?cateCode=
func (c *Client) GetDictList(ctx context.Context, token string, cateCode int) ([]map[string]any, error) {
	path := "/api/common/sys/dict/list?cateCode=" + strconv.Itoa(cateCode)
	v, err := doBizGetDecode[[]map[string]any](c, ctx, token, "GetDictList", path,
		func(resp types.UnifiedResponse) (*[]map[string]any, error) {
			data, err := types.DecodeDataList[map[string]any](resp)
			if err != nil {
				return nil, err
			}
			return &data, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return *v, nil
}
