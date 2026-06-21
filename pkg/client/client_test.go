// Package client_test 包含 nazhi-cli SDK 的全量测试。
package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ─── 辅助 ───

// unifiedJSON 生成目标平台统一响应 JSON。
func unifiedJSON(code int, msg string, returnData any, dataList any) string {
	m := map[string]any{
		"code": code,
		"msg":  msg,
	}
	if returnData != nil {
		m["returnData"] = returnData
	}
	if dataList != nil {
		m["dataList"] = dataList
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// newTestClient 为测试创建 Client（连接 mock server）。
func newTestClient(ssoServer *httptest.Server, bizServer *httptest.Server, uploadServer *httptest.Server) *client.Client {
	opts := []client.Option{
		client.WithTimeout(5 * time.Second),
	}
	if ssoServer != nil {
		opts = append(opts, client.WithSSOBase(ssoServer.URL))
	}
	if bizServer != nil {
		opts = append(opts, client.WithBaseURL(bizServer.URL))
	}
	if uploadServer != nil {
		opts = append(opts, client.WithUploadURL(uploadServer.URL))
	}
	return client.New(opts...)
}

// ─── 测试: InitSession ───

func TestInitSession(t *testing.T) {
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/uiStudentLogin/login" {
			t.Errorf("期望路径 /uiStudentLogin/login, 得到 %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("期望 GET, 得到 %s", r.Method)
		}
		// 验证 Cookie 请求头存在
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html>login</html>"))
	}))
	defer sso.Close()

	c := newTestClient(sso, nil, nil)
	err := c.InitSession(context.Background())
	if err != nil {
		t.Fatalf("InitSession 失败: %v", err)
	}
}

// ─── 测试: GetSchoolID ───

func TestGetSchoolID(t *testing.T) {
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/teacher/auth/studentLogin/getSchoolIdByStudentNumber" {
			t.Errorf("期望路径 getSchoolIdByStudentNumber, 得到 %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST, 得到 %s", r.Method)
		}

		// 验证请求头
		if r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
			t.Errorf("期望 X-Requested-With, 得到 %s", r.Header.Get("X-Requested-With"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(unifiedJSON(1, "成功", nil, []map[string]any{
			{"school_id": "173", "NAME": "福清一中"},
		})))
	}))
	defer sso.Close()

	c := newTestClient(sso, nil, nil)
	schoolID, schoolName, err := c.GetSchoolID(context.Background(), "S1234567890")
	if err != nil {
		t.Fatalf("GetSchoolID 失败: %v", err)
	}
	if schoolID != "173" {
		t.Errorf("期望 schoolID=173, 得到 %s", schoolID)
	}
	if schoolName != "福清一中" {
		t.Errorf("期望 schoolName=福清一中, 得到 %s", schoolName)
	}
}

// ─── 测试: FetchCaptcha ───

func TestFetchCaptcha(t *testing.T) {
	callOrder := 0
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			callOrder = 1
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html>login</html>"))
		case "/teacher/auth/studentLogin/getSchoolIdByStudentNumber":
			if callOrder != 1 {
				t.Errorf("GetSchoolID 应该在 InitSession 之后调用")
			}
			callOrder = 2
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(unifiedJSON(1, "成功", nil, []map[string]any{
				{"school_id": "173", "NAME": "福清一中"},
			})))
		case "/kaptcha/kaptcha.jpg":
			if callOrder != 2 {
				t.Errorf("kaptcha 应该在 GetSchoolID 之后调用")
			}
			callOrder = 3
			// 返回 JPEG 图片二进制（一个最简单的有效 JPEG）
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			// 1x1 像素 JPEG
			jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43, 0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09, 0x09, 0x08, 0x0A, 0x0C, 0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12, 0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D, 0x1A, 0x1C, 0x1C, 0x20, 0x24, 0x2E, 0x27, 0x20, 0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29, 0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27, 0x39, 0x3D, 0x38, 0x32, 0x3C, 0x2E, 0x33, 0x34, 0x32, 0xFF, 0xC0, 0x00, 0x0B, 0x08, 0x00, 0x01, 0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xFF, 0xC4, 0x00, 0x1F, 0x00, 0x00, 0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0xFF, 0xC4, 0x00, 0xB5, 0x10, 0x00, 0x02, 0x01, 0x03, 0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12, 0x21, 0x31, 0x41, 0x06, 0x13, 0x51, 0x61, 0x07, 0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xA1, 0x08, 0x23, 0x42, 0xB1, 0xC1, 0x15, 0x52, 0xD1, 0xF0, 0x24, 0x33, 0x62, 0x72, 0x82, 0x09, 0x0A, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2A, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3A, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4A, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5A, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6A, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79, 0x7A, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8A, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9A, 0xA2, 0xA3, 0xA4, 0xA5, 0xA6, 0xA7, 0xA8, 0xA9, 0xAA, 0xB2, 0xB3, 0xB4, 0xB5, 0xB6, 0xB7, 0xB8, 0xB9, 0xBA, 0xC2, 0xC3, 0xC4, 0xC5, 0xC6, 0xC7, 0xC8, 0xC9, 0xCA, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7, 0xD8, 0xD9, 0xDA, 0xE1, 0xE2, 0xE3, 0xE4, 0xE5, 0xE6, 0xE7, 0xE8, 0xE9, 0xEA, 0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3F, 0x00, 0x7B, 0x94, 0x11, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFF, 0xD9}
			w.Write(jpegData)
		default:
			t.Errorf("未期望的路径: %s", r.URL.Path)
		}
	}))
	defer sso.Close()

	c := newTestClient(sso, nil, nil)
	captchaBase64, schoolID, err := c.FetchCaptcha(context.Background(), "S1234567890")
	if err != nil {
		t.Fatalf("FetchCaptcha 失败: %v", err)
	}
	if captchaBase64 == "" {
		t.Errorf("期望非空的 base64 验证码图片")
	}
	if schoolID != "173" {
		t.Errorf("期望 schoolID=173, 得到 %s", schoolID)
	}
}

