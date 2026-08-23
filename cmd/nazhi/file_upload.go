package main

import (
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
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
SDK 内部不产生任何鉴权头（独立 clean http.Client，无 cookie jar，不携带 Authorization / X-Auth-Token / Cookie）。`,
	Example: `  nazhi file upload -f ./photo.jpg
  nazhi file upload -f ./photo.jpg --upload-url http://doc.nazhisoft.com`,
	Run: func(cmd *cobra.Command, args []string) {
		filePath, _ := cmd.Flags().GetString("file")
		if filePath == "" {
			printEnvelope(envelope.Error(400, "--file 为必填"))
			return
		}

		// 消除 inline client.New + 自动获得 trackClient。
		// urlType="upload" 走 --upload-url flag + NAZHI_UPLOAD_URL env。
		c, err := buildClient(cmd, "upload", "NAZHI_TIMEOUT")
		if err != nil {
			printError(fmt.Errorf("构造 Client 失败: %w", err))
			return
		}

		printVerbose("正在上传文件（无 token 模式）...")
		result, err := c.UploadFile(cmd.Context(), filePath)
		if err != nil {
			printError(fmt.Errorf("上传文件失败: %w", err))
			return
		}

		// envelope.data 直接传 SDK UploadFile 返回对象（attachmentID）。
		// 与 SDK 返回值 1:1 —— 不在 CLI 侧加 path 等额外字段，避免伪装成 SDK 输出。
		printEnvelope(envelope.Success(result))
	},
}

func init() {
	fileUploadCmd.Flags().StringP("file", "f", "", "本地图片路径（必填）")
	fileUploadCmd.Flags().String("upload-url", "", "上传服务器地址（默认 http://doc.nazhisoft.com）也可通过 NAZHI_UPLOAD_URL 设置")
	fileUploadCmd.Flags().Int("timeout", 30, "HTTP 超时（秒）也可通过 NAZHI_TIMEOUT 设置")
	// 显式不提供 --token flag（文件服务器独立，不需要业务域 Token）
}
