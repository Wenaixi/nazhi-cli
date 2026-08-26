package client

import (
	"errors"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestBuildTaskPayload_RemarkImageRequiredExhaustive 锁定 task.go:320-325 的
// 「任务备注含图片关键词 + pictureList 为空 → ErrInvalidPayload」校验分支。
// 该分支为 SDK 单方面发明的校验（前端 managementRightBottom.vue remark 仅展示，
// 无此校验），必须有测试守住以防重构误删。
func TestBuildTaskPayload_RemarkImageRequiredExhaustive(t *testing.T) {
	cases := []struct {
		name        string
		remark      string
		pictureList []int64
		wantErr     bool
		errSubstr   string
	}{
		// 正向：含关键词 + 无图 → 必须拒
		{"照片+无图", "请上传活动照片", nil, true, "该任务要求上传图片或附件"},
		{"图片+无图", "活动需附图片", nil, true, "该任务要求上传图片或附件"},
		{"PDF+无写大写", "Please attach PDF", nil, true, "该任务要求上传图片或附件"},
		{"PDF+无写小写", "pdf version required", nil, true, "该任务要求上传图片或附件"},

		// 正向：含关键词 + 有图 → 放行
		{"照片+有图", "请上传活动照片", []int64{1001}, false, ""},
		{"图片+有图", "活动需附图片", []int64{1001, 1002}, false, ""},
		{"PDF+有图", "PDF required", []int64{1001}, false, ""},

		// 负向：无关键词 + 无图 → 放行
		{"普通备注+无图", "普通备注", nil, false, ""},
		{"空备注+无图", "", nil, false, ""},
		{"空切片+无图", "普通备注", []int64{}, false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cap types.TaskAddCirclePayload
			// remark 走 metaEx 的 remark 字段（任务元数据），pictureList 走 TaskSubmitInput
			srv := mockExhaustive(t, metaEx(t, 5, 2, tc.remark), &cap)
			defer srv.Close()
			c, _ := New(WithBaseURL(srv.URL), WithSSOBase(srv.URL), WithTimeout(5*1000*1000*1000))
			defer c.Close()

			_, err := c.SubmitTask(t.Context(), "tok", types.TaskSubmitInput{
				TaskID:     1001,
				Content:    "c",
				ImageIDs:   tc.pictureList,
				Name:       "x",
				HostName:   "y",
				ObtainTime: "2026-01-01",
				Rank:       "1",
				Level:      "5",
			})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want ErrInvalidPayload got nil; payload=%+v", cap)
				}
				if !errors.Is(err, ErrInvalidPayload) {
					t.Fatalf("want ErrInvalidPayload got %v", err)
				}
				if !strings.Contains(err.Error(), tc.errSubstr) {
					t.Fatalf("want err contains %q got %q", tc.errSubstr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("want success got %v", err)
			}
		})
	}
}
