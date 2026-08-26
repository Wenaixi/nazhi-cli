package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// parityEditMock 在提交版 mock 基础上把 addCircle 端点换成 editCircle。
func parityEditMock(t *testing.T) (*httptest.Server, *map[string]any) {
	t.Helper()
	submitted := new(map[string]any)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			_, _ = w.Write([]byte(`{"code":1}`))
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write([]byte(`{"code":1,"returnData":{}}`))
		case "/api/studentCircleNew/getCircleTypeByTaskId":
			_, _ = w.Write([]byte(`{"code":1,"dataMap":{"task_id":42,"circle_type_id":11,"dimension_id":5,"hours":6,"task_name":"t","type_name":"tn","dimension_name":"dn","remark":"","type":3}}`))
		case "/api/studentCircleNew/editCircle":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode editCircle body: %v", err)
			}
			*submitted = body
			_, _ = w.Write([]byte(`{"code":1,"message":"ok"}`))
		default:
			http.Error(w, "unexpected "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	return srv, submitted
}

// TestPreviewParity_Edit 编辑侧防分叉锁：PreviewEditPayload 序列化后的 JSON
// 必须与 EditCircle 实际发送的 wire body 一致。两侧共用 buildTaskPayload，
// 但此前仅提交侧有字节级锁定；若未来 buildTaskPayload 引入依赖具体 Input
// 类型的分支，编辑侧可能静默分叉——本测试堵住该窗口。
func TestPreviewParity_Edit(t *testing.T) {
	in := types.TaskEditInput{
		ID: 5690624, TaskID: 42, Content: "心得", Name: "活动",
		HostName: "主办方", Address: "  场地  ", OrgName: "社区",
		Level: "  ", CheckResult: types.CheckResultPass, Hours: "2.5",
		CircleBeginDate: "2026-01-01", CircleEndDate: "2026-01-02",
	}

	// 编辑侧：真实走 EditCircle
	srv1, edited := parityEditMock(t)
	defer srv1.Close()
	c, _ := New(WithBaseURL(srv1.URL), WithSSOBase(srv1.URL), WithTimeout(5*1e9))
	defer c.Close()
	if _, err := c.EditCircle(t.Context(), "tok", in); err != nil {
		t.Fatalf("edit: %v", err)
	}

	// 预览侧：真实走 PreviewEditPayload
	srv2, _ := parityEditMock(t)
	defer srv2.Close()
	c2, _ := New(WithBaseURL(srv2.URL), WithSSOBase(srv2.URL), WithTimeout(5*1e9))
	defer c2.Close()
	preview, err := c2.PreviewEditPayload(t.Context(), "tok", in)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	previewJSON, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("marshal preview: %v", err)
	}
	var pv map[string]any
	if err := json.Unmarshal(previewJSON, &pv); err != nil {
		t.Fatalf("unmarshal preview: %v", err)
	}

	if len(pv) != len(*edited) {
		t.Fatalf("键数不一致：preview=%d edit=%d", len(pv), len(*edited))
	}
	for k, v := range pv {
		got, ok := (*edited)[k]
		if !ok {
			t.Fatalf("preview 有而 edit 没有的字段 %s = %v", k, v)
		}
		a, _ := json.Marshal(got)
		b, _ := json.Marshal(v)
		if string(a) != string(b) {
			t.Fatalf("字段 %s 分叉：preview=%s edit=%s", k, b, a)
		}
	}
}
