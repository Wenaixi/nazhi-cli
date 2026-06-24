// Package client_test 包含 nazhi-cli SDK 的全量测试。
package client_test

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
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

// ─── mock OCR ───

// mockOCR 返回固定验证码文本，用于测试。
type mockOCR struct{ text string }

func (m *mockOCR) Recognize(_ []byte) (string, error) { return m.text, nil }

// Close 是 captchaRecognizer 接口的占位实现, mock 无资源需释放。
func (m *mockOCR) Close() error { return nil }

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
	c, _ := client.New(opts...)
	return c
}

// newTestClientWithOCR 创建 Client 并注入 mock OCR。
func newTestClientWithOCR(sso *httptest.Server, mockText string, biz *httptest.Server) *client.Client {
	opts := []client.Option{
		client.WithSSOBase(sso.URL),
		client.WithTimeout(5 * time.Second),
		client.WithCustomOCR(&mockOCR{text: mockText}),
	}
	if biz != nil {
		opts = append(opts, client.WithBaseURL(biz.URL))
	}
	c, _ := client.New(opts...)
	return c
}

// warmupBizHandler 包装测试 handler，自动响应 ActivateSession 的 4 步预热路径。
// 业务方法（SubmitTask / GetMyInfo / QuerySelfEvaluation 等）现在会自动预热
// session，所以测试 mock server 必须先能响应 /、/getMenu、/getMyInfo。
//
// /getMyInfo 处理：第一次走预热响应（不返回 name，让 ActivateSession 走兜底
// 逻辑避免双重请求）；后续走 default fn（让 TestGetMyInfo 等需要实际 userInfo
// 的测试拿到自己的 mock 响应）。这是 sync.Once 保证的。
//
// 用法：
//
//	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w, r) {
//	    // 实际测试 path 的处理逻辑
//	})))
func warmupBizHandler(t *testing.T, fn http.HandlerFunc) http.HandlerFunc {
	t.Helper()
	var myInfoOnce sync.Once
	return func(w http.ResponseWriter, r *http.Request) {
		t.Logf("[mock] 收到请求: %s %s", r.Method, r.URL.Path)
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(1, "成功", nil, nil)))
		case "/api/studentInfo/getMyInfo":
			servedWarmup := false
			myInfoOnce.Do(func() {
				// 第一次响应：返回 userInfo 但 Name 为空，让 ActivateSession 不早 return
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(unifiedJSON(1, "成功", map[string]any{
					"student_no": "TEST2025001",
				}, nil)))
				servedWarmup = true
			})
			if !servedWarmup {
				// 后续请求：交给测试 handler（TestGetMyInfo 等需要真实 userInfo）
				fn(w, r)
			}
		default:
			fn(w, r)
		}
	}
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
	schoolID, schoolName, err := c.GetSchoolID(context.Background(), "TEST2025001")
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
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["username"] != "TEST2025001" {
				t.Errorf("期望 username=TEST2025001, 得到 %s", body["username"])
			}
			// HAR 验证：登录请求体无 captcha 字段
			if _, exists := body["captcha"]; exists {
				t.Errorf("登录请求体不应包含 captcha 字段（HAR 对齐）")
			}
			// HAR 验证：登录响应 200 JSON（而非 302 redirect）
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"code":1,"returnData":{"token":"eyJhbGciOiJIUzI1NiJ9.test-token-123","expires_at":1888888888}}`))
		}
	}))
	defer sso.Close()

	c := newTestClientWithOCR(sso, "AB12", nil)
	resp, err := c.Login(context.Background(), types.LoginRequest{
		Username: "TEST2025001",
		Password: "TestPass123",
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
			// 200 OK 但 code=0 表示凭证错误
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(unifiedJSON(0, "用户名或密码错误", nil, nil)))
		}
	}))
	defer sso.Close()

	c := newTestClientWithOCR(sso, "AB12", nil)
	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "TEST2025001",
		Password: "wrong",
	})
	if err == nil {
		t.Fatal("期望登录失败，但得到 nil error")
	}
	if !strings.Contains(err.Error(), "login rejected") {
		t.Errorf("期望错误包含 'login rejected', 得到 %v", err)
	}
}

// ─── 测试: ActivateSession ───

// TestActivateSession 验证 4 步 HAR 对齐的 Session 激活流程：
//  1. GET /
//  2. GET /api/studentInfo/getMenu (Referer: /homepage?token=xxx)
//  3. GET /api/studentInfo/getMenu (Referer: /home)
//  4. GET /api/studentInfo/getMyInfo
func TestActivateSession(t *testing.T) {
	callOrder := 0
	getMenuCount := 0
	biz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			if callOrder != 0 {
				t.Errorf("首页必须在最前, 当前 callOrder=%d", callOrder)
			}
			callOrder = 1
			if r.Header.Get("X-Auth-Token") != "test-token" {
				t.Errorf("期望 X-Auth-Token=test-token")
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html>home</html>"))
		case "/api/studentInfo/getMenu":
			getMenuCount++
			if callOrder == 0 {
				t.Errorf("getMenu 必须在首页之后")
			}
			if getMenuCount == 1 {
				// 步骤 2：Referer 应包含 /homepage?token=
				ref := r.Header.Get("Referer")
				if !strings.Contains(ref, "/homepage?token=") {
					t.Errorf("步骤 2 getMenu Referer 应包含 /homepage?token=, 得到 %s", ref)
				}
			} else if getMenuCount == 2 {
				// 步骤 3：Referer 应是 /home
				ref := r.Header.Get("Referer")
				if !strings.Contains(ref, "/home") {
					t.Errorf("步骤 3 getMenu Referer 应包含 /home, 得到 %s", ref)
				}
			}
			callOrder = 2
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(unifiedJSON(1, "成功", map[string]any{
				"name": "张三", "studentNumber": "TEST2025001",
			}, nil)))
		case "/api/studentInfo/getMyInfo":
			// 步骤 4：getMyInfo 在 getMenu 之后
			if getMenuCount != 2 {
				t.Errorf("getMyInfo 应在两次 getMenu 之后, 当前 getMenuCount=%d", getMenuCount)
			}
			callOrder = 3
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(unifiedJSON(1, "成功", map[string]any{
				"name":          "张三",
				"studentNumber": "TEST2025001",
				"schoolName":    "福清一中",
				"gradeName":     "高一",
				"className":     "八班",
				"seat":          45,
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
	if getMenuCount != 2 {
		t.Errorf("期望 getMenu 被调用 2 次 (HAR 4 步激活), 得到 %d", getMenuCount)
	}
	if callOrder != 3 {
		t.Errorf("期望 callOrder=3 (4 步全部完成), 得到 %d", callOrder)
	}
}

// ─── 测试: GetMyInfo ───

func TestGetMyInfo(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentInfo/getMyInfo" {
			t.Errorf("期望路径 getMyInfo, 得到 %s", r.URL.Path)
		}
		if r.Header.Get("Referer") == "" {
			t.Errorf("期望 Referer 不为空")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(unifiedJSON(1, "成功", map[string]any{
			"name":          "张三",
			"studentNumber": "TEST2025001",
			"schoolName":    "福清一中",
			"gradeName":     "高一",
			"className":     "八班",
			"seat":          45,
		}, nil)))
	})))
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
	if info.Seat != 45 {
		t.Errorf("期望 Seat=45, 得到 %d", info.Seat)
	}
}

// ─── 测试: GetMyInfo 完整字段（HAR 实测响应）───

// TestGetMyInfo_FullFields 用真实 HAR 抓包的完整响应验证 30+ 字段全部正确解析。
func TestGetMyInfo_FullFields(t *testing.T) {
	// 完整 mock 响应模拟真实账号（张三）的 getMyInfo 返回
	mockResponse := map[string]any{
		"id":                    32USER_ID_REDACTED,
		"name":                  "张三",
		"initials":              "gNAME_INITIALS_REDACTED",
		"pinyin":                "gaNAME_PINYIN_REDACTED",
		"studentNumber":         "TEST2025001",
		"studentId":             38STUDENT_ID_REDACTED,
		"schoolId":              173,
		"positionId":            0,
		"positionName":          nil,
		"email":                 "",
		"status":                1,
		"statusName":            "在籍",
		"nativePlace":           "",
		"idType":                1,
		"idCard":                "ID_CARD_REDACTED",
		"birthday":              []int{2009, 12, 11},
		"birthdayStr":           "2009-12-11 00:00:00",
		"nation":                1,
		"seat":                  29,
		"seatSort":              -1,
		"gender":                1,
		"genderName":            "男",
		"creator":               28,
		"creationTime":          []int{2025, 10, 9, 10, 32, 6},
		"creationTimeStr":       "2025-10-09 10:32:06",
		"modifier":              28,
		"modifyTime":            []int{2026, 2, 6, 10, 16, 15},
		"modifyTimeStr":         "2026-02-06 10:16:15",
		"admissionDate":         []int{2025, 9, 1},
		"admissionDateStr":      nil,
		"currentAddress":        "",
		"contactAddress":        "",
		"familyAddress":         "",
		"youthLeagueFlag":       1,
		"hobbies":               "",
		"criminalRecordFlag":    0,
		"nationalStudentNumber": "TEST2025001",
		"registrationNumber":    "",
		"studyNumber":           "STUDY_NUMBER_REDACTEDSTUDY_NUMBER_FRAGMENT_REDACTED",
		"photoAttachmentId":     nil,
		"telephone":             "",
		"level":                 0,
		"gradeId":               27900,
		"gradeName":             "高一",
		"classId":               162647,
		"className":             "高一八班",
		"studentUuid":           nil,
		"schoolName":            nil,
	}

	biz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(unifiedJSON(1, "成功", mockResponse, nil)))
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

	// 基础身份
	if info.ID != 32USER_ID_REDACTED || info.Name != "张三" || info.StudentNumber != "TEST2025001" {
		t.Errorf("基础身份字段错误: id=%d name=%s studentNumber=%s", info.ID, info.Name, info.StudentNumber)
	}
	if info.Initials != "gNAME_INITIALS_REDACTED" || info.Pinyin != "gaNAME_PINYIN_REDACTED" {
		t.Errorf("姓名辅助字段错误: initials=%s pinyin=%s", info.Initials, info.Pinyin)
	}
	if info.StudentID != 38STUDENT_ID_REDACTED || info.StudyNumber != "STUDY_NUMBER_REDACTEDSTUDY_NUMBER_FRAGMENT_REDACTED" {
		t.Errorf("学生 ID 错误: studentId=%d studyNumber=%s", info.StudentID, info.StudyNumber)
	}
	if info.NationalStudentNumber != "TEST2025001" {
		t.Errorf("全国学号错误: %s", info.NationalStudentNumber)
	}

	// 学校 / 班级
	if info.SchoolID != 173 || info.GradeID != 27900 || info.GradeName != "高一" {
		t.Errorf("学校年级错误: schoolId=%d gradeId=%d gradeName=%s", info.SchoolID, info.GradeID, info.GradeName)
	}
	if info.ClassID != 162647 || info.ClassName != "高一八班" {
		t.Errorf("班级错误: classId=%d className=%s", info.ClassID, info.ClassName)
	}
	if info.SchoolName != "" {
		t.Errorf("schoolName 应该是空字符串（平台返回 null）, 得到 %q", info.SchoolName)
	}

	// 座号
	if info.Seat != 29 || info.SeatSort != -1 {
		t.Errorf("座号错误: seat=%d seatSort=%d", info.Seat, info.SeatSort)
	}

	// 性别
	if info.Gender != 1 || info.GenderName != "男" {
		t.Errorf("性别错误: gender=%d genderName=%s", info.Gender, info.GenderName)
	}

	// 民族 / 证件
	if info.Nation != 1 || info.IDCard != "ID_CARD_REDACTED" || info.IDType != 1 {
		t.Errorf("民族证件错误: nation=%d idCard=%s idType=%d", info.Nation, info.IDCard, info.IDType)
	}

	// 生日
	if info.Birthday != "2009-12-11 00:00:00" {
		t.Errorf("生日错误: %q", info.Birthday)
	}

	// 学籍状态
	if info.Status != 1 || info.StatusName != "在籍" {
		t.Errorf("学籍错误: status=%d statusName=%s", info.Status, info.StatusName)
	}
	if info.YouthLeagueFlag != 1 || info.CriminalRecordFlag != 0 {
		t.Errorf("标志位错误: youthLeagueFlag=%d criminalRecordFlag=%d", info.YouthLeagueFlag, info.CriminalRecordFlag)
	}

	// 时间戳（数组）
	if len(info.CreationTime) != 6 || info.CreationTime[0] != 2025 {
		t.Errorf("creationTime 错误: %v", info.CreationTime)
	}
	if len(info.ModifyTime) != 6 || info.ModifyTime[5] != 15 {
		t.Errorf("modifyTime 错误: %v", info.ModifyTime)
	}
	if len(info.AdmissionDate) != 3 || info.AdmissionDate[1] != 9 {
		t.Errorf("admissionDate 错误: %v", info.AdmissionDate)
	}

	// 时间戳（字符串）
	if info.CreationTimeStr != "2025-10-09 10:32:06" {
		t.Errorf("creationTimeStr 错误: %s", info.CreationTimeStr)
	}
	if info.ModifyTimeStr != "2026-02-06 10:16:15" {
		t.Errorf("modifyTimeStr 错误: %s", info.ModifyTimeStr)
	}
	if info.AdmissionDateStr != "" {
		t.Errorf("admissionDateStr 应为空（平台返回 null）, 得到 %q", info.AdmissionDateStr)
	}

	// nullable 字段
	if info.PhotoAttachmentID != 0 {
		t.Errorf("photoAttachmentId 应为 0（平台返回 null）, 得到 %d", info.PhotoAttachmentID)
	}
	if info.StudentUUID != "" {
		t.Errorf("studentUuid 应为空（平台返回 null）, 得到 %q", info.StudentUUID)
	}
	if info.PositionID != 0 || info.PositionName != "" {
		t.Errorf("职位应为零值, 得到 positionId=%d positionName=%q", info.PositionID, info.PositionName)
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
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/addCircle" {
			t.Errorf("期望路径 addCircle, 得到 %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST, 得到 %s", r.Method)
		}
		var payload types.TaskSubmitPayload
		_ = json.NewDecoder(r.Body).Decode(&payload)
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
	})))
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

// ─── 自我评价 ───

func TestSubmitSelfEvaluation(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentMoralEduNew/addSelfEvaluation" {
			t.Errorf("期望路径 addSelfEvaluation, 得到 %s", r.URL.Path)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["studentComment"] != "很好的学期" {
			t.Errorf("期望 studentComment=很好的学期, 得到 %s", body["studentComment"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(unifiedJSON(1, "提交成功", nil, nil)))
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	err := c.SubmitSelfEvaluation(context.Background(), "test-token", "很好的学期")
	if err != nil {
		t.Fatalf("SubmitSelfEvaluation 失败: %v", err)
	}
}

func TestQuerySelfEvaluation(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
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
	})))
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

// ─── 文件上传 ───

// TestUploadFile_RealImage 验证 UploadFile 集成路径：PNG → 压缩 → 上传。
func TestUploadFile_RealImage(t *testing.T) {
	upload := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/common/upload/uploadImage" {
			t.Errorf("期望路径 uploadImage, 得到 %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST, 得到 %s", r.Method)
		}
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
	// 创建一个真实的 100×100 红色 PNG（PNG → JPG 转换路径）
	tmpfile := t.TempDir() + "/test-upload.png"
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	red := color.RGBA{255, 0, 0, 255}
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, red)
		}
	}
	f, err := os.Create(tmpfile)
	if err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("编码 PNG 失败: %v", err)
	}
	f.Close()

	id, err := c.UploadFile(context.Background(), tmpfile)
	if err != nil {
		t.Fatalf("UploadFile 失败: %v", err)
	}
	if id != 67890 {
		t.Errorf("期望 id=67890, 得到 %d", id)
	}
}

// ─── 并发 ───

// TestConcurrentLoginsSucceed 验证 5 个 goroutine 并发 Login 各自成功。
// 注：仅验证并发安全不崩溃，不验证 cookie jar 隔离（jar 隔离需额外断言，
// 见 TestCookieJarIsolation 相关文档）。
func TestConcurrentLoginsSucceed(t *testing.T) {
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
			// 200 JSON 响应（HAR 对齐）
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"code":1,"returnData":{"token":"` + token + `","expires_at":1888888888}}`))
		}
	}))
	defer sso.Close()

	errs := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			c := newTestClientWithOCR(sso, "AB12", nil)
			_, err := c.Login(context.Background(), types.LoginRequest{
				Username: "TEST2025001",
				Password: "TestPass123",
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
