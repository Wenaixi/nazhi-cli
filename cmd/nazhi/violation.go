package main

import (
	"encoding/json"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// violationCmd 是违规违纪查询父命令。
var violationCmd = &cobra.Command{
	Use:   "violation",
	Short: "违规记录查询",
	Long:  "查询当前用户的违规违纪记录和违规事由类型。",
}

// violationListCmd 查询当前用户的违规记录。
var violationListCmd = &cobra.Command{
	Use:     "list",
	Short:   "获取违规记录列表",
	Long:    "获取当前用户的违规违纪记录（分页），支持 --key 关键字筛选，并保留平台原始 records/page 字段。",
	Example: "  nazhi violation list --token eyJhbGciOiJIUzI1NiJ9.xxx\n  nazhi violation list --token eyJhbGciOiJIUzI1NiJ9.xxx --page 1 --page-size 10 --key 迟到",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		pageNo, _ := cmd.Flags().GetInt("page")
		pageSize, _ := cmd.Flags().GetInt("page-size")
		key, _ := cmd.Flags().GetString("key")

		printVerbose("正在获取违规记录...")
		raw, err := c.GetViolationListJSON(cmd.Context(), token, pageNo, pageSize, key)
		if err != nil {
			printError(fmt.Errorf("获取违规记录失败: %w", err))
			return
		}
		printEnvelope(envelope.Success(json.RawMessage(raw)))
	},
}

// violationTypesCmd 查询违规事由类型。
var violationTypesCmd = &cobra.Command{
	Use:     "types",
	Short:   "获取违规事由类型",
	Long:    "获取前端德育说明使用的全部违规事由类型。",
	Example: "  nazhi violation types --token eyJhbGciOiJIUzI1NiJ9.xxx",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		printVerbose("正在获取违规事由类型...")
		items, err := c.GetViolationTypes(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("获取违规事由类型失败: %w", err))
			return
		}
		printEnvelope(envelope.Success(items))
	},
}

func init() {
	rootCmd.AddCommand(violationCmd)

	violationCmd.AddCommand(violationListCmd)
	violationListCmd.Flags().Int("page", 1, "页码（从 1 开始）")
	violationListCmd.Flags().Int("page-size", 10, "每页条数（与前端德育表现页一致）")
	violationListCmd.Flags().String("key", "", "搜索关键字")
	registerBizFlags(violationListCmd)

	violationCmd.AddCommand(violationTypesCmd)
	registerBizFlags(violationTypesCmd)
}
