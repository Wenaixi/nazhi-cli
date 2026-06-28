package client_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// ─── E2: 学校 ID 类型分裂 RED 测试 ───

// TestGetSchoolID_NonNumericSchoolID_ReturnsErrInvalidPayload 验证 GetSchoolID
// 在收到非数字 school_id 时返回包装 ErrInvalidPayload 的错误。
// 历史 bug：school_id 从 map 提取后仅 fmt.Sprintf 转字符串，无格式校验，
// 非数字值（如 "abc"、null）会被静默传递给登录请求，导致难以排查的失败。
// 修复后应在 GetSchoolID 阶段就明确报错。
func TestGetSchoolID_NonNumericSchoolID_ReturnsErrInvalidPayload(t *testing.T) {
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/teacher/auth/studentLogin/getSchoolIdByStudentNumber" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(unifiedJSON(1, "成功", nil, []map[string]any{
				{"school_id": "not-a-number", "NAME": "测试学校"},
			})))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html>ok</html>"))
	}))
	defer sso.Close()

	c := newTestClient(sso, nil, nil)
	_, _, err := c.GetSchoolID(context.Background(), "TEST2025001")
	if err == nil {
		t.Fatal("期望非数字 school_id 返回 error, 得到 nil")
	}
	if !errors.Is(err, client.ErrInvalidPayload) {
		t.Errorf("期望错误链包含 ErrInvalidPayload, 得到: %v", err)
	}
}

// TestGetSchoolID_NilSchoolID_ReturnsErrInvalidPayload 验证 school_id 为 nil 时也返回
// ErrInvalidPayload（防御性：map[string]any 中不存在的 key 或显式 nil 值）。
func TestGetSchoolID_NilSchoolID_ReturnsErrInvalidPayload(t *testing.T) {
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/teacher/auth/studentLogin/getSchoolIdByStudentNumber" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(unifiedJSON(1, "成功", nil, []map[string]any{
				{"school_id": nil, "NAME": "测试学校"},
			})))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html>ok</html>"))
	}))
	defer sso.Close()

	c := newTestClient(sso, nil, nil)
	_, _, err := c.GetSchoolID(context.Background(), "TEST2025001")
	if err == nil {
		t.Fatal("期望 nil school_id 返回 error, 得到 nil")
	}
	if !errors.Is(err, client.ErrInvalidPayload) {
		t.Errorf("期望错误链包含 ErrInvalidPayload, 得到: %v", err)
	}
}

// TestGetSchoolID_ValidNumericSchoolID_AfterE2Fix 验证合规数字 school_id 正常返回（回归测试）。
// E2 修复后 GetSchoolID 增加数字校验，确保合法值不受影响。
func TestGetSchoolID_ValidNumericSchoolID_AfterE2Fix(t *testing.T) {
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/teacher/auth/studentLogin/getSchoolIdByStudentNumber" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(unifiedJSON(1, "成功", nil, []map[string]any{
				{"school_id": "173", "NAME": "福清一中"},
			})))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html>ok</html>"))
	}))
	defer sso.Close()

	c := newTestClient(sso, nil, nil)
	schoolID, schoolName, err := c.GetSchoolID(context.Background(), "TEST2025001")
	if err != nil {
		t.Fatalf("合法 school_id 不应返回 error: %v", err)
	}
	if schoolID != "173" {
		t.Errorf("期望 schoolID=173, 得到 %s", schoolID)
	}
	if schoolName != "福清一中" {
		t.Errorf("期望 schoolName=福清一中, 得到 %s", schoolName)
	}
}