// ─── 测试: ValidateCaptcha ───

func TestValidateCaptcha(t *testing.T) {
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/uiStudentLogin/validateCaptcha" {
			t.Errorf("期望路径 validateCaptcha, 得到 %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST, 得到 %s", r.Method)
		}

		// 解析请求体
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["captcha"] == "" {
			t.Errorf("请求体缺少 captcha 字段")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(unifiedJSON(1, "验证码校验成功", nil, nil)))
	}))
	defer sso.Close()

	c := newTestClient(sso, nil, nil)
	err := c.ValidateCaptcha(context.Background(), "AB12")
	if err != nil {
		t.Fatalf("ValidateCaptcha 失败: %v", err)
	}
}

func TestValidateCaptcha_Fail(t *testing.T) {
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(unifiedJSON(0, "验证码校验失败", nil, nil)))
	}))
	defer sso.Close()

	c := newTestClient(sso, nil, nil)
	err := c.ValidateCaptcha(context.Background(), "WRONG")
	if err == nil {
		t.Fatal("期望验证码校验失败，但得到 nil error")
	}
}

// ─── 测试: Login ───

func TestLogin(t *testing.T) {
	callStep := 0
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			callStep = 1
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html>login</html>"))
		case "/teacher/auth/studentLogin/getSchoolIdByStudentNumber":
			if callStep != 1 {
				t.Errorf("调用顺序错误: getSchoolId 应在 login 之后")
			}
			callStep = 2
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(unifiedJSON(1, "成功", nil, []map[string]any{
				{"school_id": "173", "NAME": "福清一中"},
			})))
		case "/kaptcha/kaptcha.jpg":
			if callStep != 2 {
				t.Errorf("调用顺序错误: kaptcha 应在 getSchoolId 之后")
			}
			callStep = 3
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte{0xFF, 0xD8, 0xFF})
		case "/uiStudentLogin/validateCaptcha":
			if callStep != 3 {
				t.Errorf("调用顺序错误: validateCaptcha 应在 kaptcha 之后")
			}
			callStep = 4
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(unifiedJSON(1, "验证码校验成功", nil, nil)))
		case "/teacher/auth/studentLogin/validate":
			if callStep != 4 {
				t.Errorf("调用顺序错误: validate 应在验证码之后")
			}
			callStep = 5

			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["username"] != "S1234567890" {
				t.Errorf("期望 username=S1234567890, 得到 %s", body["username"])
			}
			if body["captcha"] != "AB12" {
				t.Errorf("期望 captcha=AB12, 得到 %s", body["captcha"])
			}

			// 返回 302 重定向（标准登录成功响应）
			w.Header().Set("Location", "/homepage?token=eyJhbGciOiJIUzI1NiJ9.test-token-123")
			w.WriteHeader(http.StatusFound)
		}
	}))
	defer sso.Close()

	c := newTestClient(sso, nil, nil)
	resp, err := c.Login(context.Background(), types.LoginRequest{
		Username: "S1234567890",
		Password: "TestPass123",
		Captcha:  "AB12",
	})
	if err != nil {
		t.Fatalf("Login 失败: %v", err)
	}
	if resp.Token != "eyJhbGciOiJIUzI1NiJ9.test-token-123" {
		t.Errorf("期望 token=eyJhbGciOiJIUzI1NiJ9.test-token-123, 得到 %s", resp.Token)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html>login</html>"))
		case "/teacher/auth/studentLogin/getSchoolIdByStudentNumber":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(unifiedJSON(1, "成功", nil, []map[string]any{
				{"school_id": "173", "NAME": "福清一中"},
			})))
		case "/kaptcha/kaptcha.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte{0xFF, 0xD8, 0xFF})
		case "/uiStudentLogin/validateCaptcha":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(unifiedJSON(1, "验证码校验成功", nil, nil)))
		case "/teacher/auth/studentLogin/validate":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(unifiedJSON(0, "用户名或密码错误", nil, nil)))
		}
	}))
	defer sso.Close()

	c := newTestClient(sso, nil, nil)
	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "S1234567890",
		Password: "wrong",
		Captcha:  "AB12",
	})
	if err == nil {
		t.Fatal("期望登录失败，但得到 nil error")
	}
	if !strings.Contains(err.Error(), "login rejected") {
		t.Errorf("期望错误包含 'login rejected', 得到 %v", err)
	}
}

