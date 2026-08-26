package main

import (
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// fileDownloadCmd 表示 nazhi file download 命令
//
//	nazhi file download --id <附件ID> --output <本地路径>
//
// 本命令不接受 --token 参数。文件下载服务器独立，发送 token 反而可能被风控。
var fileDownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "按附件 ID 从公开服务器下载图片",
	Long: `按附件 ID 从公开服务器下载图片。

附件 ID 通常来自以下途径：
  - task submitted 返回的 imgList[].attachment_id
  - file upload 返回的 id

URL 流程：
  GET https://www.nazhisoft.com/common/attachment/getImg?id=<ID>
    ↓ 302 重定向
  GET http://doc.nazhisoft.com/other/M00/.../<image>.jpg

本命令不发送任何鉴权头（公开服务），跟随重定向时仅允许 nazhisoft.com 同域。
`,
	Example: `  # 下载附件 ID 5006375 到当前目录
  nazhi file download --id 5006375 --output ./photo.jpg

  # 配合 task submitted 使用：jq 提取 attachment_id 后批量下载
  # （全量模式 data 为裸数组；--limit 模式 data 带 records 包装，路径为 .data.records[].imgList[].attachment_id）
  nazhi task submitted | jq -r '.data[]?.imgList[]?.attachment_id // .data.records[]?.imgList[]?.attachment_id' | \
    xargs -I {} nazhi file download --id {} --output ./img_{}.jpg`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		id, _ := cmd.Flags().GetInt64("id")
		output, _ := cmd.Flags().GetString("output")

		if id <= 0 {
			printEnvelope(envelope.Error(400, "--id 为必填且必须 > 0"))
			return
		}
		if output == "" {
			printEnvelope(envelope.Error(400, "--output 为必填"))
			return
		}

		// urlType="sso" 走 --sso-base flag + NAZHI_SSO_BASE env
		// （下载入口在 SSO 域名下 /common/attachment/getImg）
		// "sso" 路径同时短路 token 读取（公开服务无 token）
		c, err := buildClient(cmd, "sso", "NAZHI_TIMEOUT")
		if err != nil {
			// 构造失败属本地配置问题，走参数档。当前 urlType="sso" 组装链路实际不可达此分支，防御保留。
			printParamError(fmt.Errorf("构造 Client 失败: %w", err))
			return
		}

		printVerbose("正在下载附件（无 token 模式）id=%d → %s", id, output)
		if err := c.DownloadFile(cmd.Context(), id, output); err != nil {
			printError(fmt.Errorf("下载文件失败: %w", err))
			return
		}

		// DownloadFile SDK 在成功路径不返回业务数据（仅 error）。
		// 用 envelope.Empty (HTTP 204) 表达"成功但无业务负载"，与 SDK 语义一致——
		// CLI 不伪造不存在的 SDK 字段（如 id/output 实际上是命令行参数，不是 SDK 返回）。
		printEnvelope(envelope.Empty("下载成功"))
	},
}

func init() {
	fileDownloadCmd.Flags().Int64("id", 0, "附件 ID（必填，通常来自 task submitted 或 file upload）")
	fileDownloadCmd.Flags().StringP("output", "o", "", "本地保存路径（必填）")
	fileDownloadCmd.Flags().String("sso-base", "", "SSO 域名根地址（默认 https://www.nazhisoft.com）也可通过 NAZHI_SSO_BASE 设置")
	fileDownloadCmd.Flags().Int("timeout", 30, "HTTP 超时（秒）也可通过 NAZHI_TIMEOUT 设置")
	// 显式不提供 --token flag（下载服务器独立，不需要业务域 Token）
}
