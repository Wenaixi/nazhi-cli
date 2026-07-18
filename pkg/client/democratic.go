package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// GetDemocraticActivities 分页查询民主评价活动。
//
// 对应前端：DemocraticActivity.vue → queryActivity
// API: GET /api/studentDemocraticNew/getActivity?pageNo={pageNo}&pageSize={pageSize}
func (c *Client) GetDemocraticActivities(ctx context.Context, token string, pageNo, pageSize int) ([]types.DemocraticActivity, *types.PageBean, error) {
	path := "/api/studentDemocraticNew/getActivity?pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(pageSize)
	resp, err := c.doBizAndDecode(ctx, token, "GetDemocraticActivities", path, http.MethodGet, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("GetDemocraticActivities 失败: %w", err)
	}

	pb, err := types.DecodePageBean(*resp)
	if err != nil {
		return nil, nil, fmt.Errorf("GetDemocraticActivities 解析分页信息失败: %w", err)
	}

	activities, err := types.DecodeDataList[types.DemocraticActivity](*resp)
	if err != nil {
		return nil, nil, fmt.Errorf("GetDemocraticActivities 解析活动列表失败: %w", err)
	}

	return activities, pb, nil
}

// GetDemocraticActivityByID 获取单个民主评价活动信息。
//
// 对应前端：DemocraticBox.vue → getActivityInfo
// API: GET /api/studentDemocraticNew/getDemocraticActivityById?id={id}
func (c *Client) GetDemocraticActivityByID(ctx context.Context, token string, activityID int64) (*types.DemocraticActivity, error) {
	path := "/api/studentDemocraticNew/getDemocraticActivityById?id=" + strconv.FormatInt(activityID, 10)
	resp, err := c.doBizAndDecode(ctx, token, "GetDemocraticActivityByID", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetDemocraticActivityByID 失败: %w", err)
	}

	activity, err := types.DecodeReturnData[types.DemocraticActivity](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetDemocraticActivityByID 解析失败: %w", err)
	}

	return activity, nil
}

// GetSelfEvaluationData 获取自评数据。
//
// 对应前端：DemocraticBox.vue → getSelfEvaluation
// API: GET /api/studentDemocraticNew/getSelfEvaluation?activityId={activityId}&subPlanId={subPlanId}
func (c *Client) GetSelfEvaluationData(ctx context.Context, token string, activityID, subPlanID int64) ([]types.SelfEvaluationItem, error) {
	path := "/api/studentDemocraticNew/getSelfEvaluation?activityId=" + strconv.FormatInt(activityID, 10) + "&subPlanId=" + strconv.FormatInt(subPlanID, 10)
	resp, err := c.doBizAndDecode(ctx, token, "GetSelfEvaluationData", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetSelfEvaluationData 失败: %w", err)
	}

	items, err := types.DecodeDataList[types.SelfEvaluationItem](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetSelfEvaluationData 解析自评数据失败: %w", err)
	}

	return items, nil
}

// GetMutualPersonInfo 获取互评人员信息。
//
// 对应前端：DemocraticBox.vue → getMutualPersonInfo
// API: GET /api/studentDemocraticNew/getMutualPersonInfo?activityId={activityId}
func (c *Client) GetMutualPersonInfo(ctx context.Context, token string, activityID int64) (*types.MutualPersonInfo, error) {
	path := "/api/studentDemocraticNew/getMutualPersonInfo?activityId=" + strconv.FormatInt(activityID, 10)
	resp, err := c.doBizAndDecode(ctx, token, "GetMutualPersonInfo", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetMutualPersonInfo 失败: %w", err)
	}

	info, err := types.DecodeReturnData[types.MutualPersonInfo](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetMutualPersonInfo 解析失败: %w", err)
	}

	return info, nil
}

// GetDemocraticResult 获取评价结果汇总。
//
// 对应前端：DemocraticBox.vue → getDemocraticResult
// API: GET /api/studentDemocraticNew/getDemocraticResult?activityId={activityId}
func (c *Client) GetDemocraticResult(ctx context.Context, token string, activityID int64) (*types.DemocraticResult, error) {
	path := "/api/studentDemocraticNew/getDemocraticResult?activityId=" + strconv.FormatInt(activityID, 10)
	resp, err := c.doBizAndDecode(ctx, token, "GetDemocraticResult", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetDemocraticResult 失败: %w", err)
	}

	result, err := types.DecodeReturnData[types.DemocraticResult](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetDemocraticResult 解析失败: %w", err)
	}

	return result, nil
}

// GetMutualEvaluationDetail 获取互评详情。
//
// 对应前端：DemocraticNR.vue → getMutualEvaluationDetail
// API: POST /api/studentDemocraticNew/getMutualEvaluationDetail?activityId={activityId}&subPlanId={subPlanId}
func (c *Client) GetMutualEvaluationDetail(ctx context.Context, token string, activityID, subPlanID int64) ([]types.MutualEvaluation, error) {
	path := "/api/studentDemocraticNew/getMutualEvaluationDetail?activityId=" + strconv.FormatInt(activityID, 10) + "&subPlanId=" + strconv.FormatInt(subPlanID, 10)
	resp, err := c.doBizAndDecode(ctx, token, "GetMutualEvaluationDetail", path, http.MethodPost, nil)
	if err != nil {
		return nil, fmt.Errorf("GetMutualEvaluationDetail 失败: %w", err)
	}

	details, err := types.DecodeDataList[types.MutualEvaluation](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetMutualEvaluationDetail 解析互评详情失败: %w", err)
	}

	return details, nil
}

// AddOrUpdateSelfEvaluation 提交/更新自评。
//
// 对应前端：DemocraticBox.vue → submit
// API: POST /api/studentDemocraticNew/addOrUpdateSelfEvaluation
func (c *Client) AddOrUpdateSelfEvaluation(ctx context.Context, token string, evaluations []types.SelfEvaluationItem) error {
	_, err := c.doBizAndDecode(ctx, token, "AddOrUpdateSelfEvaluation", "/api/studentDemocraticNew/addOrUpdateSelfEvaluation", http.MethodPost, evaluations)
	if err != nil {
		return fmt.Errorf("AddOrUpdateSelfEvaluation 失败: %w", err)
	}
	return nil
}

// AddOrUpdateMutualEvaluation 提交/更新互评。
//
// 对应前端：DemocraticNR.vue → submit
// API: POST /api/studentDemocraticNew/addOrUpdateMutualEvaluation
func (c *Client) AddOrUpdateMutualEvaluation(ctx context.Context, token string, evaluations []types.MutualEvaluation) error {
	_, err := c.doBizAndDecode(ctx, token, "AddOrUpdateMutualEvaluation", "/api/studentDemocraticNew/addOrUpdateMutualEvaluation", http.MethodPost, evaluations)
	if err != nil {
		return fmt.Errorf("AddOrUpdateMutualEvaluation 失败: %w", err)
	}
	return nil
}
