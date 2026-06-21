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
var fileUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "上传图片到文件服务器",
	Long:  `上传图片到文件服务器。不需要 Token，文件服务器独立。只支持图片文件。`,
	Example: `  nazhi file upload -f ./photo.jpg
  nazhi file upload -f ./photo.jpg --upload-url http://doc.nazhisoft.com`,
	Run: func(cmd *cobra.Command, args []string) {
		filePath, _ := cmd.Flags().GetString("file")
		uploadURL, _ := cmd.Flags().GetString("upload-url")
		timeoutSec, _ := cmd.Flags().GetInt("timeout")

		if filePath == "" {
			printError(fmt.Errorf("--file 为必填"))
			return
		}

		opts := []client.Option{client.WithTimeout(time.Duration(timeoutSec) * time.Second)}
		if uploadURL != "" {
			opts = append(opts, client.WithUploadURL(uploadURL))
		}
		c := client.New(opts...)

		printVerbose("正在上传文件...")
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
	fileUploadCmd.Flags().String("upload-url", "", "上传服务器地址（默认 http://doc.nazhisoft.com）")
	fileUploadCmd.Flags().Int("timeout", 30, "HTTP 超时（秒）")
}
