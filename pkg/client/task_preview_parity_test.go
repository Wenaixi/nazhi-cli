package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// parityMock 返回固定任务元数据的 mock 服务端；addCircle 收到的请求体写入 returned map。
func parityMock(t *testing.T) (*httptest.Server, *map[string]any) {
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
		case "/api/studentCircleNew/addCircle":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode addCircle body: %v", err)
			}
			*submitted = body
			_, _ = w.Write([]byte(`{"code":1,"message":"ok"}`))
		default:
			http.Error(w, "unexpected "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	return srv, submitted
}

// TestPreviewParity_线上JSON与提交一致 是防分叉锁：
// 预览组装的 payload 序列化后的线上 JSON 必须与 SubmitTask 实际发送的一致
// （id 为新增记录的空值差异，不参与对比）。
// 任何一侧单独加字段/改 Trim 语义而另一侧未同步时，本测试失败。
func TestPreviewParity_线上JSON与提交一致(t *testing.T) {
	in := types.TaskSubmitInput{
		TaskID: 42, Content: "心得", Name: "活动", HostName: "主办方",
		Address: "  场地  ", OrgName: "社区", Level: "  ",
		CheckResult: types.CheckResultPass, Hours: "2.5",
		CircleBeginDate: "2026-01-01", CircleEndDate: "2026-01-02",
	}

	// 提交侧：真实走 SubmitTask
	srv1, submitted := parityMock(t)
	defer srv1.Close()
	c, _ := New(WithBaseURL(srv1.URL), WithSSOBase(srv1.URL), WithTimeout(5*1e9))
	defer c.Close()
	if _, err := c.SubmitTask(t.Context(), "tok", in); err != nil {
		t.Fatalf("submit: %v", err)
	}
	delete(*submitted, "id") // 新增记录 id 为空，预览同样省略

	// 预览侧：真实走 PreviewSubmitPayload
	srv2, _ := parityMock(t)
	defer srv2.Close()
	c2, _ := New(WithBaseURL(srv2.URL), WithSSOBase(srv2.URL), WithTimeout(5*1e9))
	defer c2.Close()
	preview, err := c2.PreviewSubmitPayload(t.Context(), "tok", in)
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

	if len(pv) != len(*submitted) {
		t.Fatalf("键数不一致：preview=%d submit=%d", len(pv), len(*submitted))
	}
	for k, v := range pv {
		got, ok := (*submitted)[k]
		if !ok {
			t.Fatalf("preview 有而 submit 没有的字段 %s = %v", k, v)
		}
		a, _ := json.Marshal(got)
		b, _ := json.Marshal(v)
		if string(a) != string(b) {
			t.Fatalf("字段 %s 分叉：preview=%s submit=%s", k, b, a)
		}
	}
}
