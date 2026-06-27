package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// testBizHandlerDoBiz 为 doBizAndDecode 测试创建 mock biz server，
// 自动处理 session 预热 4 步路径（/、/api/studentInfo/getMenu、/api/studentInfo/getMyInfo）。
//
// testHandler 接收非预热路径的请求，由测试用例自行断言。
func testBizHandlerDoBiz(t *testing.T, testHandler func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>home</html>"))
		case "/api/studentInfo/getMenu":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
		default:
			testHandler(w, r)
		}
	}
}

func TestDoBizAndDecode_Success(t *testing.T) {
	biz := httptest.NewServer(testBizHandlerDoBiz(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/test" {
			t.Errorf("期望路径 /api/test, 得到 %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("期望 GET, 得到 %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"returnData":{"key":"value"}}`))
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	resp, err := c.doBizAndDecode(context.Background(), "test-token", "TestOp", "/api/test", http.MethodGet, nil)
	if err != nil {
		t.Fatalf("doBizAndDecode 失败: %v", err)
	}
	if resp.Code != 1 {
		t.Errorf("期望 Code=1, 得到 %d", resp.Code)
	}
	if resp.ReturnData == nil {
		t.Error("期望 ReturnData 不为 nil")
	}
}

func TestDoBizAndDecode_BusinessError(t *testing.T) {
	biz := httptest.NewServer(testBizHandlerDoBiz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0,"msg":"业务拒绝"}`))
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	_, err = c.doBizAndDecode(context.Background(), "test-token", "TestOp", "/api/test", http.MethodGet, nil)
	if err == nil {
		t.Fatal("期望业务错误，但得到 nil")
	}
	if !errors.Is(err, ErrBusinessRejected) {
		t.Errorf("期望 errors.Is(err, ErrBusinessRejected)=true, 得到 %v", err)
	}
}

func TestDoBizAndDecode_BadJSON(t *testing.T) {
	biz := httptest.NewServer(testBizHandlerDoBiz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`这不是 JSON`))
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	_, err = c.doBizAndDecode(context.Background(), "test-token", "TestOp", "/api/test", http.MethodGet, nil)
	if err == nil {
		t.Fatal("期望解析错误，但得到 nil")
	}
}

