package main

import (
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// violationCmd 表示 nazhi violation 父命令
var violationCmd = &cobra.Command{
	Use:   "violation",
	Short: "违规记录管理",
	Long:  `查询违规记录和违规类型。`,
}

// violationListCmd 表示 nazhi violation list 命令
var violationListCmd = &cobra.Command{
	Use:     "list",
	Short:   "获取违规记录列表",
	Long:    `获取当前用户的违规记录列表（分页）。`,
	Example: "  nazhi violation list --page 1 --size 20 --token xxx",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		pageNo, _ := cmd.Flags().GetInt("page")
		pageSize, _ := cmd.Flags().GetInt("size")
		key, _ := cmd.Flags().GetString("key")

		printVerbose("正在获取违规记录...")
		result, err := c.GetViolationList(cmd.Context(), token, pageNo, pageSize, key)
		if err != nil {
			printError(fmt.Errorf("获取违规记录失败: %w", err))
			return
		}

		printEnvelope(envelope.Success(map[string]any{
			"records": result.Records,
			"page":    result.Page,
		}))
	},
}

// violationTypesCmd 表示 nazhi violation types 命令
var violationTypesCmd = &cobra.Command{
	Use:     "types",
	Short:   "获取违规类型",
	Long:    `获取所有违规类型列表。`,
	Example: "  nazhi violation types --token xxx",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在获取违规类型...")
		typesList, err := c.GetViolationTypes(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("获取违规类型失败: %w", err))
			return
		}

		printEnvelope(envelope.Success(typesList))
	},
}

func init() {
	rootCmd.AddCommand(violationCmd)
	violationCmd.AddCommand(violationListCmd)
	violationCmd.AddCommand(violationTypesCmd)

	violationListCmd.Flags().Int("page", 1, "页码（从 1 开始）")
	violationListCmd.Flags().Int("size", 20, "每页条数")
	violationListCmd.Flags().String("key", "", "搜索关键字")
	registerBizFlags(violationListCmd)

	violationTypesCmd.Flags().String("page", "1", "页码（从 1 开始）")
	registerBizFlags(violationTypesCmd)
}
