package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// GetExamInitInfo 初始化学期/考试/课程数据。
//
// 对应前端：Realismtop.vue → getTermData
// API: GET /api/studentExamNew/getInitInfo?termId={termId}
func (c *Client) GetExamInitInfo(ctx context.Context, token string, termID int64) (*types.ExamInitInfo, error) {
	path := "/api/studentExamNew/getInitInfo?termId=" + strconv.FormatInt(termID, 10)
	resp, err := c.doBizAndDecode(ctx, token, "GetExamInitInfo", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetExamInitInfo 失败: %w", err)
	}

	info, err := types.DecodeReturnData[types.ExamInitInfo](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetExamInitInfo 解析失败: %w", err)
	}

	return info, nil
}

// QueryStudentExam 查询学生成绩。
//
// 对应前端：Realismbottom.vue → searchData
// API: POST /api/studentExamNew/queryStudentExam
func (c *Client) QueryStudentExam(ctx context.Context, token string, termID int64, courseList, examList []map[string]any) ([]types.ExamResult, error) {
	payload := map[string]any{
		"termId":     termID,
		"courseList": courseList,
		"examList":   examList,
	}
	resp, err := c.doBizAndDecode(ctx, token, "QueryStudentExam", "/api/studentExamNew/queryStudentExam", http.MethodPost, payload)
	if err != nil {
		return nil, fmt.Errorf("QueryStudentExam 失败: %w", err)
	}

	results, err := types.DecodeDataList[types.ExamResult](*resp)
	if err != nil {
		return nil, fmt.Errorf("QueryStudentExam 解析成绩列表失败: %w", err)
	}

	return results, nil
}
