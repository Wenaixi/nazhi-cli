package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

func previewPureMock(t *testing.T) *httptest.Server {
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
			_, _ = w.Write([]byte(`{"code":1,"dataMap":{"task_id":1,"circle_type_id":10,"dimension_id":3,"hours":2,"task_name":"t","type_name":"x","dimension_name":"y","remark":"","type":1}}`))
		case "/api/studentCircleNew/addCircle", "/api/studentCircleNew/editCircle":
			// 纯预览不应走到这里
			t.Fatalf("pure preview must not POST %s", r.URL.Path)
			w.WriteHeader(500)
		default:
			if r.URL.Path == "/common/upload/uploadImage" {
				t.Fatalf("pure preview must not upload %s", r.URL.Path)
				w.WriteHeader(500)
				return
			}
			w.WriteHeader(500)
		}
	}))
}

func TestPreviewSubmit_PureNoUpload(t *testing.T) {
	srv := previewPureMock(t)
	defer srv.Close()
	c, _ := New(WithBaseURL(srv.URL), WithSSOBase(srv.URL), WithUploadURL(srv.URL), WithTimeout(5*1e9))
	defer c.Close()
	p, err := c.PreviewSubmitPayload(t.Context(), "tok", types.TaskSubmitInput{
		TaskID: 1, Content: "c", Name: "n", Address: "a", PlayRole: "1",
		ImagePaths: []string{"/tmp/should-not-upload.jpg"},
	})
	if err != nil {
		t.Fatalf("preview submit pure: %v", err)
	}
	if p == nil {
		t.Fatal("preview nil")
	}
	// 纯预览：ImagePaths 不应被上传，pictureList 应为空（仅 ImageIDs 合并）
	if len(p.PictureList) != 0 {
		raw, _ := json.Marshal(p.PictureList)
		t.Fatalf("pure preview must have empty pictureList, got %s", string(raw))
	}
	if p.Address != "a" {
		t.Fatalf("address not preserved %q", p.Address)
	}
}
