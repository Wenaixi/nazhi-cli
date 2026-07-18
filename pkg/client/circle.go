package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// DeleteCircle 删除一条写实记录。
//
// 对应前端：managementRightBottom.vue → open2(circleId)
// API: GET /api/studentCircleNew/deleteCircle?id={id}
func (c *Client) DeleteCircle(ctx context.Context, token string, circleID int64) error {
	path := "/api/studentCircleNew/deleteCircle?id=" + strconv.FormatInt(circleID, 10)
	_, err := c.doBizAndDecode(ctx, token, "DeleteCircle", path, http.MethodGet, nil)
	if err != nil {
		return fmt.Errorf("DeleteCircle 失败: %w", err)
	}
	return nil
}

// AddCircleComment 添加写实评论。
//
// 对应前端：managementRightBottom.vue / mainMidSearch.vue → addComment
// API: POST /api/studentCircleNew/addCircleComment
func (c *Client) AddCircleComment(ctx context.Context, token string, circleID int64, content string) error {
	payload := map[string]string{
		"circleId": strconv.FormatInt(circleID, 10),
		"content":  content,
	}
	_, err := c.doBizAndDecode(ctx, token, "AddCircleComment", "/api/studentCircleNew/addCircleComment", http.MethodPost, payload)
	if err != nil {
		return fmt.Errorf("AddCircleComment 失败: %w", err)
	}
	return nil
}

// SetCircleLike 点赞/取消点赞写实记录。
//
// 对应前端：managementRightBottom.vue → likeIt
// API: GET /api/studentCircleNew/setCircleLikeById?circleId={id}
func (c *Client) SetCircleLike(ctx context.Context, token string, circleID int64) error {
	path := "/api/studentCircleNew/setCircleLikeById?circleId=" + strconv.FormatInt(circleID, 10)
	_, err := c.doBizAndDecode(ctx, token, "SetCircleLike", path, http.MethodGet, nil)
	if err != nil {
		return fmt.Errorf("SetCircleLike 失败: %w", err)
	}
	return nil
}

// GetCircleImages 查询上传的图片列表。
//
// 对应前端：managementLeftBottom.vue → queryCircleImg
// API: GET /api/studentCircleNew/getCircleImg?pageNo={pageNo}&pageSize={pageSize}
func (c *Client) GetCircleImages(ctx context.Context, token string, pageNo, pageSize int) ([]types.CircleImage, *types.PageBean, error) {
	path := "/api/studentCircleNew/getCircleImg?pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(pageSize)
	resp, err := c.doBizAndDecode(ctx, token, "GetCircleImages", path, http.MethodGet, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("GetCircleImages 失败: %w", err)
	}

	pb, err := types.DecodePageBean(*resp)
	if err != nil {
		return nil, nil, fmt.Errorf("GetCircleImages 解析分页信息失败: %w", err)
	}

	images, err := types.DecodeDataList[types.CircleImage](*resp)
	if err != nil {
		return nil, nil, fmt.Errorf("GetCircleImages 解析图片列表失败: %w", err)
	}

	return images, pb, nil
}

// GetCircleTasks 根据类别获取任务列表。
//
// 对应前端：managementRightTop.vue → changeCircleTask
// API: GET /api/studentCircleNew/getCircleTask?typeId={typeId}
func (c *Client) GetCircleTasks(ctx context.Context, token string, typeID int64) ([]types.Task, error) {
	path := "/api/studentCircleNew/getCircleTask?typeId=" + strconv.FormatInt(typeID, 10)
	resp, err := c.doBizAndDecode(ctx, token, "GetCircleTasks", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetCircleTasks 失败: %w", err)
	}

	tasks, err := types.DecodeDataList[types.Task](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetCircleTasks 解析任务列表失败: %w", err)
	}

	return tasks, nil
}

// GetCircleTypes 根据维度获取类别列表。
//
// 对应前端：managementRightTop.vue → changeCircleType
// API: GET /api/studentCircleNew/getCircleType?dimensionId={dimensionId}
func (c *Client) GetCircleTypes(ctx context.Context, token string, dimensionID int64) ([]types.Dimension, error) {
	path := "/api/studentCircleNew/getCircleType?dimensionId=" + strconv.FormatInt(dimensionID, 10)
	resp, err := c.doBizAndDecode(ctx, token, "GetCircleTypes", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetCircleTypes 失败: %w", err)
	}

	dims, err := types.DecodeDataList[types.Dimension](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetCircleTypes 解析类别列表失败: %w", err)
	}

	return dims, nil
}

// GetDimensionsBySchool 获取学校维度配置。
//
// 对应前端：managementRightTop.vue → queryDimension
// API: GET /api/teacher/circle/circleType/queryDimensionBySchoolIdAndStateType?stateType=0
func (c *Client) GetDimensionsBySchool(ctx context.Context, token string) ([]types.Dimension, error) {
	path := "/api/teacher/circle/circleType/queryDimensionBySchoolIdAndStateType?stateType=0"
	resp, err := c.doBizAndDecode(ctx, token, "GetDimensionsBySchool", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetDimensionsBySchool 失败: %w", err)
	}

	dims, err := types.DecodeDataList[types.Dimension](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetDimensionsBySchool 解析维度列表失败: %w", err)
	}

	return dims, nil
}

// GetDictList 获取等级字典。
//
// 对应前端：managementRightTop.vue / managementRightBottom.vue → getLevel
// API: GET /api/common/sys/dict/list?cateCode=23
func (c *Client) GetDictList(ctx context.Context, token string, cateCode int) ([]types.HonorSelectOption, error) {
	path := "/api/common/sys/dict/list?cateCode=" + strconv.Itoa(cateCode)
	resp, err := c.doBizAndDecode(ctx, token, "GetDictList", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetDictList 失败: %w", err)
	}

	opts, err := types.DecodeDataList[types.HonorSelectOption](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetDictList 解析字典列表失败: %w", err)
	}

	return opts, nil
}
