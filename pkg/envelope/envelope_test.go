package envelope

import (
	"encoding/json"
	"testing"
)

func TestSuccess(t *testing.T) {
	e := Success(map[string]string{"k": "v"})
	if e.Status != StatusSuccess {
		t.Errorf("Status = %q, want success", e.Status)
	}
	if e.Code != 200 {
		t.Errorf("Code = %d, want 200", e.Code)
	}
	if e.Message != "" {
		t.Errorf("Message = %q, want empty", e.Message)
	}
	if e.ExitCode() != 0 {
		t.Errorf("ExitCode = %d, want 0", e.ExitCode())
	}
	// 验证 JSON 序列化
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"status":"success","code":200,"message":"","data":{"k":"v"}}`
	if string(b) != want {
		t.Errorf("JSON = %s, want %s", b, want)
	}
}

func TestEmpty(t *testing.T) {
	e := Empty("no data")
	if e.Status != StatusSuccess {
		t.Errorf("Status = %q, want success", e.Status)
	}
	if e.Code != 204 {
		t.Errorf("Code = %d, want 204", e.Code)
	}
	if e.Message != "no data" {
		t.Errorf("Message = %q, want no data", e.Message)
	}
	if e.ExitCode() != 0 {
		t.Errorf("ExitCode = %d, want 0", e.ExitCode())
	}
}

func TestPartial(t *testing.T) {
	e := Partial(207, "多维度部分失败", []string{"task1"})
	if e.Status != StatusPartial {
		t.Errorf("Status = %q, want partial", e.Status)
	}
	if e.Code != 207 {
		t.Errorf("Code = %d, want 207", e.Code)
	}
	if e.ExitCode() != 1 {
		t.Errorf("ExitCode = %d, want 1", e.ExitCode())
	}
}

func TestError(t *testing.T) {
	cases := []struct {
		code   int
		wantEC int
		desc   string
	}{
		{400, 3, "参数错误 → 3"},
		{401, 1, "鉴权失败 (4xx 非 400) → 1"},
		{404, 1, "资源不存在 (4xx 非 400) → 1"},
		{500, 2, "服务端错误 → 2"},
		{502, 2, "网关错误 → 2"},
		{0, 1, "code=0 兜底 → 1"},
	}
	for _, tc := range cases {
		e := Error(tc.code, "msg")
		if e.ExitCode() != tc.wantEC {
			t.Errorf("code=%d: ExitCode=%d, want %d (%s)", tc.code, e.ExitCode(), tc.wantEC, tc.desc)
		}
		if e.Status != StatusError {
			t.Errorf("code=%d: Status=%q, want error", tc.code, e.Status)
		}
	}
}

func TestExitCodeBoundary(t *testing.T) {
	// 验证边界：Code=399 视为业务错误 1，Code=400 参数错误 3，Code=499 业务错误 1，Code=500 服务端 2
	cases := []struct {
		status Status
		code   int
		want   int
	}{
		{StatusSuccess, 200, 0},
		{StatusPartial, 207, 1},
		{StatusError, 399, 1}, // 4xx 边界外（接近 400）→ 业务错误 1
		{StatusError, 400, 3}, // 参数错误
		{StatusError, 401, 1}, // 业务错误
		{StatusError, 499, 1}, // 业务错误边界
		{StatusError, 500, 2}, // 服务端
		{StatusError, 599, 2}, // 服务端
	}
	for _, tc := range cases {
		e := &Envelope{Status: tc.status, Code: tc.code}
		if got := e.ExitCode(); got != tc.want {
			t.Errorf("Status=%q Code=%d: ExitCode=%d, want %d", tc.status, tc.code, got, tc.want)
		}
	}
}
