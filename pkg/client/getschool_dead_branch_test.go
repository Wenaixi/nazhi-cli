// Package client_test 外部黑盒测试。
// M1: 删除 GetSchoolID 死分支 school_name 兜底。
// 历史 bug（auth.go:64-68）：
//
//	school := schools[0]
//	schoolID = fmt.Sprintf("%v", school["school_id"])
//	if v, ok := school["NAME"]; ok {
//	 schoolName = fmt.Sprintf("%v", v)
//	} else if v, ok := school["school_name"]; ok {
//	 schoolName = fmt.Sprintf("%v", v)
//	}
//
// 死分支分析：
// - HAR fixture (Wenaixi/nazhi-cli 真实抓包) dataList[0] 键是 school_id + NAME
// - 真实平台响应键也是 NAME（不是 school_name）
// - 学校名称字段历史上一直是 NAME，从未出现过 school_name
// - else if 死分支从未被触发，但代码还在那里误导后来者
// 修复后：只保留 NAME 一个分支。
// 验证策略：
// - server 只返 school_name 时，schoolName 应为空（不再兜底）
// - 现有 TestGetSchoolID (client_test.go) server 返 NAME 仍然 PASS
package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestGetSchoolID_NoLongerFallsBackToSchoolName 验证 GetSchoolID 不再
// 走 school_name 兜底分支。HAR fixture + 真实平台都只返 NAME 键，school_name
// 历史上从未出现过，保留 else if 死分支是误导。
// 场景：server 返 dataList[0] = {school_id: "173", school_name: "某学校"}，
// 期望 schoolName == ""（死分支删除，不再兜底）。
func TestGetSchoolID_NoLongerFallsBackToSchoolName(t *testing.T) {
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// 只返 school_name，无 NAME 键
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功","dataList":[{"school_id":"173","school_name":"某学校"}]}`))
	}))
	defer sso.Close()

	c, err := client.New(
		client.WithSSOBase(sso.URL),
		client.WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("client.New 失败: %v", err)
	}
	defer func() { _ = c.Close() }()

	schoolID, schoolName, err := c.GetSchoolID(context.Background(), "TEST2025001")
	if err != nil {
		t.Fatalf("GetSchoolID 失败: %v", err)
	}
	if schoolID != "173" {
		t.Errorf("期望 schoolID=173，得到 %s", schoolID)
	}
	// M1 关键断言：school_name 兜底删除，schoolName 应为空字符串
	if schoolName != "" {
		t.Errorf("期望 schoolName=\"\"（school_name 死分支已删除），实际: %q", schoolName)
	}
}
