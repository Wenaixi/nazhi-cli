package types

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestAddHonorPayload_CertImgWireFormat 锁定证书图附件 ID 的 wire 形态：
// 前端上传成功后发裸 number（performanceM.vue:576 赋 returnData.id），
// 无图时发 "" 空串（form 初始值 :219）——服务端双兼容已由前端长期验证。
// SDK 对齐：有图发裸数字、无图省略键（omitempty），入站继续兼容 string/number/空串。
func TestAddHonorPayload_CertImgWireFormat(t *testing.T) {
	// 出站：有附件 → 裸数字
	b, _ := json.Marshal(&AddHonorPayload{CertImgAttachmentID: 4383235})
	if !strings.Contains(string(b), `"certImgAttachmentId":4383235`) {
		t.Fatalf("有附件应输出裸数字，实际 %s", b)
	}
	// 出站：无附件 → 键省略
	b0, _ := json.Marshal(&AddHonorPayload{})
	if strings.Contains(string(b0), "certImgAttachmentId") {
		t.Fatalf("空附件应省略键，实际 %s", b0)
	}
	// 入站兼容：number / 数字字符串 / 空串 / null
	var p1 AddHonorPayload
	if err := json.Unmarshal([]byte(`{"certImgAttachmentId":4383235}`), &p1); err != nil || p1.CertImgAttachmentID != 4383235 {
		t.Fatalf("number 入站失败: err=%v val=%v", err, p1.CertImgAttachmentID)
	}
	var p2 AddHonorPayload
	if err := json.Unmarshal([]byte(`{"certImgAttachmentId":"4383235"}`), &p2); err != nil || p2.CertImgAttachmentID != 4383235 {
		t.Fatalf("字符串入站失败: err=%v val=%v", err, p2.CertImgAttachmentID)
	}
	var p3 AddHonorPayload
	if err := json.Unmarshal([]byte(`{"certImgAttachmentId":""}`), &p3); err != nil || p3.CertImgAttachmentID != 0 {
		t.Fatalf("空串入站失败: err=%v val=%v", err, p3.CertImgAttachmentID)
	}
}
