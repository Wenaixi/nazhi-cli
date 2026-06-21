package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

func main() {
	c := client.New(client.WithTimeout(15 * time.Second))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	base64Img, schoolID, err := c.FetchCaptcha(ctx, "S1234567890")
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取验证码失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "schoolID: %s\n", schoolID)

	imgBytes, err := base64.StdEncoding.DecodeString(base64Img)
	if err != nil {
		fmt.Fprintf(os.Stderr, "解码失败: %v\n", err)
		os.Exit(1)
	}

	err = os.WriteFile("captcha.jpg", imgBytes, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "写文件失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "captcha.jpg 已保存 (%d 字节)\n", len(imgBytes))
	fmt.Println(base64Img)
}
