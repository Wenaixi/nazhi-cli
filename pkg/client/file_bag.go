package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// GetTermList 查询学期列表。
//
// 对应前端：filesBox.vue / presentationBox.vue → getTermInfo
// API: GET /api/teacher/school/use/pageQueryTermBySchoolId?pageNo={pageNo}&pageSize={pageSize}
func (c *Client) GetTermList(ctx context.Context, token string) ([]types.TermInfo, error) {
	path := "/api/teacher/school/use/pageQueryTermBySchoolId?pageNo=1&pageSize=100"
	resp, err := c.doBizAndDecode(ctx, token, "GetTermList", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetTermList 失败: %w", err)
	}

	termList, err := types.DecodeDataList[types.TermInfo](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetTermList 解析学期列表失败: %w", err)
	}

	return termList, nil
}

// GetStudentInfoForTerm 获取学生档案信息。
//
// 对应前端：filesBox.vue / presentationBox.vue → gotoReport / gotoFileBag
// API: GET /api/teacher/school/studentReport/getStudentInfoForTermId?termId={termId}
func (c *Client) GetStudentInfoForTerm(ctx context.Context, token string, termID int64) (map[string]any, error) {
	path := "/api/teacher/school/studentReport/getStudentInfoForTermId?termId=" + strconv.FormatInt(termID, 10)
	resp, err := c.doBizAndDecode(ctx, token, "GetStudentInfoForTerm", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetStudentInfoForTerm 失败: %w", err)
	}

	info, err := types.DecodeReturnData[map[string]any](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetStudentInfoForTerm 解析失败: %w", err)
	}

	return *info, nil
}
