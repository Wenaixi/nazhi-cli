package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/Wenaixi/nazhi-cli/internal/ocr"
	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

func main() {
	c := client.New(client.WithTimeout(30 * time.Second))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 获取验证码图片
	fmt.Fprintln(os.Stderr, "=== 获取验证码 ===")
	base64Img, schoolID, err := c.FetchCaptcha(ctx, "S1234567890")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 获取验证码失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "schoolID: %s, 图片未解码\n", schoolID)

	// 解码
	imgBytes, _ := base64.StdEncoding.DecodeString(base64Img)
	fmt.Fprintf(os.Stderr, "图片大小: %d 字节\n", len(imgBytes))

	// OCR 识别
	fmt.Fprintln(os.Stderr, "\n=== OCR 识别 ===")
	ocrSvc := ocr.New()
	result, err := ocrSvc.Recognize(imgBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ OCR 识别失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✅ OCR 识别结果: %q\n", result)
	ocrSvc.Close()

	// 输出 Base64 图片到 stdout（方便人工对比）
	fmt.Println(base64Img)
	fmt.Fprintf(os.Stderr, "\n（Base64 图片已输出到 stdout，可解码后人工对比）\n")
}
