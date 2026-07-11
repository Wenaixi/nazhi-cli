//go:build ignore

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

func main() {
	c, err := client.New(
		client.WithSSOBase("https://www.nazhisoft.com"),
		client.WithTimeout(15*time.Second),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Client 初始化失败：%v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	ctx := context.Background()

	// 直接看原始 HTTP 响应
	u := "https://www.nazhisoft.com/teacher/auth/studentLogin/getSchoolIdByStudentNumber?userName=G350181200912110035"

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader([]byte(`{"key":""}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.nazhisoft.com/uiStudentLogin/login?userName=G350181200912110035")
	req.Header.Set("Origin", "https://www.nazhisoft.com")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "请求失败：%v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	// 美化输出
	var pretty bytes.Buffer
	json.Indent(&pretty, body, "", "  ")
	fmt.Println(pretty.String())

	// 同时也打出 SDK 解析后的值
	info, err := c.GetSchoolID(ctx, "G350181200912110035")
	sdkResult := map[string]any{
		"sdK_schoolID":   "",
		"sdK_schoolName": "",
		"sdK_error":      nil,
	}
	if err != nil {
		sdkResult["sdK_error"] = err.Error()
	} else {
		sdkResult["sdK_schoolID"] = info.SchoolID
		sdkResult["sdK_schoolName"] = info.SchoolName
	}
	fmt.Println("--- SDK parsed ---")
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(sdkResult)
}
