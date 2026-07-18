package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// GetExamInitInfo 获取成绩管理初始化数据（学期/考试/课程列表）。
// GET /api/studentExamNew/getInitInfo?termId=
func (c *Client) GetExamInitInfo(ctx context.Context, token string, termID int64) (*types.ExamInitInfo, error) {
	path := "/api/studentExamNew/getInitInfo?termId=" + strconv.FormatInt(termID, 10)
	return doBizGetDecode[types.ExamInitInfo](c, ctx, token, "GetExamInitInfo", path,
		types.DecodeDataMap[types.ExamInitInfo],
	)
}

// QueryStudentExam 查询学生成绩。
// POST /api/studentExamNew/queryStudentExam
func (c *Client) QueryStudentExam(ctx context.Context, token string, params types.QueryExamParams) ([]types.ExamResult, error) {
	resp, err := c.doBizAndDecode(ctx, token, "QueryStudentExam",
		"/api/studentExamNew/queryStudentExam", http.MethodPost, params)
	if err != nil {
		return nil, fmt.Errorf("QueryStudentExam 失败: %w", err)
	}
	return types.DecodeDataList[types.ExamResult](*resp)
}
