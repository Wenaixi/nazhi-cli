package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// typicalCaseCmd 表示 nazhi typical-case 父命令
//
//	nazhi typical-case submit --token <token> --payload '<json>'
//	nazhi typical-case list --token <token> [--page <页>] [--page-size <条>]
var typicalCaseCmd = &cobra.Command{
	Use:   "typical-case",
	Short: "典型案例管理",
	Long:  `管理综合评价典型案例：提交典型案例、查看已提交记录。`,
}

// typicalCaseSubmitCmd 表示 nazhi typical-case submit 命令
//
//	nazhi typical-case submit --token <token> --payload '<json>'
//	成功时 envelope.Empty("典型案例提交成功")，与 AddHonor 写操作模式一致
var typicalCaseSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "提交典型案例",
	Long: `提交一条典型案例。payload 是 addTypicalCase 请求体 JSON，
可用 @file.json 从文件读取，或 - 从 stdin 读取。`,
	Example: `  nazhi typical-case submit --token eyJhbGciOiJIUzI1NiJ9.xxx --payload '{"title":"...","type":"1","typeName":"研究性学习报告","teacherName":"王隆滨","content":"..."}'
  nazhi typical-case submit --token eyJhbGciOiJIUzI1NiJ9.xxx --payload @case.json
  echo '{"title":"..."}' | nazhi typical-case submit --token "xxx" --payload -`,
	Run: func(cmd *cobra.Command, args []string) {
		payloadRaw, _ := cmd.Flags().GetString("payload")
		if payloadRaw == "" {
			printEnvelope(envelope.Error(400, "--payload 为必填"))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		payloadBytes, err := parsePayloadFromArg(payloadRaw)
		if err != nil {
			printError(fmt.Errorf("读取 payload 失败: %w", err))
			return
		}

		var payload types.AddTypicalCasePayload
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			printError(fmt.Errorf("解析 payload JSON 失败: %w", err))
			return
		}

		printVerbose("正在提交典型案例...")
		if err := c.AddTypicalCase(cmd.Context(), token, payload); err != nil {
			printError(fmt.Errorf("提交典型案例失败: %w", err))
			return
		}

		printEnvelope(envelope.Empty("典型案例提交成功"))
	},
}

// typicalCaseListCmd 表示 nazhi typical-case list 命令
//
//	nazhi typical-case list --token <token> [--page <页>] [--page-size <条>]
//	envelope.data 透传 SDK GetTypicalCaseListJSON 的原始 JSON（含 records + page）
var typicalCaseListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取已提交典型案例",
	Long:  `获取当前用户已提交的全部典型案例记录（分页）。`,
	Example: `  nazhi typical-case list --token eyJhbGciOiJIUzI1NiJ9.xxx
  nazhi typical-case list --token eyJhbGciOiJIUzI1NiJ9.xxx --page 1 --page-size 20`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		pageNo, _ := cmd.Flags().GetInt("page")
		pageSize, _ := cmd.Flags().GetInt("page-size")

		printVerbose("正在获取典型案例列表...")
		raw, err := c.GetTypicalCaseListJSON(cmd.Context(), token, pageNo, pageSize)
		if err != nil {
			printError(fmt.Errorf("获取典型案例列表失败: %w", err))
			return
		}

		printEnvelope(envelope.Success(json.RawMessage(raw)))
	},
}

func init() {
	// typical-case submit
	typicalCaseCmd.AddCommand(typicalCaseSubmitCmd)
	typicalCaseSubmitCmd.Flags().String("payload", "", "典型案例 JSON（必填，可用 @file.json 从文件读取，或 - 从 stdin 读取）")
	registerBizFlags(typicalCaseSubmitCmd)

	// typical-case list
	typicalCaseCmd.AddCommand(typicalCaseListCmd)
	typicalCaseListCmd.Flags().Int("page", 1, "页码（从 1 开始）")
	typicalCaseListCmd.Flags().Int("page-size", 20, "每页条数")
	registerBizFlags(typicalCaseListCmd)

	// typical-case update
	typicalCaseCmd.AddCommand(typicalCaseUpdateCmd)
	typicalCaseUpdateCmd.Flags().String("payload", "", "典型案例 JSON（必填，可用 @file.json）")
	registerBizFlags(typicalCaseUpdateCmd)

	// typical-case delete
	typicalCaseCmd.AddCommand(typicalCaseDeleteCmd)
	typicalCaseDeleteCmd.Flags().String("id", "", "案例 ID（必填）")
	registerBizFlags(typicalCaseDeleteCmd)
}

// typicalCaseUpdateCmd 表示 nazhi typical-case update 命令
var typicalCaseUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "更新典型案例",
	Run: func(cmd *cobra.Command, args []string) {
		payloadRaw, _ := cmd.Flags().GetString("payload")
		if payloadRaw == "" {
			printEnvelope(envelope.Error(400, "--payload 为必填"))
			return
		}
		payloadBytes, err := parsePayloadFromArg(payloadRaw)
		if err != nil {
			printError(fmt.Errorf("读取 payload 失败: %w", err))
			return
		}
		var payload map[string]any
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			printError(fmt.Errorf("解析 payload JSON 失败: %w", err))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在更新典型案例...")
		if err := c.UpdateTypicalCase(cmd.Context(), token, payload); err != nil {
			printError(fmt.Errorf("更新典型案例失败: %w", err))
			return
		}
		printEnvelope(envelope.Empty("典型案例更新成功"))
	},
}

// typicalCaseDeleteCmd 表示 nazhi typical-case delete 命令
var typicalCaseDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "删除典型案例",
	Run: func(cmd *cobra.Command, args []string) {
		idStr, _ := cmd.Flags().GetString("id")
		if idStr == "" {
			printEnvelope(envelope.Error(400, "--id 为必填"))
			return
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			printEnvelope(envelope.Error(400, "--id 必须为正整数"))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在删除典型案例...")
		if err := c.DeleteTypicalCase(cmd.Context(), token, id); err != nil {
			printError(fmt.Errorf("删除典型案例失败: %w", err))
			return
		}
		printEnvelope(envelope.Empty("典型案例已删除"))
	},
}