func TestDoBizAndDecode_POST(t *testing.T) {
	biz := httptest.NewServer(testBizHandlerDoBiz(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/submit" {
			t.Errorf("期望路径 /api/submit, 得到 %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST, 得到 %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"msg":"提交成功"}`))
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	resp, err := c.doBizAndDecode(context.Background(), "test-token", "SubmitOp", "/api/submit", http.MethodPost, map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("doBizAndDecode(POST) 失败: %v", err)
	}
	if resp.Code != 1 {
		t.Errorf("期望 Code=1, 得到 %d", resp.Code)
	}
}

// TestDoBizAndDecode_CheckCodeResult 验证返回的 UnifiedResponse 中的字段可直接用于 fallback 解析。
func TestDoBizAndDecode_CheckCodeResult(t *testing.T) {
	biz := httptest.NewServer(testBizHandlerDoBiz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"dataList":[{"id":9,"name":"思想品德"}]}`))
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	resp, err := c.doBizAndDecode(context.Background(), "test-token", "TestOp", "/api/test", http.MethodGet, nil)
	if err != nil {
		t.Fatalf("doBizAndDecode 失败: %v", err)
	}

	// 验证返回的 UnifiedResponse 可以被 DecodeDataList 消费
	dims, err := types.DecodeDataList[types.Dimension](*resp)
	if err != nil {
		t.Fatalf("DecodeDataList 失败: %v", err)
	}
	if len(dims) != 1 {
		t.Fatalf("期望 1 个维度, 得到 %d", len(dims))
	}
	if dims[0].ID != 9 || dims[0].Name != "思想品德" {
		t.Errorf("期望维度 id=9 name=思想品德, 得到 id=%d name=%s", dims[0].ID, dims[0].Name)
	}
}

// testGetDecodeUser 用于 doBizGetDecode 测试中的类型安全解码验证。
type testGetDecodeUser struct {
	Name string `json:"name"`
}

func TestDoBizGetDecode_Success(t *testing.T) {
	biz := httptest.NewServer(testBizHandlerDoBiz(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/test" {
			t.Errorf("期望路径 /api/test, 得到 %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("期望 GET, 得到 %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三"}}`))
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	result, err := doBizGetDecode[testGetDecodeUser](c, context.Background(), "test-token", "TestOp", "/api/test",
		types.DecodeReturnData[testGetDecodeUser],
	)
	if err != nil {
		t.Fatalf("doBizGetDecode 失败: %v", err)
	}
	if result == nil {
		t.Fatal("期望非 nil 结果")
	}
	if result.Name != "张三" {
		t.Errorf("期望 Name=张三, 得到 %s", result.Name)
	}
}

func TestDoBizGetDecode_Success_DataList(t *testing.T) {
	biz := httptest.NewServer(testBizHandlerDoBiz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"dataList":[{"id":9,"name":"思想品德"}]}`))
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	dataListFallback := func(resp types.UnifiedResponse) (*[]types.Dimension, error) {
		v, err := types.DecodeDataList[types.Dimension](resp)
		if err != nil {
			return nil, err
		}
		return &v, nil
	}

	result, err := doBizGetDecode[[]types.Dimension](c, context.Background(), "test-token", "TestOp", "/api/test",
		dataListFallback,
	)
	if err != nil {
		t.Fatalf("doBizGetDecode 失败: %v", err)
	}
	if result == nil {
		t.Fatal("期望非 nil 结果")
	}
	if len(*result) != 1 {
		t.Fatalf("期望 1 个维度, 得到 %d", len(*result))
	}
	if (*result)[0].ID != 9 || (*result)[0].Name != "思想品德" {
		t.Errorf("期望 id=9 name=思想品德, 得到 id=%d name=%s", (*result)[0].ID, (*result)[0].Name)
	}
}

func TestDoBizGetDecode_BusinessError(t *testing.T) {
	biz := httptest.NewServer(testBizHandlerDoBiz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0,"msg":"业务拒绝"}`))
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	_, err = doBizGetDecode[testGetDecodeUser](c, context.Background(), "test-token", "TestOp", "/api/test",
		types.DecodeReturnData[testGetDecodeUser],
	)
	if err == nil {
		t.Fatal("期望业务错误，但得到 nil")
	}
	if !errors.Is(err, ErrBusinessRejected) {
		t.Errorf("期望 errors.Is(err, ErrBusinessRejected)=true, 得到 %v", err)
	}
}

func TestDoBizGetDecode_BadJSON(t *testing.T) {
	biz := httptest.NewServer(testBizHandlerDoBiz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`这不是 JSON`))
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	_, err = doBizGetDecode[testGetDecodeUser](c, context.Background(), "test-token", "TestOp", "/api/test",
		types.DecodeReturnData[testGetDecodeUser],
	)
	if err == nil {
		t.Fatal("期望解析错误，但得到 nil")
	}
}

func TestDoBizGetDecode_AllFallbacksFail(t *testing.T) {
	biz := httptest.NewServer(testBizHandlerDoBiz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1}`)) // 空响应，无 returnData/dataList/dataMap
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	result, err := doBizGetDecode[testGetDecodeUser](c, context.Background(), "test-token", "TestOp", "/api/test",
		types.DecodeReturnData[testGetDecodeUser], // returnData 为 nil → (nil, nil)
	)
	if err == nil {
		t.Fatal("期望错误，但得到 nil")
	}
	if result != nil {
		t.Errorf("期望 nil 结果，但得到 %v", result)
	}
}

func TestDoBizGetDecode_SecondFallbackSucceeds(t *testing.T) {
	biz := httptest.NewServer(testBizHandlerDoBiz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// returnData 类型不匹配导致 DecodeReturnData 失败，回退到 dataMap
		_, _ = w.Write([]byte(`{"code":1,"returnData":"not an object","dataMap":{"name":"李四"}}`))
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	result, err := doBizGetDecode[testGetDecodeUser](c, context.Background(), "test-token", "TestOp", "/api/test",
		types.DecodeReturnData[testGetDecodeUser], // returnData="not an object" → 解码失败
		types.DecodeDataMap[testGetDecodeUser],    // dataMap 有数据 → 成功
	)
	if err != nil {
		t.Fatalf("doBizGetDecode 失败: %v", err)
	}
	if result == nil {
		t.Fatal("期望非 nil 结果")
	}
	if result.Name != "李四" {
		t.Errorf("期望 Name=李四, 得到 %s", result.Name)
	}
}
