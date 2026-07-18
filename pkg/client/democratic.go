package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// GetDemocraticActivities 获取民主评价活动列表。
// GET /api/studentDemocraticNew/getActivity
func (c *Client) GetDemocraticActivities(ctx context.Context, token string) ([]types.DemocraticActivity, error) {
	resp, err := c.doBizAndDecode(ctx, token, "GetDemocraticActivities",
		"/api/studentDemocraticNew/getActivity", http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetDemocraticActivities 失败: %w", err)
	}
	return types.DecodeDataList[types.DemocraticActivity](*resp)
}

// GetDemocraticActivityByID 获取单个民主评价活动详情。
// GET /api/studentDemocraticNew/getDemocraticActivityById?id=
func (c *Client) GetDemocraticActivityByID(ctx context.Context, token string, id int64) (*types.DemocraticActivity, error) {
	path := "/api/studentDemocraticNew/getDemocraticActivityById?id=" + strconv.FormatInt(id, 10)
	return doBizGetDecode[types.DemocraticActivity](c, ctx, token, "GetDemocraticActivityByID", path,
		types.DecodeReturnData[types.DemocraticActivity],
	)
}

// GetSelfEvaluation 获取自评数据。
// GET /api/studentDemocraticNew/getSelfEvaluation?activityId=
func (c *Client) GetSelfEvaluation(ctx context.Context, token string, activityID int64) ([]types.SelfEvaluationItem, error) {
	path := "/api/studentDemocraticNew/getSelfEvaluation?activityId=" + strconv.FormatInt(activityID, 10)
	resp, err := c.doBizAndDecode(ctx, token, "GetSelfEvaluation", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetSelfEvaluation 失败: %w", err)
	}
	return types.DecodeDataList[types.SelfEvaluationItem](*resp)
}

// GetMutualPersonInfo 获取互评人员信息。
// GET /api/studentDemocraticNew/getMutualPersonInfo?activityId=
func (c *Client) GetMutualPersonInfo(ctx context.Context, token string, activityID int64) (*types.MutualPersonInfo, error) {
	path := "/api/studentDemocraticNew/getMutualPersonInfo?activityId=" + strconv.FormatInt(activityID, 10)
	return doBizGetDecode[types.MutualPersonInfo](c, ctx, token, "GetMutualPersonInfo", path,
		types.DecodeReturnData[types.MutualPersonInfo],
	)
}

// GetDemocraticResult 获取民主评价结果。
// GET /api/studentDemocraticNew/getDemocraticResult?activityId=
func (c *Client) GetDemocraticResult(ctx context.Context, token string, activityID int64) (*types.DemocraticResult, error) {
	path := "/api/studentDemocraticNew/getDemocraticResult?activityId=" + strconv.FormatInt(activityID, 10)
	return doBizGetDecode[types.DemocraticResult](c, ctx, token, "GetDemocraticResult", path,
		types.DecodeReturnData[types.DemocraticResult],
	)
}

// GetMutualEvaluationDetail 获取互评详情。
// POST /api/studentDemocraticNew/getMutualEvaluationDetail?activityId=
func (c *Client) GetMutualEvaluationDetail(ctx context.Context, token string, activityID int64, studentID int64) ([]types.MutualEvaluation, error) {
	path := "/api/studentDemocraticNew/getMutualEvaluationDetail?activityId=" + strconv.FormatInt(activityID, 10)
	payload := map[string]any{"studentId": studentID}
	resp, err := c.doBizAndDecode(ctx, token, "GetMutualEvaluationDetail", path, http.MethodPost, payload)
	if err != nil {
		return nil, fmt.Errorf("GetMutualEvaluationDetail 失败: %w", err)
	}
	return types.DecodeDataList[types.MutualEvaluation](*resp)
}

// AddOrUpdateSelfEvaluation 提交或更新自评。
// POST /api/studentDemocraticNew/addOrUpdateSelfEvaluation
func (c *Client) AddOrUpdateSelfEvaluation(ctx context.Context, token string, activityID int64, evaluationResult string) error {
	payload := map[string]any{
		"activityId":       activityID,
		"evaluationResult": evaluationResult,
	}
	_, err := c.doBizAndDecode(ctx, token, "AddOrUpdateSelfEvaluation",
		"/api/studentDemocraticNew/addOrUpdateSelfEvaluation", http.MethodPost, payload)
	if err != nil {
		return fmt.Errorf("AddOrUpdateSelfEvaluation 失败: %w", err)
	}
	return nil
}

// AddOrUpdateMutualEvaluation 提交或更新互评。
// POST /api/studentDemocraticNew/addOrUpdateMutualEvaluation
func (c *Client) AddOrUpdateMutualEvaluation(ctx context.Context, token string, activityID int64, studentID int64, result string) error {
	payload := map[string]any{
		"activityId":       activityID,
		"studentId":        studentID,
		"evaluationResult": result,
	}
	_, err := c.doBizAndDecode(ctx, token, "AddOrUpdateMutualEvaluation",
		"/api/studentDemocraticNew/addOrUpdateMutualEvaluation", http.MethodPost, payload)
	if err != nil {
		return fmt.Errorf("AddOrUpdateMutualEvaluation 失败: %w", err)
	}
	return nil
}
