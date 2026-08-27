package client

import captchasdk "github.com/Wenaixi/nazhi-captcha-sdk"

// builtinCaptchaRecognizer 用 nazhi-captcha-sdk 内置预训练库做纯本地验证码识别。
// 零外部依赖、零网络 OCR 调用；未命中返回空串让 Login 换图重试。
type builtinCaptchaRecognizer struct{}

func newBuiltinCaptchaRecognizer() *builtinCaptchaRecognizer {
	return &builtinCaptchaRecognizer{}
}

// Recognize 对图片做纯本地查表识别。
// 命中返回 4 字符验证码；未命中返回 (空串, nil)（由 Login 的多图重试循环换图）。
func (b *builtinCaptchaRecognizer) Recognize(img []byte) (string, error) {
	if len(img) == 0 {
		return "", nil
	}
	code, ok := captchasdk.MatchImage(img)
	if !ok {
		return "", nil
	}
	return code, nil
}

// Close 无资源需释放（captcha-sdk 模板表是进程级共享单例）。
func (b *builtinCaptchaRecognizer) Close() error { return nil }