// ─── 测试: ActivateSession ───

func TestActivateSession(t *testing.T) {
	callOrder := 0
	biz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			callOrder = 1
			if r.Header.Get("X-Auth-Token") != "test-token" {
				t.Errorf("期望 X-Auth-Token=test-token")
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html>home</html>"))
		case "/api/studentInfo/getMenu":
			if callOrder != 1 {
				t.Errorf("getMenu 应在首页之后")
			}
			callOrder = 2
			if r.Header.Get("Referer") == "" {
				t.Errorf("期望 Referer 不为空")
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(unifiedJSON(1, "成功", map[string]any{
				"name": "张三", "studentNumber": "S1234567890",
			}, nil)))
		}
	}))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	info, err := c.ActivateSession(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("ActivateSession 失败: %v", err)
	}
	if info == nil {
		t.Fatal("期望非 nil UserInfo")
	}
	if info.Name != "张三" {
		t.Errorf("期望 Name=张三, 得到 %s", info.Name)
	}
}

// ─── 测试: GetMyInfo ───

func TestGetMyInfo(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentInfo/getMyInfo" {
			t.Errorf("期望路径 getMyInfo, 得到 %s", r.URL.Path)
		}
		if r.Header.Get("Referer") == "" {
			t.Errorf("期望 Referer 不为空")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(unifiedJSON(1, "成功", map[string]any{
			"name": "张三",
			"studentNumber": "S1234567890",
			"schoolName":    "福清一中",
			"gradeName":     "高一",
			"className":     "八班",
			"seat":          45,
		}, nil)))
	}))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	info, err := c.GetMyInfo(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetMyInfo 失败: %v", err)
	}
	if info == nil {
		t.Fatal("期望非 nil UserInfo")
	}
	if info.Name != "张三" {
		t.Errorf("期望 Name=张三, 得到 %s", info.Name)
	}
	if info.ClassName != "八班" {
		t.Errorf("期望 ClassName=八班, 得到 %s", info.ClassName)
	}
}

// ─── 测试: FetchTasks ───

func TestFetchTasks(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html>home</html>"))
		case "/api/studentInfo/getMenu":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(unifiedJSON(1, "成功", nil, nil)))
		case "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(unifiedJSON(1, "成功", nil, []map[string]any{
				{"id": 9, "name": "思想品德"},
				{"id": 14, "name": "劳动素养"},
			})))
		case "/api/studentCircleNew/getCircleStatistics":
			dimID := r.URL.Query().Get("dimensionId")
			var tasks []map[string]any
			if dimID == "9" {
				tasks = []map[string]any{
					{"id": 1001, "name": "班会", "circleTypeId": 9256, "hours": 1.0, "circleTaskStatus": "上传期 未提交", "upPic": 1},
				}
			} else if dimID == "14" {
				tasks = []map[string]any{
					{"id": 1002, "name": "劳动", "circleTypeId": 9275, "hours": 2.0, "circleTaskStatus": "上传期 未提交", "upPic": 1},
				}
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(unifiedJSON(1, "成功", nil, tasks)))
		}
	}))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	tasks, err := c.FetchTasks(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("FetchTasks 失败: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("期望 2 个任务, 得到 %d", len(tasks))
	}
}

// ─── 测试: SubmitTask ───

