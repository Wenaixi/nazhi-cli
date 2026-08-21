package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// DeleteCircle 删除一条写实记录。
// GET /api/studentCircleNew/deleteCircle?id=
func (c *Client) DeleteCircle(ctx context.Context, token string, circleID int64) error {
	path := "/api/studentCircleNew/deleteCircle?id=" + strconv.FormatInt(circleID, 10)
	return c.doBizVoid(ctx, token, "DeleteCircle", path, http.MethodGet, nil)
}

// AddCircleComment 给写实记录添加评论。
// POST /api/studentCircleNew/addCircleComment
// 请求体: {"circleId": circleID, "content": content}
//
// 前端在评论成功后执行 `commentList.unshift(response.data.returnData)`，
// 服务端 returnData 即新创建的 Comment 对象。为保留该语义，SDK 解析
// returnData 并返回；调用方可直接 unshift 到本地列表而无需重新查询。
// 若服务端未返回 returnData（空或解析失败）则返回 nil, nil 由调用方忽略即可。
func (c *Client) AddCircleComment(ctx context.Context, token string, circleID int64, content string) (*types.Comment, error) {
	payload := map[string]any{
		"circleId": circleID,
		"content":  content,
	}
	resp, err := c.doBizAndDecode(ctx, token, "AddCircleComment",
		"/api/studentCircleNew/addCircleComment", http.MethodPost, payload)
	if err != nil {
		return nil, err
	}
	comment, err := types.DecodeReturnData[types.Comment](*resp)
	if err != nil {
		return nil, err
	}
	return comment, nil
}

// SetCircleLike 点赞或取消点赞写实记录。
// GET /api/studentCircleNew/setCircleLikeById?circleId=
// 服务端自动切换点赞/取消状态。
func (c *Client) SetCircleLike(ctx context.Context, token string, circleID int64) error {
	path := "/api/studentCircleNew/setCircleLikeById?circleId=" + strconv.FormatInt(circleID, 10)
	return c.doBizVoid(ctx, token, "SetCircleLike", path, http.MethodGet, nil)
}

// GetCircleTypes 获取指定维度下的写实类别。
// GET /api/studentCircleNew/getCircleType?dimensionId=&pid=
// pid 为空时省略 &pid=，与普校前端（`?dimensionId=`+e.id）一致；
// 非空时经 url.QueryEscape，避免 &/= 等字符破坏查询串。
// 元洪附小专用接口 getCircleImgByDimensionId / getCircleTypeByDimensionId /
// getCircleStatisticsByTypeId 不实现（WARN 非 FAIL，见审计 06）。
func (c *Client) GetCircleTypes(ctx context.Context, token string, dimensionID int64, pid string) ([]map[string]any, error) {
	path := "/api/studentCircleNew/getCircleType?dimensionId=" + strconv.FormatInt(dimensionID, 10)
	if pid != "" {
		path += "&pid=" + url.QueryEscape(pid)
	}
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
// GET /api/studentCircleNew/getCircleImg?pageNo=&pageSize=
func (c *Client) GetCircleImages(ctx context.Context, token string, pageNo, pageSize int) ([]map[string]any, error) {
	path := "/api/studentCircleNew/getCircleImg?pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(pageSize)
	v, err := doBizGetDecode[[]map[string]any](c, ctx, token, "GetCircleImages", path,
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
