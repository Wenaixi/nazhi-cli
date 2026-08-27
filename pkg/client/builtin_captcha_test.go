package client

import "testing"

func TestBuiltinCaptchaRecognizer_EmptyInput(t *testing.T) {
	r := newBuiltinCaptchaRecognizer()
	defer r.Close()
	text, err := r.Recognize(nil)
	if err != nil {
		t.Fatalf("空输入不应报错: %v", err)
	}
	if text != "" {
		t.Fatalf("空输入应返回空串，实际 %q", text)
	}
}

func TestBuiltinCaptchaRecognizer_InvalidImage(t *testing.T) {
	r := newBuiltinCaptchaRecognizer()
	defer r.Close()
	// 非法字节：MatchImage 返回 false → Recognize 返回 (空串, nil)
	text, err := r.Recognize([]byte("not-an-image"))
	if err != nil {
		t.Fatalf("非法图片不应报错: %v", err)
	}
	if text != "" {
		t.Fatalf("非法图片应返回空串，实际 %q", text)
	}
}

func TestBuiltinCaptchaRecognizer_ImplementsInterface(t *testing.T) {
	var _ CaptchaRecognizer = (*builtinCaptchaRecognizer)(nil)
}
