package e2e

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

func TestE2E_ReadLive(t *testing.T) {
	requireLive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	type readCase struct {
		name string
		fn   func(t *testing.T)
	}
	cases := []readCase{
		{"whoami/GetMyInfo", func(t *testing.T) {
			info, err := liveClient.GetMyInfo(ctx, liveToken)
			if isSkipableLiveErr(err) {
				t.Skipf("skip live (transient): %v", err)
			}
			if err != nil {
				t.Fatalf("GetMyInfo: %v", err)
			}
			if info == nil || info.Name == "" || info.StudentNumber == "" {
				t.Fatalf("GetMyInfo 空数据: %+v", info)
			}
			t.Logf("whoami: %s / %s / %s", info.Name, info.SchoolName, info.ClassName)
		}},
		{"session/ActivateSession", func(t *testing.T) {
			info, err := liveClient.ActivateSession(ctx, liveToken)
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("ActivateSession: %v", err)
			}
			if info == nil {
				t.Fatalf("ActivateSession 返回 nil")
			}
		}},
		{"task/FetchTasks", func(t *testing.T) {
			tasks, err := liveClient.FetchTasks(ctx, liveToken)
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("FetchTasks: %v", err)
			}
			t.Logf("FetchTasks: %d tasks", len(tasks))
		}},
		{"task/GetDimensions", func(t *testing.T) {
			dims, err := liveClient.GetDimensions(ctx, liveToken)
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("GetDimensions: %v", err)
			}
			if len(dims) == 0 {
				t.Fatalf("GetDimensions 空")
			}
		}},
		{"task/GetCircleTypeByTaskID(18160)", func(t *testing.T) {
			info, err := liveClient.GetCircleTypeByTaskID(ctx, liveToken, 18160)
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("GetCircleTypeByTaskID: %v", err)
			}
			if info == nil {
				t.Fatalf("GetCircleTypeByTaskID 返回 nil")
			}
		}},
		{"task/GetSubmittedCircles", func(t *testing.T) {
			records, err := liveClient.GetSubmittedCircles(ctx, liveToken, "")
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("GetSubmittedCircles: %v", err)
			}
			t.Logf("submitted: %d", len(records))
		}},
		{"task/GetTeacherCircles", func(t *testing.T) {
			records, err := liveClient.GetTeacherCircles(ctx, liveToken, "")
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("GetTeacherCircles: %v", err)
			}
			t.Logf("teacher: %d", len(records))
		}},
		{"task/GetWithdrawnCircles", func(t *testing.T) {
			records, err := liveClient.GetWithdrawnCircles(ctx, liveToken, "")
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("GetWithdrawnCircles: %v", err)
			}
			t.Logf("withdrawn: %d", len(records))
		}},
		{"task/GetPublicCircles", func(t *testing.T) {
			records, err := liveClient.GetPublicCircles(ctx, liveToken, "")
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("GetPublicCircles: %v", err)
			}
			t.Logf("public: %d", len(records))
		}},
		{"circle/GetCircleTypes(dim=13)", func(t *testing.T) {
			types, err := liveClient.GetCircleTypes(ctx, liveToken, 13, "")
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("GetCircleTypes: %v", err)
			}
			if len(types) == 0 {
				t.Fatalf("GetCircleTypes 空")
			}
		}},
		{"circle/GetCircleTasks(type=3694)", func(t *testing.T) {
			tasks, err := liveClient.GetCircleTasks(ctx, liveToken, 3694)
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("GetCircleTasks: %v", err)
			}
			if len(tasks) == 0 {
				t.Fatalf("GetCircleTasks 空")
			}
		}},
		{"circle/GetCircleImages", func(t *testing.T) {
			imgs, err := liveClient.GetCircleImages(ctx, liveToken, 1, 3)
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("GetCircleImages: %v", err)
			}
			t.Logf("images: %d", len(imgs))
		}},
		{"circle/GetDictList(23)", func(t *testing.T) {
			dicts, err := liveClient.GetDictList(ctx, liveToken, 23)
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("GetDictList: %v", err)
			}
			if len(dicts) == 0 {
				t.Fatalf("GetDictList 空")
			}
		}},
		{"honor/GetHonorTypes", func(t *testing.T) {
			list, err := liveClient.GetHonorTypes(ctx, liveToken)
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("GetHonorTypes: %v", err)
			}
			if len(list) == 0 {
				t.Fatalf("GetHonorTypes 空")
			}
		}},
		{"honor/GetHonorTypeOptions", func(t *testing.T) {
			opts, err := liveClient.GetHonorTypeOptions(ctx, liveToken)
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("GetHonorTypeOptions: %v", err)
			}
			if len(opts) == 0 {
				t.Fatalf("GetHonorTypeOptions 空")
			}
		}},
		{"honor/GetHonorTypeForSelect", func(t *testing.T) {
			opts, err := liveClient.GetHonorTypeForSelect(ctx, liveToken)
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("GetHonorTypeForSelect: %v", err)
			}
			if len(opts) == 0 {
				t.Fatalf("GetHonorTypeForSelect 空")
			}
		}},
		{"honor/GetHonorLevel(1148)", func(t *testing.T) {
			opts, err := liveClient.GetHonorLevel(ctx, liveToken, 1148)
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("GetHonorLevel: %v", err)
			}
			if len(opts) == 0 {
				t.Fatalf("GetHonorLevel 空")
			}
		}},
		{"honor/GetHonorList", func(t *testing.T) {
			res, err := liveClient.GetHonorList(ctx, liveToken, 1, 10, "")
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("GetHonorList: %v", err)
			}
			if res == nil {
				t.Fatalf("GetHonorList 返回 nil")
			}
		}},
		{"typical/GetTypicalCaseList", func(t *testing.T) {
			res, err := liveClient.GetTypicalCaseList(ctx, liveToken, 1, 10)
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("GetTypicalCaseList: %v", err)
			}
			if res == nil {
				t.Fatalf("GetTypicalCaseList 返回 nil")
			}
		}},
		{"typical/GetTypicalCaseList status=1", func(t *testing.T) {
			res, err := liveClient.GetTypicalCaseList(ctx, liveToken, 1, 10, 1)
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("GetTypicalCaseList status=1: %v", err)
			}
			if res == nil {
				t.Fatalf("返回 nil")
			}
		}},
		{"self-eval/QuerySelfEvaluation", func(t *testing.T) {
			st, err := liveClient.QuerySelfEvaluation(ctx, liveToken)
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("QuerySelfEvaluation: %v", err)
			}
			t.Logf("self-eval: %+v", st)
		}},
		{"self-eval/QuerySelfGradEvaluation", func(t *testing.T) {
			st, err := liveClient.QuerySelfGradEvaluation(ctx, liveToken)
			if isSkipableLiveErr(err) {
				t.Skipf("skip live: %v", err)
			}
			if err != nil {
				t.Fatalf("QuerySelfGradEvaluation: %v", err)
			}
			t.Logf("grad: %+v", st)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, tc.fn)
	}
}

func isSkipableLiveErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, client.ErrRateLimited) || errors.Is(err, client.ErrServiceUnavailable) || errors.Is(err, client.ErrNetwork) {
		return true
	}
	msg := err.Error()
	// 常见瞬态文案
	if contains(msg, "429") || contains(msg, "503") || contains(msg, "502") || contains(msg, "504") || contains(msg, "timeout") || contains(msg, "Timeout") {
		return true
	}
	return false
}

func contains(s, sub string) bool { return len(s) >= len(sub) && indexOf(s, sub) >= 0 }

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
