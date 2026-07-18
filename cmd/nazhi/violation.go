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
//
//	nazhi violation list --token <token> [--page <页>] [--page-size <条>] [--base-url <url>] [--timeout <秒>]
var violationListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取违规记录列表",
	Long:  `获取当前用户的违规记录列表（分页）。`,
	Example: `  nazhi violation list --token eyJhbGciOiJIUzI1NiJ9.xxx
		  nazhi violation list --token eyJhbGciOiJIUzI1NiJ9.xxx --page 1 --page-size 20`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		pageNo, _ := cmd.Flags().GetInt("page")
		pageSize, _ := cmd.Flags().GetInt("page-size")

		printVerbose("正在获取违规记录...")
		records, pb, err := c.GetViolationList(cmd.Context(), token, pageNo, pageSize, "")
		if err != nil {
			printError(fmt.Errorf("获取违规记录失败: %w", err))
			return
		}

		printEnvelope(envelope.Success(map[string]any{
			"records": records,
			"page":    pb,
		}))
	},
}

// violationTypesCmd 表示 nazhi violation types 命令
//
//	nazhi violation types --token <token> [--base-url <url>] [--timeout <秒>]
var violationTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "获取违规类型",
	Long:  `获取所有违规类型列表。`,
	Example: `  nazhi violation types --token eyJhbGciOiJIUzI1NiJ9.xxx
		  nazhi violation types --token eyJhbGciOiJIUzI1NiJ9.xxx --base-url http://139.159.205.146:8280`,
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
	// violation 父命令
	rootCmd.AddCommand(violationCmd)

	// violation list
	violationCmd.AddCommand(violationListCmd)
	violationListCmd.Flags().Int("page", 1, "页码（从 1 开始）")
	violationListCmd.Flags().Int("page-size", 20, "每页条数")
	registerBizFlags(violationListCmd)

	// violation types
	violationCmd.AddCommand(violationTypesCmd)
	registerBizFlags(violationTypesCmd)
}
