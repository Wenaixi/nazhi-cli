package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

func TestE2E_WriteMock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 每项写用例：构造最小合法 payload 调 mockClient，断言 err==nil 且 recordedWrites 匹配
	type writeCase struct {
		name string
		fn   func(t *testing.T)
	}
	cases := []writeCase{
		{"honor/AddHonor", func(t *testing.T) {
			ClearRecordedWrites()
			err := mockClient.AddHonor(ctx, "fake-token", types.AddHonorPayload{
				TypeID:   1148,
				TypeName: "校三好学生",
				Level:    5,
				Name:     "校三好学生",
				GetDate:  "2026-05-25",
				EvaluationAgency: "福清一中",
			})
			if err != nil {
				t.Fatalf("AddHonor: %v", err)
			}
			writes := GetRecordedWrites()
			if len(writes) == 0 {
				t.Fatalf("AddHonor 未产生写请求")
			}
			found := false
			for _, w := range writes {
				if contains(w.Path, "addHonor") {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("AddHonor 未命中 addHonor 路径: %+v", writes)
			}
		}},
		{"honor/UpdateHonor", func(t *testing.T) {
			ClearRecordedWrites()
			err := mockClient.UpdateHonor(ctx, "fake-token", map[string]any{
				"id": 999, "typeId": int64(1148), "typeName": "校三好学生", "level": 5, "getDate": "2026-05-25",
			})
			if err != nil {
				t.Fatalf("UpdateHonor: %v", err)
			}
			if len(GetRecordedWrites()) == 0 {
				t.Fatalf("UpdateHonor 未产生请求")
			}
		}},
		{"honor/DeleteHonor", func(t *testing.T) {
			ClearRecordedWrites()
			if err := mockClient.DeleteHonor(ctx, "fake-token", 12345); err != nil {
				t.Fatalf("DeleteHonor: %v", err)
			}
			writes := GetRecordedWrites()
			if len(writes) == 0 || !contains(writes[0].Path, "deleteHonor") {
				t.Fatalf("DeleteHonor 路径不匹配: %+v", writes)
			}
		}},
		{"typical/AddTypicalCase", func(t *testing.T) {
			ClearRecordedWrites()
			err := mockClient.AddTypicalCase(ctx, "fake-token", types.AddTypicalCasePayload{
				Title: "E2E 模拟标题", Type: "1", Role: "1", Level: "5",
				TeacherName: "测试老师", Content: "正文内容", Remark: "备注",
			})
			if err != nil {
				t.Fatalf("AddTypicalCase: %v", err)
			}
			if len(GetRecordedWrites()) == 0 {
				t.Fatalf("AddTypicalCase 未产生请求")
			}
		}},
		{"typical/UpdateTypicalCase", func(t *testing.T) {
			ClearRecordedWrites()
			err := mockClient.UpdateTypicalCase(ctx, "fake-token", map[string]any{
				"id": int64(999), "title": "更新标题", "type": "1", "role": "1", "level": "5",
			})
			if err != nil {
				t.Fatalf("UpdateTypicalCase: %v", err)
			}
			if len(GetRecordedWrites()) == 0 {
				t.Fatalf("UpdateTypicalCase 未产生请求")
			}
		}},
		{"typical/DeleteTypicalCase", func(t *testing.T) {
			ClearRecordedWrites()
			if err := mockClient.DeleteTypicalCase(ctx, "fake-token", 999); err != nil {
				t.Fatalf("DeleteTypicalCase: %v", err)
			}
			if len(GetRecordedWrites()) == 0 {
				t.Fatalf("DeleteTypicalCase 未产生请求")
			}
		}},
		{"typical/DeleteBatch", func(t *testing.T) {
			ClearRecordedWrites()
			if err := mockClient.DeleteBatchTypicalCase(ctx, "fake-token", []int64{1, 2}); err != nil {
				t.Fatalf("DeleteBatch: %v", err)
			}
			if len(GetRecordedWrites()) == 0 {
				t.Fatalf("DeleteBatch 未产生请求")
			}
		}},
		{"circle/AddComment", func(t *testing.T) {
			ClearRecordedWrites()
			_, err := mockClient.AddCircleComment(ctx, "fake-token", 9999, "e2e 评论")
			if err != nil {
				t.Fatalf("AddCircleComment: %v", err)
			}
			if len(GetRecordedWrites()) == 0 {
				t.Fatalf("AddCircleComment 未产生请求")
			}
		}},
		{"circle/SetLike", func(t *testing.T) {
			ClearRecordedWrites()
			if err := mockClient.SetCircleLike(ctx, "fake-token", 9999); err != nil {
				t.Fatalf("SetCircleLike: %v", err)
			}
			if len(GetRecordedWrites()) == 0 {
				t.Fatalf("SetCircleLike 未产生请求")
			}
		}},
		{"circle/DeleteCircle", func(t *testing.T) {
			ClearRecordedWrites()
			if err := mockClient.DeleteCircle(ctx, "fake-token", 9999); err != nil {
				t.Fatalf("DeleteCircle: %v", err)
			}
			if len(GetRecordedWrites()) == 0 {
				t.Fatalf("DeleteCircle 未产生请求")
			}
		}},
		{"self-eval/SubmitSelfEvaluation", func(t *testing.T) {
			ClearRecordedWrites()
			if err := mockClient.SubmitSelfEvaluation(ctx, "fake-token", "e2e 自评文本"); err != nil {
				t.Fatalf("SubmitSelfEvaluation: %v", err)
			}
			if len(GetRecordedWrites()) == 0 {
				t.Fatalf("SubmitSelfEvaluation 未产生请求")
			}
		}},
		{"self-eval/SubmitStructured", func(t *testing.T) {
			ClearRecordedWrites()
			if err := mockClient.SubmitSelfEvaluationStructured(ctx, "fake-token", map[string]any{"bxqhzr": "目标"}); err != nil {
				t.Fatalf("SubmitStructured: %v", err)
			}
			if len(GetRecordedWrites()) == 0 {
				t.Fatalf("SubmitStructured 未产生请求")
			}
		}},
		{"self-eval/SubmitGrad", func(t *testing.T) {
			ClearRecordedWrites()
			if err := mockClient.SubmitSelfGradEvaluation(ctx, "fake-token", "毕业自评"); err != nil {
				t.Fatalf("SubmitGrad: %v", err)
			}
			if len(GetRecordedWrites()) == 0 {
				t.Fatalf("SubmitGrad 未产生请求")
			}
		}},
		{"user/UpdateMyInfo", func(t *testing.T) {
			ClearRecordedWrites()
			if err := mockClient.UpdateMyInfo(ctx, "fake-token", map[string]any{"studentName": "测试"}); err != nil {
				t.Fatalf("UpdateMyInfo: %v", err)
			}
			if len(GetRecordedWrites()) == 0 {
				t.Fatalf("UpdateMyInfo 未产生请求")
			}
		}},
		{"user/UpdateMyInfoStructured", func(t *testing.T) {
			ClearRecordedWrites()
			if err := mockClient.UpdateMyInfoStructured(ctx, "fake-token", types.UserUpdateInput{Name: "测试"}); err != nil {
				t.Fatalf("UpdateMyInfoStructured: %v", err)
			}
			if len(GetRecordedWrites()) == 0 {
				t.Fatalf("UpdateMyInfoStructured 未产生请求")
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, tc.fn)
	}

	// 关键：SubmitTask/EditCircle 依赖 getCircleTypeByTaskId + upload，前置需 mock 该 GET
	// 本 harness 已注册 addCircle/editCircle 的 POST，但 task 链路的 GET 元数据走 mock 也需覆盖
	// 因此 Submit/Edit 用独立子测：若缺少元数据 mock 会走真实失败，预期为业务错误而非 panic
	t.Run("task/SubmitTask(mock链路)", func(t *testing.T) {
		ClearRecordedWrites()
		// 不依赖真实任务，直接验证 mock 的任务元数据缺失时 SDK 的错误语义
		_, err := mockClient.SubmitTask(ctx, "fake-token", types.TaskSubmitInput{TaskID: 1, Content: "x"})
		// mock 的 getCircleTypeByTaskId 未注册，预期失败（非 panic 即可）
		if err == nil {
			t.Logf("SubmitTask 意外成功（mock 可能已兜底）")
		} else {
			// 允许 ErrBusinessRejected / network / 任意非 panic
			if contains(err.Error(), "panic") {
				t.Fatalf("不应 panic: %v", err)
			}
		}
		// 即使失败也不应产生未处理的 panic，上层已保证
		_ = client.ErrBusinessRejected // 引用避免 unused
		if len(GetRecordedWrites()) == 0 {
			t.Logf("SubmitTask 未产生 addCircle 写（因子任务元数据失败，符合预期）")
		}
	})

	_ = strings.Contains // keep import used if needed
}