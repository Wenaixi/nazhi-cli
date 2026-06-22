package main

import (
	"fmt"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// fileUploadCmd 表示 nazhi file upload 命令
//
//	nazhi file upload -f <path> [--upload-url <url>] [--timeout <秒>]
//
// ⚠️ 本命令不接受 --token 参数。文件服务器独立，发送 token 反而可能被风控。
var fileUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "上传图片到文件服务器",
	Long: `上传图片到文件服务器。

注意：本命令不接受 --token 参数。
文件上传服务器（doc.nazhisoft.com）是独立公共服务，不需要业务域鉴权。
SDK 内部会主动清除 Authorization / X-Auth-Token / Cookie 三个 Header。`,
	Example: `  nazhi file upload -f ./photo.jpg
  nazhi file upload -f ./photo.jpg --upload-url http://doc.nazhisoft.com`,
	Run: func(cmd *cobra.Command, args []string) {
		filePath, _ := cmd.Flags().GetString("file")
		uploadURL, _ := cmd.Flags().GetString("upload-url")
		timeoutSec, _ := cmd.Flags().GetInt("timeout")

		// 环境变量 fallback
		if uploadURL == "" {
			uploadURL = envString("NAZHI_UPLOAD_URL", "")
		}
		if timeoutSec == 30 {
			timeoutSec = envInt("NAZHI_TIMEOUT", 30)
		}

		if filePath == "" {
			printError(fmt.Errorf("--file 为必填"))
			return
		}

		opts := []client.Option{client.WithTimeout(time.Duration(timeoutSec) * time.Second)}
		if uploadURL != "" {
			opts = append(opts, client.WithUploadURL(uploadURL))
		}
		c := client.New(opts...)

		printVerbose("正在上传文件（无 token 模式）...")
		id, err := c.UploadFile(cmd.Context(), filePath)
		if err != nil {
			printError(fmt.Errorf("上传文件失败: %w", err))
			return
		}

		printJSON(map[string]any{
			"id":   id,
			"path": filePath,
		})
	},
}

func init() {
	fileUploadCmd.Flags().StringP("file", "f", "", "本地图片路径（必填）")
	fileUploadCmd.Flags().String("upload-url", "", "上传服务器地址（默认 http://doc.nazhisoft.com）也可通过 NAZHI_UPLOAD_URL 设置")
	fileUploadCmd.Flags().Int("timeout", 30, "HTTP 超时（秒）也可通过 NAZHI_TIMEOUT 设置")
	// 显式不提供 --token flag（文件服务器独立，不需要业务域 Token）
}
