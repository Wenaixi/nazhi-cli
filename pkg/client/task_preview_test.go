package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

func previewMock(t *testing.T, metaJSON string, cap *types.TaskAddCirclePayload) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"code":1}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"code":1,"returnData":{}}`))
		case "/api/studentCircleNew/getCircleTypeByTaskId":
			w.WriteHeader(200)
			_, _ = w.Write([]byte(metaJSON))
		case "/api/studentCircleNew/addCircle", "/api/studentCircleNew/editCircle":
			t.Fatalf("preview should not call %s", r.URL.Path)
			w.WriteHeader(500)
		default:
			w.WriteHeader(500)
		}
	}))
}

func TestPreviewSubmit_ExposesPresetsAndKeepsEmptyDefaultsEmpty(t *testing.T) {
	var cap types.TaskAddCirclePayload
	meta := `{"code":1,"dataMap":{"task_id":999,"circle_type_id":55,"dimension_id":7,"hours":8,"task_name":"2025-2026第二学期调查表","type_name":"社会调查","dimension_name":"实践创新","remark":"","type":6}}`
	srv := previewMock(t, meta, &cap)
	defer srv.Close()
	c, _ := New(WithBaseURL(srv.URL), WithSSOBase(srv.URL), WithTimeout(5*1e9))
	defer c.Close()
	preview, err := c.PreviewSubmitPayload(t.Context(), "tok", types.TaskSubmitInput{
		TaskID: 999, Content: "心得",
		OrgName: "社区", Address: "基地",
		// 空 Level/CheckResult 的部分场景在 6 中必填，显式给 checkResult，但 level 保持空验证不糊弄
		CheckResult: types.CheckResultPass,
		// Hours 留空 -> 应该用任务预设 8
	})
	if err != nil {
		t.Fatalf("preview submit: %v", err)
	}
	if preview.CircleTaskID != 999 || preview.CircleTypeID != 55 || preview.DimensionID != 7 {
		t.Fatalf("preset ids not exposed %+v", preview)
	}
	if preview.Hours != 8 {
		t.Fatalf("hours preset 8 not exposed, got %v", preview.Hours)
	}
	if preview.Address != "基地" || preview.OrgName != "社区" {
		t.Fatalf("user fields not kept %+v", preview)
	}
	if preview.Level != "" {
		t.Fatalf("empty Level must stay empty, not \"5\", got %q", preview.Level)
	}
	// Address empty case: another preview with no Address must stay "" not school name
	preview2, err := c.PreviewSubmitPayload(t.Context(), "tok", types.TaskSubmitInput{
		TaskID: 999, Content: "心得2", OrgName: "社区", CheckResult: types.CheckResultExcellent, Hours: "2",
	})
	if err != nil {
		t.Fatalf("preview2: %v", err)
	}
	if preview2.Address != "" {
		t.Fatalf("empty Address must stay empty, got %q", preview2.Address)
	}
	_ = json.RawMessage{}
}

func TestPreviewEdit_CarriesIDAndSamePresets(t *testing.T) {
	var cap types.TaskAddCirclePayload
	meta := `{"code":1,"dataMap":{"task_id":1001,"circle_type_id":10,"dimension_id":3,"hours":0,"task_name":"t","type_name":"x","dimension_name":"y","remark":"","type":4}}`
	srv := previewMock(t, meta, &cap)
	defer srv.Close()
	c, _ := New(WithBaseURL(srv.URL), WithSSOBase(srv.URL), WithTimeout(5*1e9))
	defer c.Close()
	preview, err := c.PreviewEditPayload(t.Context(), "tok", types.TaskEditInput{
		ID: 5400001, TaskID: 1001, Content: "修改",
		Name: "活动", HostName: "校", ObtainTime: "2026-04-15",
		Level: types.TaskLevelSchool, Hours: "4",
	})
	if err != nil {
		t.Fatalf("preview edit: %v", err)
	}
	if preview.ID == nil || *preview.ID != 5400001 {
		t.Fatalf("preview edit must carry id, got %v", preview.ID)
	}
	if preview.Level != types.TaskLevelSchool || preview.Hours != 4 {
		t.Fatalf("preview edit fields %+v", preview)
	}
}
