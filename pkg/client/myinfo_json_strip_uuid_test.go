package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestGetMyInfoJSON_StripsStudentUuid 锁定 P2-1（user-info 域）：
// GetMyInfoJSON 与 ActivateSessionJSON 是 CLI whoami/GetMyInfoJSON 的输出通道，
// 序列化前必须剔除 StudentUuid（学生 UUID/密码）敏感值——前端 modifyBox.vue:185 读取后
// 显式清零即佐证该字段属只写不读的敏感载体。剔除用浅拷贝（禁止原地置空——info 与
// sm.cachedUserInfo 共享指针，见 session.go P0-A7 教训），结构化 GetMyInfo 返回值不受影响
// （Go 调用方自行裁决是否消费该字段）。
func TestGetMyInfoJSON_StripsStudentUuid(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001","studentUuid":"sensitive-uuid"}}`))
		default:
			t.Errorf("未预期路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	// 1) JSON 透传路径：studentUuid 必须被剔除
	raw, err := c.GetMyInfoJSON(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetMyInfoJSON 失败: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("输出非合法 JSON: %v", err)
	}
	if v, ok := out["studentUuid"]; ok && v != nil && v != "" {
		t.Errorf("GetMyInfoJSON 输出不应含 studentUuid 敏感值，实际 %v", v)
	}
	if _, ok := out["name"]; !ok {
		t.Error("GetMyInfoJSON 正常字段应保留（name 缺失）")
	}

	// 2) ActivateSessionJSON 同契约
	raw2, err := c.ActivateSessionJSON(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("ActivateSessionJSON 失败: %v", err)
	}
	var out2 map[string]any
	if err := json.Unmarshal(raw2, &out2); err != nil {
		t.Fatalf("ActivateSessionJSON 输出非合法 JSON: %v", err)
	}
	if v, ok := out2["studentUuid"]; ok && v != nil && v != "" {
		t.Errorf("ActivateSessionJSON 输出不应含 studentUuid 敏感值，实际 %v", v)
	}

	// 3) 结构化 GetMyInfo：字段保留（Go 调用方自行裁决）
	info, err := c.GetMyInfo(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetMyInfo 失败: %v", err)
	}
	if info == nil || info.StudentUuid != "sensitive-uuid" {
		t.Errorf("结构化 GetMyInfo 应保留 StudentUuid（调用方裁决），实际 %+v", info)
	}
}