func TestSubmitTask(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/addCircle" {
			t.Errorf("期望路径 addCircle, 得到 %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST, 得到 %s", r.Method)
		}

		var payload types.TaskSubmitPayload
		json.NewDecoder(r.Body).Decode(&payload)
		if payload.CircleTaskID != 1001 {
			t.Errorf("期望 CircleTaskID=1001, 得到 %d", payload.CircleTaskID)
		}
		if payload.Name != "班会" {
			t.Errorf("期望 Name=班会, 得到 %s", payload.Name)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(unifiedJSON(1, "提交成功", map[string]any{
			"insertID": 12345,
		}, nil)))
	}))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	result, err := c.SubmitTask(context.Background(), "test-token", types.TaskSubmitPayload{
		CircleTaskID: 1001,
		CircleTypeID: 9256,
		DimensionID:  9,
		Hours:        1.0,
		Name:         "班会",
		Address:      "高一八班",
		PlayRole:     "3",
	})
	if err != nil {
		t.Fatalf("SubmitTask 失败: %v", err)
	}
	if result.Code != 1 {
		t.Errorf("期望 Code=1, 得到 %d", result.Code)
	}
}

// ─── 测试: Self Evaluation ───

func TestSubmitSelfEvaluation(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentMoralEduNew/addSelfEvaluation" {
			t.Errorf("期望路径 addSelfEvaluation, 得到 %s", r.URL.Path)
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["studentComment"] != "很好的学期" {
			t.Errorf("期望 studentComment=很好的学期, 得到 %s", body["studentComment"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(unifiedJSON(1, "提交成功", nil, nil)))
	}))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	err := c.SubmitSelfEvaluation(context.Background(), "test-token", "很好的学期")
	if err != nil {
		t.Fatalf("SubmitSelfEvaluation 失败: %v", err)
	}
}

func TestQuerySelfEvaluation(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentMoralEduNew/querySelfEvaluation" {
			t.Errorf("期望路径 querySelfEvaluation, 得到 %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(unifiedJSON(1, "成功", map[string]any{
			"student_comment": "我表现很好",
			"teacher_comment": "继续努力",
			"student_name":    "张三",
			"class_name":      "高一八班",
		}, nil)))
	}))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	status, err := c.QuerySelfEvaluation(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("QuerySelfEvaluation 失败: %v", err)
	}
	if status.StudentComment != "我表现很好" {
		t.Errorf("期望 StudentComment=我表现很好, 得到 %s", status.StudentComment)
	}
	if status.TeacherComment != "继续努力" {
		t.Errorf("期望 TeacherComment=继续努力, 得到 %s", status.TeacherComment)
	}
}

// ─── 测试: File Upload ───

func TestUploadFile(t *testing.T) {
	upload := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/common/upload/uploadImage" {
			t.Errorf("期望路径 uploadImage, 得到 %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST, 得到 %s", r.Method)
		}

		// 验证 multipart 内容
		contentType := r.Header.Get("Content-Type")
		if !strings.Contains(contentType, "multipart/form-data") {
			t.Errorf("期望 multipart/form-data, 得到 %s", contentType)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(unifiedJSON(1, "上传成功", map[string]any{
			"id": 67890,
		}, nil)))
	}))
	defer upload.Close()

	c := newTestClient(nil, nil, upload)

	// 创建临时测试文件
	tmpfile := t.TempDir() + "/test-upload.jpg"
	if err := writeFile(tmpfile, []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	id, err := c.UploadFile(context.Background(), tmpfile)
	if err != nil {
		t.Fatalf("UploadFile 失败: %v", err)
	}
	if id != 67890 {
		t.Errorf("期望 id=67890, 得到 %d", id)
	}
}

// ─── 测试: 并发隔离 ───

func TestConcurrentLoginIsolation(t *testing.T) {
	// 验证多个 Client 实例的 cookie jar 相互独立
	var mu sync.Mutex
	counter := 0
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html>login</html>"))
		case "/teacher/auth/studentLogin/getSchoolIdByStudentNumber":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(unifiedJSON(1, "成功", nil, []map[string]any{
				{"school_id": "173", "NAME": "福清一中"},
			})))
		case "/kaptcha/kaptcha.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte{0xFF, 0xD8, 0xFF})
		case "/uiStudentLogin/validateCaptcha":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(unifiedJSON(1, "成功", nil, nil)))
		case "/teacher/auth/studentLogin/validate":
			mu.Lock()
			counter++
			token := "token-" + string(rune('A'+counter-1))
			mu.Unlock()
			w.Header().Set("Location", "/homepage?token="+token)
			w.WriteHeader(http.StatusFound)
		}
	}))
	defer sso.Close()

	// 并发创建 5 个 Client
	errs := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			c := newTestClient(sso, nil, nil)
			_, err := c.Login(context.Background(), types.LoginRequest{
				Username: "S1234567890",
				Password: "TestPass123",
				Captcha:  "AB12",
			})
			errs <- err
		}()
	}

	for i := 0; i < 5; i++ {
		if err := <-errs; err != nil {
			t.Errorf("并发登录失败: %v", err)
		}
	}
}

// ─── 辅助函数 ───

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}
