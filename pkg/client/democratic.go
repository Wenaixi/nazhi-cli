package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// GetDemocraticActivities 获取民主评价活动列表（分页）。
// GET /api/studentDemocraticNew/getActivity?pageNo=&pageSize=
func (c *Client) GetDemocraticActivities(ctx context.Context, token string, pageNo, pageSize int) (*types.DemocraticActivityListResult, error) {
	path := "/api/studentDemocraticNew/getActivity?pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(pageSize)
	resp, err := c.doBizAndDecode(ctx, token, "GetDemocraticActivities", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetDemocraticActivities 失败: %w", err)
	}
	pb, err := types.DecodePageBean(*resp)
	if err != nil {
		return nil, fmt.Errorf("GetDemocraticActivities 解析分页信息失败: %w", err)
	}
	records, err := types.DecodeDataList[types.DemocraticActivity](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetDemocraticActivities 解析活动列表失败: %w", err)
	}
	return &types.DemocraticActivityListResult{Records: records, Page: pb}, nil
}

// GetDemocraticActivityByID 获取单个民主评价活动详情。
// GET /api/studentDemocraticNew/getDemocraticActivityById?id=
//
// 注意：前端实际从 dataMap 读取 snake_case 字段（如 end_date），
// 而 DemocraticActivity 使用 camelCase（如 endDate）。
// 因此本方法返回原始 JSON，让调用方按需解码。
func (c *Client) GetDemocraticActivityByID(ctx context.Context, token string, id int64) (json.RawMessage, error) {
	path := "/api/studentDemocraticNew/getDemocraticActivityById?id=" + strconv.FormatInt(id, 10)
	resp, err := c.doBizAndDecode(ctx, token, "GetDemocraticActivityByID", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetDemocraticActivityByID 失败: %w", err)
	}
	// 前端从 dataMap 读取，优先 dataMap
	if resp.DataMap != nil {
		return *resp.DataMap, nil
	}
	if resp.ReturnData != nil {
		return *resp.ReturnData, nil
	}
	return nil, nil
}

// GetSelfEvaluation 获取自评数据。
// GET /api/studentDemocraticNew/getSelfEvaluation?activityId=&subPlanId=
func (c *Client) GetSelfEvaluation(ctx context.Context, token string, activityID, subPlanID int64) ([]types.SelfEvaluationItem, error) {
	path := "/api/studentDemocraticNew/getSelfEvaluation?activityId=" + strconv.FormatInt(activityID, 10) +
		"&subPlanId=" + strconv.FormatInt(subPlanID, 10)
	resp, err := c.doBizAndDecode(ctx, token, "GetSelfEvaluation", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetSelfEvaluation 失败: %w", err)
	}
	return types.DecodeDataList[types.SelfEvaluationItem](*resp)
}

// GetMutualPersonInfo 获取互评人员信息。
// GET /api/studentDemocraticNew/getMutualPersonInfo?activityId=
//
// 该接口的响应分布在两个顶级字段：
//   - dataMap: { ifMutualPerson, evaluatedNumbers, notEvaluatedNumbers }
//   - dataList: 班级学生列表（[]ClassStudent）
//
// 本方法手动拼装两个来源为一个 MutualPersonInfo。
func (c *Client) GetMutualPersonInfo(ctx context.Context, token string, activityID int64) (*types.MutualPersonInfo, error) {
	path := "/api/studentDemocraticNew/getMutualPersonInfo?activityId=" + strconv.FormatInt(activityID, 10)
	resp, err := c.doBizAndDecode(ctx, token, "GetMutualPersonInfo", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetMutualPersonInfo 失败: %w", err)
	}

	info := &types.MutualPersonInfo{}

	// 从 dataMap 解析元数据
	if resp.DataMap != nil {
		var meta map[string]any
		if err := json.Unmarshal(*resp.DataMap, &meta); err == nil {
			if v, ok := meta["ifMutualPerson"].(bool); ok {
				info.IfMutualPerson = v
			}
			if v, ok := meta["evaluatedNumbers"].(float64); ok {
				info.EvaluatedNumbers = int(v)
			}
			if v, ok := meta["notEvaluatedNumbers"].(float64); ok {
				info.NotEvaluatedNumbers = int(v)
			}
		}
	}

	// 从 dataList 解析班级学生列表
	if resp.DataList != nil {
		students, err := types.DecodeDataList[types.ClassStudent](*resp)
		if err == nil {
			info.ClassStudentList = students
		}
	}

	return info, nil
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
// POST /api/studentDemocraticNew/getMutualEvaluationDetail?activityId=&subPlanId=
//
// 请求体是 JSON 数组 [{student_id, student_name}, ...]，
// 响应是 dataList 数组，每项含 {student_name, student_id, data: [...]}。
func (c *Client) GetMutualEvaluationDetail(ctx context.Context, token string, activityID, subPlanID int64, students []map[string]any) ([]map[string]any, error) {
	path := "/api/studentDemocraticNew/getMutualEvaluationDetail?activityId=" + strconv.FormatInt(activityID, 10) +
		"&subPlanId=" + strconv.FormatInt(subPlanID, 10)

	resp, err := c.doBizAndDecode(ctx, token, "GetMutualEvaluationDetail", path, http.MethodPost, students)
	if err != nil {
		return nil, fmt.Errorf("GetMutualEvaluationDetail 失败: %w", err)
	}

	records, err := types.DecodeDataList[map[string]any](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetMutualEvaluationDetail 解析返回数据失败: %w", err)
	}
	return records, nil
}

// AddOrUpdateSelfEvaluation 提交或更新自评。
// POST /api/studentDemocraticNew/addOrUpdateSelfEvaluation
//
// 前端实际发送 JSON 数组，每个元素包含评价项细节。
// items 参数为 []SelfEvaluationInput。
func (c *Client) AddOrUpdateSelfEvaluation(ctx context.Context, token string, items []types.SelfEvaluationInput) error {
	_, err := c.doBizAndDecode(ctx, token, "AddOrUpdateSelfEvaluation",
		"/api/studentDemocraticNew/addOrUpdateSelfEvaluation", http.MethodPost, items)
	if err != nil {
		return fmt.Errorf("AddOrUpdateSelfEvaluation 失败: %w", err)
	}
	return nil
}

// AddOrUpdateMutualEvaluation 提交或更新互评。
// POST /api/studentDemocraticNew/addOrUpdateMutualEvaluation
//
// 前端实际发送嵌套 JSON 数组：
// [{student_id, student_name, data: [{activityId, quotaId, evaluationResult, ...}]}]
func (c *Client) AddOrUpdateMutualEvaluation(ctx context.Context, token string, items []types.MutualEvaluationInput) error {
	_, err := c.doBizAndDecode(ctx, token, "AddOrUpdateMutualEvaluation",
		"/api/studentDemocraticNew/addOrUpdateMutualEvaluation", http.MethodPost, items)
	if err != nil {
		return fmt.Errorf("AddOrUpdateMutualEvaluation 失败: %w", err)
	}
	return nil
}
