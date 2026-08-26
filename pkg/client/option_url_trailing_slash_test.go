package client

import (
	"strings"
	"testing"
)

// TestURLOptions_TrailingSlashNormalized 锁定 URL Option 尾斜杠规范化契约：
// 用户传 --upload-url http://localhost:8080/ 这类带尾斜杠的地址时，
// 拼接点（uploadURL + "/common/..."、ssoBaseURL + "/common/..."）不得产生
// //common/... 双斜杠路径——个别 nginx merge_slashes 关闭配置下会 404 且难排查。
func TestURLOptions_TrailingSlashNormalized(t *testing.T) {
	c, err := New(
		WithSSOBase("https://example.com/"),
		WithBaseURL("http://api.example.com/"),
		WithUploadURL("http://upload.example.com//"),
	)
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	for name, got := range map[string]string{
		"ssoBaseURL": c.ssoBaseURL,
		"baseURL":    c.baseURL,
		"uploadURL":  c.uploadURL,
	} {
		if strings.HasSuffix(got, "/") {
			t.Errorf("%s 尾斜杠未规范化: %q", name, got)
		}
	}
}
