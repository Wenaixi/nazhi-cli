package client

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestSubmitTask_TooManyPicturesSkipsUpload 回归测试：
// 图片超过 2 张上限时必须在任何上传发生前拒绝。
//
// 背景（十三域审计 P2-D）：原实现在上传循环之后才校验 len(pictureList)>2，
// 传 3 个 ImagePaths 会先把 3 张全部上传成功再返回 ErrInvalidPayload，
// 已上传附件成为服务端孤儿（无删除接口可回收）。前端 el-upload :limit=2
// 在选择阶段即拦截，不产生该副作用。
func TestSubmitTask_TooManyPicturesSkipsUpload(t *testing.T) {
	var uploadCalls atomic.Int32
	upload := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploadCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"returnData":{"id":67890}}`))
	}))
	defer upload.Close()

	biz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","schoolName":"测试中学"}}`))
		case "/api/studentCircleNew/getCircleTypeByTaskId":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","dataMap":{"task_name":"班会","circle_type_id":9256,"hours":1.0,"type_name":"主题班会","dimension_id":9,"dimension_name":"思想品德","task_id":1001,"remark":"","type":10}}`))
		case "/api/studentCircleNew/addCircle":
			t.Error("超量图片不应到达 addCircle")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"提交成功"}`))
		default:
			t.Errorf("意外路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer biz.Close()

	c, err := New(
		WithBaseURL(biz.URL),
		WithSSOBase(biz.URL),
		WithUploadURL(upload.URL),
		WithTimeout(5*1000*1000*1000),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer func() { _ = c.Close() }()

	// 构造 3 张合法小图片
	dir := t.TempDir()
	paths := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		p := filepath.Join(dir, string(rune('0'+i))+".png")
		f, err := os.Create(p)
		if err != nil {
			t.Fatalf("创建测试图片失败: %v", err)
		}
		img := image.NewRGBA(image.Rect(0, 0, 2, 2))
		img.Set(0, 0, color.RGBA{255, 0, 0, 255})
		if err := png.Encode(f, img); err != nil {
			t.Fatalf("编码测试图片失败: %v", err)
		}
		_ = f.Close()
		paths = append(paths, p)
	}

	_, err = c.SubmitTask(context.Background(), "test-token", types.TaskSubmitInput{
		TaskID:     1001,
		Content:    "三张图的写实",
		ImagePaths: paths,
	})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("超量图片应返回 ErrInvalidPayload，实际: %v", err)
	}
	if n := uploadCalls.Load(); n != 0 {
		t.Errorf("拒绝必须发生在任何上传之前，实际已上传 %d 次", n)
	}
}
