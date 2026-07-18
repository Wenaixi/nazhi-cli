package main

import (
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// bonusCmd 表示 nazhi bonus 父命令
var bonusCmd = &cobra.Command{
	Use:   "bonus",
	Short: "积分管理",
	Long:  `查询学生积分信息。`,
}

// bonusMonthCmd 表示 nazhi bonus month 命令
//
//	nazhi bonus month --token <token> [--base-url <url>] [--timeout <秒>]
var bonusMonthCmd = &cobra.Command{
	Use:   "month",
	Short: "获取月积分",
	Long:  `获取当前月份的积分信息。`,
	Example: `  nazhi bonus month --token eyJhbGciOiJIUzI1NiJ9.xxx
		  nazhi bonus month --token eyJhbGciOiJIUzI1NiJ9.xxx --base-url http://139.159.205.146:8280`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在获取月积分...")
		bonusList, err := c.GetMonthBonus(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("获取月积分失败: %w", err))
			return
		}

		printEnvelope(envelope.Success(bonusList))
	},
}

// bonusRankCmd 表示 nazhi bonus rank 命令
//
//	nazhi bonus rank --token <token> [--limit <条>] [--base-url <url>] [--timeout <秒>]
var bonusRankCmd = &cobra.Command{
	Use:   "rank",
	Short: "获取班级积分排名",
	Long:  `获取班级积分排名。`,
	Example: `  nazhi bonus rank --token eyJhbGciOiJIUzI1NiJ9.xxx --limit 10
		  nazhi bonus rank --token eyJhbGciOiJIUzI1NiJ9.xxx --limit 10 --base-url http://139.159.205.146:8280`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		limit, _ := cmd.Flags().GetInt("limit")

		printVerbose("正在获取积分排名...")
		ranks, err := c.GetBonusRank(cmd.Context(), token, limit)
		if err != nil {
			printError(fmt.Errorf("获取积分排名失败: %w", err))
			return
		}

		printEnvelope(envelope.Success(ranks))
	},
}

func init() {
	// bonus 父命令
	rootCmd.AddCommand(bonusCmd)

	// bonus month
	bonusCmd.AddCommand(bonusMonthCmd)
	registerBizFlags(bonusMonthCmd)

	// bonus rank
	bonusCmd.AddCommand(bonusRankCmd)
	bonusRankCmd.Flags().Int("limit", 10, "排名数量限制")
	registerBizFlags(bonusRankCmd)
}
